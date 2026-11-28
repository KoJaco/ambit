package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"
)

// Verify run limits.
const (
	DefaultVerifyTimeoutS = 120
	MaxVerifyTimeoutS     = 600
	// verifyOutputCap is how many trailing bytes of a command's output are kept.
	verifyOutputCap = 8 * 1024
	// MaxDependentsRun caps how many dependents check_dependents runs in one call.
	MaxDependentsRun = 16
)

// Verify command statuses.
const (
	VerifyPassed      = "passed"
	VerifyFailed      = "failed"
	VerifyTimeout     = "timeout"
	VerifyNotRunnable = "not_runnable"
)

// VerifyOptions selects what RunVerify runs and where.
type VerifyOptions struct {
	// Root is where .arch lives and the default checkout to run in.
	Root string
	// FilesDir runs in another checkout (a git worktree) of the same repository.
	FilesDir string
	// NodeID is the node whose verify commands run.
	NodeID NodeID
	// TimeoutS is the per-command timeout in seconds (default 120, at most 600).
	TimeoutS int
}

// VerifyResult is the outcome of one verify command.
type VerifyResult struct {
	Command string `json:"command"`
	Status  string `json:"status"`
	// ExitCode is set for passed and failed runs.
	ExitCode   int `json:"exit_code"`
	DurationMS int `json:"duration_ms"`
	// Reason explains not_runnable and timeout.
	Reason string `json:"reason,omitempty"`
	// Output is the tail of the combined stdout and stderr.
	Output          string `json:"output,omitempty"`
	OutputTruncated bool   `json:"output_truncated,omitempty"`
}

// VerifyReport is RunVerify's result.
type VerifyReport struct {
	NodeID  NodeID         `json:"node_id"`
	Dir     string         `json:"dir"`
	Pass    bool           `json:"pass"`
	Results []VerifyResult `json:"results"`
	// Counts by status.
	Passed      int      `json:"passed"`
	Failed      int      `json:"failed"`
	TimedOut    int      `json:"timed_out"`
	NotRunnable int      `json:"not_runnable"`
	Warnings    []string `json:"warnings"`
}

// RunVerify runs the verify commands of a node, taken only from the model on disk (never
// from a pending proposal), with `sh -c` in the checkout directory. Each command has its own
// timeout and a minimal environment: a fixed allowlist of variables plus the variables the
// command names itself. A command whose binary cannot be found, or that references an
// unset environment variable, is reported not_runnable and is not started.
//
// This executes shell commands from the model, so only accept verify commands you would run
// yourself. Nothing is sandboxed beyond the environment and the timeout.
func RunVerify(ctx context.Context, idx *Index, opts VerifyOptions) (VerifyReport, error) {
	rep := VerifyReport{NodeID: opts.NodeID, Results: []VerifyResult{}, Warnings: []string{}}
	dir, err := checkoutDir(opts.Root, opts.FilesDir)
	if err != nil {
		return rep, err
	}
	rep.Dir = dir
	idx.mu.Lock()
	n := idx.Nodes[opts.NodeID]
	var cmds []string
	if n != nil {
		cmds = append(cmds, n.Verify...)
	}
	idx.mu.Unlock()
	if n == nil {
		return rep, fmt.Errorf("%w: %q", ErrNotFound, opts.NodeID)
	}
	timeout := time.Duration(DefaultVerifyTimeoutS) * time.Second
	if opts.TimeoutS > 0 {
		if opts.TimeoutS > MaxVerifyTimeoutS {
			opts.TimeoutS = MaxVerifyTimeoutS
		}
		timeout = time.Duration(opts.TimeoutS) * time.Second
	}
	if len(cmds) == 0 {
		rep.Warnings = append(rep.Warnings, "the node has no verify commands")
	}
	for _, c := range cmds {
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		res := runOne(ctx, dir, c, timeout)
		rep.Results = append(rep.Results, res)
		switch res.Status {
		case VerifyPassed:
			rep.Passed++
		case VerifyFailed:
			rep.Failed++
		case VerifyTimeout:
			rep.TimedOut++
		case VerifyNotRunnable:
			rep.NotRunnable++
		}
	}
	rep.Pass = rep.Failed == 0 && rep.TimedOut == 0 && rep.NotRunnable == 0
	return rep, nil
}

func checkoutDir(root, filesDir string) (string, error) {
	if root == "" {
		root = "."
	}
	if filesDir == "" {
		return root, nil
	}
	return resolveWorktree(root, filesDir)
}

func runOne(ctx context.Context, dir, command string, timeout time.Duration) VerifyResult {
	res := VerifyResult{Command: command}
	if reason := notRunnableReason(dir, command); reason != "" {
		res.Status, res.Reason = VerifyNotRunnable, reason
		return res
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		res.Status, res.Reason = VerifyNotRunnable, "sh not found on PATH"
		return res
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, sh, "-c", command)
	cmd.Dir = dir
	cmd.Env = verifyEnv(dir, command)
	out := &tailBuffer{max: verifyOutputCap}
	cmd.Stdout, cmd.Stderr = out, out
	prepareProcessGroup(cmd)
	cmd.WaitDelay = 2 * time.Second
	start := time.Now()
	err = cmd.Run()
	res.DurationMS = int(time.Since(start) / time.Millisecond)
	res.Output, res.OutputTruncated = out.String()
	var exit *exec.ExitError
	switch {
	case err == nil:
		res.Status = VerifyPassed
	case errors.Is(cctx.Err(), context.DeadlineExceeded):
		res.Status, res.ExitCode = VerifyTimeout, -1
		res.Reason = fmt.Sprintf("timed out after %s", timeout)
	case errors.As(err, &exit):
		res.Status, res.ExitCode = VerifyFailed, exit.ExitCode()
	default:
		res.Status, res.ExitCode, res.Reason = VerifyFailed, -1, err.Error()
	}
	return res
}

// tailBuffer keeps the last max bytes written to it.
type tailBuffer struct {
	max       int
	buf       []byte
	truncated bool
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = append([]byte(nil), t.buf[len(t.buf)-t.max:]...)
		t.truncated = true
	}
	return len(p), nil
}

func (t *tailBuffer) String() (string, bool) {
	b := t.buf
	if t.truncated {
		// The cut may split a rune; drop the partial lead bytes.
		for len(b) > 0 && !utf8.RuneStart(b[0]) {
			b = b[1:]
		}
	}
	return strings.ToValidUTF8(string(bytes.TrimSpace(b)), "?"), t.truncated
}

// verifyEnvAllow is the environment a verify command inherits.
var verifyEnvAllow = []string{
	"PATH", "HOME", "USER", "LOGNAME", "SHELL", "LANG", "LC_ALL", "LC_CTYPE", "TERM", "TZ", "TMPDIR", "CI",
	"GOPATH", "GOCACHE", "GOMODCACHE", "GOFLAGS", "GOPROXY", "GOROOT", "GOTOOLCHAIN",
	"XDG_CACHE_HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "SYSTEMROOT", "TEMP", "TMP",
}

var (
	envRefPattern    = regexp.MustCompile(`\$(?:\{([A-Za-z_][A-Za-z0-9_]*)([^}]*)\}|([A-Za-z_][A-Za-z0-9_]*))`)
	envAssignPattern = regexp.MustCompile(`(?:^|[\s;&|(])([A-Za-z_][A-Za-z0-9_]*)=`)
	segmentSplit     = regexp.MustCompile(`&&|\|\||;|\||\n`)
	envNamePattern   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// referencedEnv lists the environment variables a command reads that it does not default
// or assign itself.
func referencedEnv(command string) []string {
	assigned := map[string]bool{}
	for _, m := range envAssignPattern.FindAllStringSubmatch(command, -1) {
		assigned[m[1]] = true
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range envRefPattern.FindAllStringSubmatch(command, -1) {
		name, rest := m[1], m[2]
		if name == "" {
			name = m[3]
		}
		if assigned[name] || seen[name] {
			continue
		}
		if rest != "" && (strings.HasPrefix(rest, ":-") || strings.HasPrefix(rest, ":=") || strings.HasPrefix(rest, ":+") ||
			strings.HasPrefix(rest, "-") || strings.HasPrefix(rest, "=") || strings.HasPrefix(rest, "+")) {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func verifyEnv(dir, command string) []string {
	var env []string
	have := map[string]bool{}
	add := func(k string) {
		if have[k] {
			return
		}
		if v, ok := os.LookupEnv(k); ok {
			have[k] = true
			if k == "PATH" {
				v = filepath.Join(dir, "node_modules", ".bin") + string(os.PathListSeparator) + v
			}
			env = append(env, k+"="+v)
		}
	}
	for _, k := range verifyEnvAllow {
		add(k)
	}
	for _, k := range referencedEnv(command) {
		add(k)
	}
	return env
}

var shellBuiltinsAndKeywords = map[string]bool{
	"cd": true, "echo": true, "printf": true, "test": true, "[": true, "[[": true, "true": true, "false": true,
	"exit": true, "export": true, "set": true, "unset": true, "source": true, ".": true, ":": true, "eval": true,
	"exec": true, "read": true, "wait": true, "ulimit": true, "umask": true, "pwd": true, "return": true,
	"shift": true, "trap": true, "type": true, "command": true, "alias": true, "hash": true,
	"if": true, "then": true, "else": true, "elif": true, "fi": true, "for": true, "while": true, "until": true,
	"do": true, "done": true, "case": true, "esac": true, "in": true, "time": true, "!": true, "{": true, "}": true,
	"function": true,
}

// notRunnableReason returns why command cannot work here (a missing binary or an unset
// environment variable), or "". It is a best-effort scan of the shell text, not a parser.
func notRunnableReason(dir, command string) string {
	if strings.TrimSpace(command) == "" {
		return "empty command"
	}
	for _, name := range referencedEnv(command) {
		if _, ok := os.LookupEnv(name); !ok {
			return fmt.Sprintf("environment variable %s is not set", name)
		}
	}
	for _, seg := range segmentSplit.Split(command, -1) {
		word := firstCommandWord(seg)
		if word == "" || shellBuiltinsAndKeywords[word] {
			continue
		}
		if !binaryAvailable(dir, word) {
			return fmt.Sprintf("binary %q not found", word)
		}
	}
	return ""
}

// firstCommandWord returns the program a shell segment runs: the first word after leading
// NAME=value assignments, groupers and redirections-free prefixes. Words built from
// expansions are skipped ("").
func firstCommandWord(seg string) string {
	seg = strings.TrimLeft(seg, " \t(")
	for {
		seg = strings.TrimLeft(seg, " \t")
		if seg == "" {
			return ""
		}
		end := strings.IndexAny(seg, " \t")
		word := seg
		if end >= 0 {
			word = seg[:end]
		}
		if i := strings.IndexByte(word, '='); i > 0 && envNamePattern.MatchString(word[:i]) {
			// A NAME=value prefix. A quoted value with spaces is not handled; the rest is
			// then judged on its first word, which is the conservative direction.
			if end < 0 {
				return ""
			}
			seg = seg[end:]
			continue
		}
		word = strings.Trim(word, `"'`)
		if word == "" || strings.ContainsAny(word, "$`") || strings.HasPrefix(word, ">") || strings.HasPrefix(word, "<") {
			return ""
		}
		return word
	}
}

func binaryAvailable(dir, word string) bool {
	if strings.ContainsRune(word, '/') {
		p := word
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			return false
		}
		return runtime.GOOS == "windows" || st.Mode()&0o111 != 0
	}
	if _, err := exec.LookPath(word); err == nil {
		return true
	}
	local := filepath.Join(dir, "node_modules", ".bin", word)
	if st, err := os.Stat(local); err == nil && !st.IsDir() {
		return true
	}
	return false
}

// DependentsReport is CheckDependents' result.
type DependentsReport struct {
	NodeID     NodeID             `json:"node_id"`
	Pass       bool               `json:"pass"`
	Dependents []DependentVerify  `json:"dependents"`
	Skipped    []DependentSkipped `json:"skipped"`
	Warnings   []string           `json:"warnings"`
}

// DependentVerify is one dependent's verify run.
type DependentVerify struct {
	NodeID NodeID       `json:"node_id"`
	Report VerifyReport `json:"report"`
}

// DependentSkipped names a dependent that was not run and why.
type DependentSkipped struct {
	NodeID NodeID `json:"node_id"`
	Reason string `json:"reason"`
}

// directDependentsLocked lists the nodes that depend directly on id: those whose depends_on
// names it, and those it gates. Ordered by model order. Caller holds idx.mu.
func (idx *Index) directDependentsLocked(id NodeID) []NodeID {
	gated := map[NodeID]bool{}
	if n := idx.Nodes[id]; n != nil {
		for _, t := range n.Gates {
			gated[t] = true
		}
	}
	var out []NodeID
	for _, oid := range idx.Order {
		o := idx.Nodes[oid]
		if o == nil || oid == id {
			continue
		}
		if gated[oid] {
			out = append(out, oid)
			continue
		}
		for _, d := range o.DependsOn {
			if d == id {
				out = append(out, oid)
				break
			}
		}
	}
	return out
}

// CheckDependents runs the verify commands of every direct dependent of the node (see
// RunVerify for what that means). At most MaxDependentsRun dependents are run per call.
func CheckDependents(ctx context.Context, idx *Index, opts VerifyOptions) (DependentsReport, error) {
	rep := DependentsReport{NodeID: opts.NodeID, Pass: true, Dependents: []DependentVerify{}, Skipped: []DependentSkipped{}, Warnings: []string{}}
	idx.mu.Lock()
	if idx.Nodes[opts.NodeID] == nil {
		idx.mu.Unlock()
		return rep, fmt.Errorf("%w: %q", ErrNotFound, opts.NodeID)
	}
	deps := idx.directDependentsLocked(opts.NodeID)
	hasVerify := map[NodeID]bool{}
	for _, d := range deps {
		hasVerify[d] = len(idx.Nodes[d].Verify) > 0
	}
	idx.mu.Unlock()
	if len(deps) == 0 {
		rep.Warnings = append(rep.Warnings, "no node depends directly on this node")
	}
	ran := 0
	for _, d := range deps {
		switch {
		case !hasVerify[d]:
			rep.Skipped = append(rep.Skipped, DependentSkipped{NodeID: d, Reason: "no verify commands"})
			continue
		case ran >= MaxDependentsRun:
			rep.Skipped = append(rep.Skipped, DependentSkipped{NodeID: d, Reason: fmt.Sprintf("more than %d dependents; call run_verify for this one", MaxDependentsRun)})
			continue
		}
		ran++
		o := opts
		o.NodeID = d
		vr, err := RunVerify(ctx, idx, o)
		if err != nil {
			return rep, err
		}
		if !vr.Pass {
			rep.Pass = false
		}
		rep.Dependents = append(rep.Dependents, DependentVerify{NodeID: d, Report: vr})
	}
	return rep, nil
}
