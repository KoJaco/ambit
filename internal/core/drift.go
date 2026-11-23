package core

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
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
}

// Findings is the total number of drift findings.
func (r DriftReport) Findings() int {
	return len(r.Unowned) + len(r.Unmatched) + len(r.Overlaps)
}

// TrackedFiles lists git-tracked paths in dir (git ls-files -z), slash separated.
func TrackedFiles(dir string) ([]string, error) {
	if dir == "" {
		dir = "."
	}
	cmd := exec.Command("git", "ls-files", "-z")
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
	var files []string
	for _, line := range strings.Split(stdout.String(), "\x00") {
		if line != "" {
			files = append(files, toSlash(line))
		}
	}
	return files, nil
}

// Drift compares the model's implementation and scope globs with the files git tracks
// under root. Files under .arch/ are the model itself and are not reported as unowned.
// It does not write anything and is safe to call while Watch runs on idx.
func Drift(idx *Index, root string) (DriftReport, error) {
	if idx == nil {
		return DriftReport{}, errors.New("no index")
	}
	files, err := TrackedFiles(root)
	if err != nil {
		return DriftReport{}, err
	}
	// Snapshot what matching needs under the lock; match after releasing it.
	idx.mu.Lock()
	snap := &Index{Order: append([]NodeID(nil), idx.Order...), Nodes: make(map[NodeID]*Node, len(idx.Nodes))}
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
	return driftLocked(snap, files)
}

func driftLocked(idx *Index, files []string) (DriftReport, error) {
	report := DriftReport{Unowned: []string{}, Unmatched: []UnmatchedGlob{}, Overlaps: []Overlap{}}
	if err := validateGlobs(idx); err != nil {
		return report, err
	}
	for _, path := range files {
		var owners []NodeID
		for _, id := range idx.Order {
			n := idx.Nodes[id]
			if n == nil {
				continue
			}
			ok, err := matchGlobs(n.Implementation, path)
			if err != nil {
				return report, fmt.Errorf("node %s: implementation: %w", id, err)
			}
			if ok {
				owners = append(owners, id)
			}
		}
		switch {
		case len(owners) == 0:
			if path != ".arch" && !strings.HasPrefix(path, ".arch/") {
				report.Unowned = append(report.Unowned, path)
			}
		case len(owners) > 1 && hasNonNestedPair(idx, owners):
			report.Overlaps = append(report.Overlaps, Overlap{Path: path, Owners: owners})
		}
	}
	for _, id := range idx.Order {
		n := idx.Nodes[id]
		if n == nil {
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
	return b.String()
}
