package core

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// DoneOptions selects what CheckDone inspects.
type DoneOptions struct {
	// Root is where .arch lives and the default checkout to inspect.
	Root string
	// FilesDir inspects another checkout (a git worktree) of the same repository.
	FilesDir string
	// NodeID is the node being finished.
	NodeID NodeID
	// Base is the git ref the branch started from. Empty finds main or master.
	Base string
}

// DoneViolation is one file outside the node's scope.
type DoneViolation struct {
	Path   string `json:"path"`
	Rule   string `json:"rule"`
	Detail string `json:"detail,omitempty"`
}

// DoneCompletion tells the agent what it may do next under the completion policy.
type DoneCompletion struct {
	AgentMayComplete bool `json:"agent_may_complete"`
	ReviewRequired   bool `json:"review_required"`
	// FinishStatus is the status to set when finished: needs_review, done, or empty when
	// agents may not mark nodes finished.
	FinishStatus string `json:"finish_status"`
}

// DoneReport is the single "am I done?" answer.
type DoneReport struct {
	NodeID      NodeID          `json:"node_id"`
	Pass        bool            `json:"pass"`
	Base        string          `json:"base"`
	Files       []string        `json:"files"`
	Violations  []DoneViolation `json:"scope_violations"`
	Drift       DriftReport     `json:"drift"`
	Verify      []string        `json:"verify_commands"`
	Completion  DoneCompletion  `json:"completion"`
	Problems    []string        `json:"problems"`
	Warnings    []string        `json:"warnings"`
	ReviewNotes string          `json:"review_notes,omitempty"`
}

// BranchFiles lists files that differ from base in the checkout at dir: committed and
// uncommitted changes to tracked files plus untracked files that are not gitignored. An
// empty base finds the merge-base with main or master (falling back to HEAD, which means
// "uncommitted work only"). Files under .arch/ are left out. The resolved base is returned.
func BranchFiles(dir, base string) (files []string, resolved string, err error) {
	if dir == "" {
		dir = "."
	}
	git := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		var out, errb bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errb
		if err := cmd.Run(); err != nil {
			msg := strings.TrimSpace(errb.String())
			if msg == "" {
				msg = err.Error()
			}
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
		}
		return out.String(), nil
	}
	if _, herr := git("rev-parse", "--verify", "--quiet", "HEAD"); herr != nil {
		tree, terr := emptyTreeID(dir)
		if terr != nil {
			return nil, "", terr
		}
		base = tree
	} else if strings.HasPrefix(base, "-") {
		return nil, "", fmt.Errorf("base %q is not a git ref", base)
	} else if base == "" {
		base = "HEAD"
		for _, cand := range []string{"main", "master", "origin/main", "origin/master"} {
			if _, e := git("rev-parse", "--verify", "--quiet", cand); e != nil {
				continue
			}
			if mb, e := git("merge-base", "HEAD", cand); e == nil {
				base = strings.TrimSpace(mb)
			}
			break
		}
	}
	// Resolve to a commit id so an option-like or path-like value can never reach git diff
	// as anything but a revision.
	if sha, rerr := git("rev-parse", "--verify", "--quiet", base+"^{commit}"); rerr != nil {
		if tree, terr := git("rev-parse", "--verify", "--quiet", base+"^{tree}"); terr != nil || strings.TrimSpace(tree) == "" {
			return nil, "", fmt.Errorf("base %q is not a commit in this repository", base)
		}
	} else {
		base = strings.TrimSpace(sha)
	}
	tracked, err := git("diff", "--name-only", "--no-renames", "-z", base, "--")
	if err != nil {
		return nil, "", err
	}
	untracked, err := git("ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return nil, "", err
	}
	seen := map[string]bool{}
	for _, list := range []string{tracked, untracked} {
		for _, p := range strings.Split(list, "\x00") {
			// The model itself (status updates, accepted proposals) is not the agent's
			// code, so it never counts as an edit outside scope.
			if p = toSlash(p); p != "" && !seen[p] && p != ".arch" && !strings.HasPrefix(p, ".arch/") {
				seen[p] = true
				files = append(files, p)
			}
		}
	}
	return files, base, nil
}

// CheckDone answers "am I done?" for a node in one call: it takes the files that changed
// on the branch (not a hand-built list), checks them against the node's scope, runs drift
// limited to the node, reports contract changes, and states what the completion policy
// lets the agent do next. Pass means no scope violation and no node drift.
func CheckDone(idx *Index, opts DoneOptions) (DoneReport, error) {
	rep := DoneReport{NodeID: opts.NodeID, Files: []string{}, Violations: []DoneViolation{}, Problems: []string{}, Warnings: []string{}, Verify: []string{}}
	checkoutDir := opts.Root
	if opts.FilesDir != "" {
		top, err := resolveWorktree(opts.Root, opts.FilesDir)
		if err != nil {
			return rep, err
		}
		checkoutDir = top
	}
	files, base, err := BranchFiles(checkoutDir, opts.Base)
	if err != nil {
		return rep, err
	}
	rep.Base = base
	if head, herr := revParseHead(checkoutDir); herr == nil && head == base && opts.Base == "" {
		rep.Warnings = append(rep.Warnings, "no trunk branch (main or master) to compare with, or you are on it: only uncommitted changes were checked; pass base to include committed work")
	}
	rep.Files = append(rep.Files, files...)

	results, err := CheckScope(opts.NodeID, files, idx)
	if err != nil {
		return rep, err
	}
	for _, r := range results {
		if r.Kind != KindViolation {
			continue
		}
		v := DoneViolation{Path: r.Path}
		if len(r.Hits) > 0 {
			v.Rule, v.Detail = r.Hits[0].Rule, r.Hits[0].Detail
		}
		rep.Violations = append(rep.Violations, v)
		rep.Problems = append(rep.Problems, fmt.Sprintf("%s is outside this node's scope (%s)", r.Path, v.Rule))
	}

	rep.Drift, err = DriftWith(idx, DriftOptions{Root: opts.Root, FilesDir: opts.FilesDir, NodeID: opts.NodeID})
	if err != nil {
		return rep, err
	}
	for _, u := range rep.Drift.Unmatched {
		rep.Problems = append(rep.Problems, fmt.Sprintf("%s glob %q matches no file yet", u.Field, u.Glob))
	}
	for _, o := range rep.Drift.Overlaps {
		rep.Problems = append(rep.Problems, fmt.Sprintf("%s is owned by several non-nested nodes", o.Path))
	}

	idx.mu.Lock()
	n := idx.Nodes[opts.NodeID]
	m := idx.activeMethodLocked()
	if n != nil {
		rep.Verify = append(rep.Verify, n.Verify...)
		if idx.contractChangedLocked(n) {
			rep.Warnings = append(rep.Warnings, "a contract this node must honour changed after its status was last set; re-read get_context and re-check the implementation")
		}
		rep.Completion = DoneCompletion{AgentMayComplete: m.Completion.AgentMayComplete, ReviewRequired: reviewRequiredFor(m, n)}
		switch {
		case !m.Completion.AgentMayComplete:
			rep.ReviewNotes = "Agents may not mark nodes finished under this method: report your result to the architect."
		case rep.Completion.ReviewRequired:
			rep.Completion.FinishStatus = string(StatusNeedsReview)
			rep.ReviewNotes = "Review is required: set needs_review, not done."
		default:
			rep.Completion.FinishStatus = string(StatusDone)
		}
		if n.Risk != "" {
			rep.Warnings = append(rep.Warnings, "node risk is "+n.Risk)
		}
	}
	idx.mu.Unlock()
	if len(rep.Verify) > 0 {
		rep.Warnings = append(rep.Warnings, "run the verify commands too; check_done does not execute them")
	}
	rep.Pass = len(rep.Problems) == 0
	return rep, nil
}

func revParseHead(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
