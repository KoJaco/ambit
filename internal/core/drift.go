package core

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// UnmatchedGlob is an implementation or scope glob that matches no tracked file.
type UnmatchedGlob struct {
	Node  NodeID `json:"node"`
	Field string `json:"field"` // "implementation" or "scope"
	Glob  string `json:"glob"`
}

// Overlap is a tracked file owned by two or more nodes of which at least two are not
// in an ancestor/descendant relationship.
type Overlap struct {
	Path   string   `json:"path"`
	Owners []NodeID `json:"owners"`
}

// DriftReport lists where the model and the tracked files disagree.
type DriftReport struct {
	Unowned   []string        `json:"unowned_files"`
	Unmatched []UnmatchedGlob `json:"unmatched_globs"`
	Overlaps  []Overlap       `json:"overlaps"`
	// Ignored counts files skipped because they match a drift_ignore glob.
	Ignored int `json:"ignored_files"`
}

// Findings is the total number of drift findings.
func (r DriftReport) Findings() int {
	return len(r.Unowned) + len(r.Unmatched) + len(r.Overlaps)
}

// TrackedFiles lists git-tracked paths in dir (git ls-files -z), slash separated.
func TrackedFiles(dir string) ([]string, error) {
	return listFiles(dir, false)
}

// ProjectFiles lists tracked paths plus untracked paths that .gitignore does not
// exclude, so new files count before they are staged or committed.
func ProjectFiles(dir string) ([]string, error) {
	return listFiles(dir, true)
}

func listFiles(dir string, withUntracked bool) ([]string, error) {
	if dir == "" {
		dir = "."
	}
	args := []string{"ls-files", "-z"}
	if withUntracked {
		args = append(args, "--cached", "--others", "--exclude-standard")
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git ls-files: %s", msg)
	}
	seen := map[string]bool{}
	var files []string
	for _, line := range strings.Split(stdout.String(), "\x00") {
		if line == "" {
			continue
		}
		line = toSlash(line)
		if !seen[line] {
			seen[line] = true
			files = append(files, line)
		}
	}
	return files, nil
}

// DefaultDriftIgnore lists tooling files that no node normally owns. A project adds
// to it with the drift_ignore array in .arch/config.json.
var DefaultDriftIgnore = []string{
	".gitignore", ".gitattributes", ".editorconfig", ".github/**",
	"**/go.mod", "**/go.sum", "**/go.work", "**/go.work.sum",
	"**/package.json", "**/package-lock.json", "**/pnpm-lock.yaml", "**/yarn.lock",
	"**/bun.lock", "**/bun.lockb", "**/tsconfig*.json", "**/.npmrc", "**/.nvmrc",
	"**/Cargo.lock", "**/uv.lock", "**/poetry.lock", "**/.terraform.lock.hcl",
	"LICENSE", "LICENSE.*", "README.md",
}

// DriftOptions narrows or redirects a drift run.
type DriftOptions struct {
	// Root is where the model's .arch lives and the default place files are listed.
	Root string
	// FilesDir lists files from another checkout (a git worktree) of the same
	// repository. Empty means Root.
	FilesDir string
	// NodeID limits the report to that node: its unmatched globs and the overlaps
	// it takes part in. Unowned files are not reported.
	NodeID NodeID
}

// Drift compares the model's implementation and scope globs with the files in the
// project under root: tracked files plus untracked files that are not gitignored.
// Files under .arch/ are the model itself and are not reported as unowned, and files
// matching DefaultDriftIgnore or config.json drift_ignore are skipped.
// It does not write anything and is safe to call while Watch runs on idx.
func Drift(idx *Index, root string) (DriftReport, error) {
	return DriftWith(idx, DriftOptions{Root: root})
}

// DriftWith is Drift with options.
func DriftWith(idx *Index, opts DriftOptions) (DriftReport, error) {
	if idx == nil {
		return DriftReport{}, errors.New("no index")
	}
	filesDir := opts.FilesDir
	if filesDir == "" {
		filesDir = opts.Root
	}
	if opts.FilesDir != "" {
		top, err := resolveWorktree(opts.Root, opts.FilesDir)
		if err != nil {
			return DriftReport{}, err
		}
		filesDir = top
	}
	files, err := ProjectFiles(filesDir)
	if err != nil {
		return DriftReport{}, err
	}
	// Snapshot what matching needs under the lock; match after releasing it.
	idx.mu.Lock()
	if opts.NodeID != "" && idx.Nodes[opts.NodeID] == nil {
		idx.mu.Unlock()
		return DriftReport{}, fmt.Errorf("unknown node %q", opts.NodeID)
	}
	snap := &Index{Order: append([]NodeID(nil), idx.Order...), Nodes: make(map[NodeID]*Node, len(idx.Nodes))}
	snap.DriftIgnore = append([]string(nil), idx.DriftIgnore...)
	for id, n := range idx.Nodes {
		if n == nil {
			continue
		}
		c := *n
		c.Implementation = append([]string(nil), n.Implementation...)
		c.Scope = append([]string(nil), n.Scope...)
		snap.Nodes[id] = &c
	}
	idx.mu.Unlock()
	return driftLocked(snap, files, opts.NodeID)
}

func driftLocked(idx *Index, files []string, only NodeID) (DriftReport, error) {
	report := DriftReport{Unowned: []string{}, Unmatched: []UnmatchedGlob{}, Overlaps: []Overlap{}}
	if err := validateGlobs(idx); err != nil {
		return report, err
	}
	ignore := append(append([]string(nil), DefaultDriftIgnore...), idx.DriftIgnore...)
	var kept []string
	for _, path := range files {
		if path == ".arch" || strings.HasPrefix(path, ".arch/") {
			continue
		}
		skip, err := matchGlobs(ignore, path)
		if err != nil {
			return report, fmt.Errorf("drift_ignore: %w", err)
		}
		if skip {
			// A file a node explicitly owns is still checked below.
			owners, oerr := ownersOfPath(idx, path)
			if oerr != nil {
				return report, oerr
			}
			if len(owners) == 0 {
				report.Ignored++
				continue
			}
		}
		kept = append(kept, path)
	}
	files = kept
	for _, path := range files {
		owners, err := ownersOfPath(idx, path)
		if err != nil {
			return report, err
		}
		switch {
		case len(owners) == 0:
			if only == "" {
				report.Unowned = append(report.Unowned, path)
			}
		case len(owners) > 1 && hasNonNestedPair(idx, owners):
			if only == "" || containsNode(owners, only) {
				report.Overlaps = append(report.Overlaps, Overlap{Path: path, Owners: owners})
			}
		}
	}
	for _, id := range idx.Order {
		n := idx.Nodes[id]
		if n == nil || (only != "" && id != only) {
			continue
		}
		for _, f := range []struct {
			name  string
			globs []string
		}{{"implementation", n.Implementation}, {"scope", n.Scope}} {
			for _, g := range f.globs {
				if strings.TrimSpace(g) == "" {
					continue
				}
				hit := false
				for _, path := range files {
					ok, err := matchGlobs([]string{g}, path)
					if err != nil {
						return report, fmt.Errorf("node %s: %s: %w", id, f.name, err)
					}
					if ok {
						hit = true
						break
					}
				}
				if !hit {
					report.Unmatched = append(report.Unmatched, UnmatchedGlob{Node: id, Field: f.name, Glob: g})
				}
			}
		}
	}
	sort.Strings(report.Unowned)
	return report, nil
}

// ownersOfPath lists the nodes whose implementation globs match path, in model order.
func ownersOfPath(idx *Index, path string) ([]NodeID, error) {
	var owners []NodeID
	for _, id := range idx.Order {
		n := idx.Nodes[id]
		if n == nil {
			continue
		}
		ok, err := matchGlobs(n.Implementation, path)
		if err != nil {
			return nil, fmt.Errorf("node %s: implementation: %w", id, err)
		}
		if ok {
			owners = append(owners, id)
		}
	}
	return owners, nil
}

func containsNode(ids []NodeID, id NodeID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func hasNonNestedPair(idx *Index, owners []NodeID) bool {
	for i := 0; i < len(owners); i++ {
		for j := i + 1; j < len(owners); j++ {
			if !isAncestor(idx, owners[i], owners[j]) && !isAncestor(idx, owners[j], owners[i]) {
				return true
			}
		}
	}
	return false
}

// isAncestor reports whether anc is a strict ancestor of id via parent_id.
func isAncestor(idx *Index, anc, id NodeID) bool {
	seen := map[NodeID]bool{}
	for {
		n := idx.Nodes[id]
		if n == nil || n.ParentID == "" || seen[id] {
			return false
		}
		seen[id] = true
		if n.ParentID == anc {
			return true
		}
		id = n.ParentID
	}
}

// FormatDrift renders the report as plain text. An empty report returns "".
func FormatDrift(r DriftReport) string {
	var b strings.Builder
	if len(r.Unowned) > 0 {
		b.WriteString("Files owned by no node:\n")
		for _, p := range r.Unowned {
			fmt.Fprintf(&b, "  %s\n", p)
		}
	}
	if len(r.Unmatched) > 0 {
		b.WriteString("Globs matching no tracked file:\n")
		for _, u := range r.Unmatched {
			fmt.Fprintf(&b, "  %s %s %s\n", u.Node, u.Field, u.Glob)
		}
	}
	if len(r.Overlaps) > 0 {
		b.WriteString("Files owned by multiple non-nested nodes:\n")
		for _, o := range r.Overlaps {
			ids := make([]string, len(o.Owners))
			for i, id := range o.Owners {
				ids[i] = string(id)
			}
			fmt.Fprintf(&b, "  %s %s\n", o.Path, strings.Join(ids, ", "))
		}
	}
	if b.Len() > 0 {
		b.WriteString("Drift compares the model with files git can see (tracked, plus untracked files that are not gitignored). Files in another worktree are only seen when that worktree is passed.\n")
		if r.Ignored > 0 {
			fmt.Fprintf(&b, "(%d file(s) skipped by drift_ignore.)\n", r.Ignored)
		}
	}
	return b.String()
}

// resolveWorktree returns the top level of the checkout other, after checking it is a
// checkout (main or linked worktree) of the same repository as root, so a tool argument
// cannot point drift at an unrelated tree. A relative path is taken relative to root, and
// a subdirectory resolves to its checkout's top level so paths stay repository-relative.
func resolveWorktree(root, other string) (string, error) {
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(other) {
		other = filepath.Join(absRoot, other)
	}
	a, err := gitCommonDir(absRoot)
	if err != nil {
		return "", err
	}
	b, err := gitCommonDir(other)
	if err != nil {
		return "", fmt.Errorf("worktree %q: %w", other, err)
	}
	if a != b {
		return "", fmt.Errorf("worktree %q belongs to a different repository", other)
	}
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = other
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("worktree %q: not a work tree", other)
	}
	return strings.TrimSpace(string(out)), nil
}

// gitCommonDir is the shared .git directory of the checkout containing dir, resolved to
// an absolute symlink-free path. It avoids --path-format, which needs git 2.31.
func gitCommonDir(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git checkout: %s", dir)
	}
	p := strings.TrimSpace(string(out))
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	if real, err := filepath.EvalSymlinks(p); err == nil {
		p = real
	}
	return filepath.Clean(p), nil
}
