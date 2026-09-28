package core

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

const (
	KindAllowed       = "allowed"
	KindViolation     = "violation"
	KindInformational = "informational"

	RuleProtected    = "protected"
	RuleOutsideScope = "outside_scope"
)

// Hit is one rule failure for a path. Node is the node that owns the file
// when the path matches an implementation glob, or the assigned node when
// the path is unmapped.
type Hit struct {
	Rule   string
	Node   NodeID
	Detail string
}

// FileResult is the verdict for one path. Allowed and informational results
// have no hits. A violation can carry more than one hit.
type FileResult struct {
	Path string
	Kind string
	Hits []Hit
}

// DanglingAssignmentError means the assignment id is not in the index.
// CheckScope does not treat that as "no assignment".
type DanglingAssignmentError struct {
	ID NodeID
}

func (e *DanglingAssignmentError) Error() string {
	return fmt.Sprintf("assignment node_id %q does not exist", e.ID)
}

// CheckScope judges files against the index. An empty assignment means no
// assignment. The function does not call git and does not read or write
// local.json. It is safe to call while Watch runs on idx.
func CheckScope(assignment NodeID, files []string, idx *Index) ([]FileResult, error) {
	if idx == nil {
		return nil, errors.New("no index")
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return checkScopeLocked(assignment, files, idx)
}

func checkScopeLocked(assignment NodeID, files []string, idx *Index) ([]FileResult, error) {
	var assigned *Node
	if assignment != "" {
		n, ok := idx.Nodes[assignment]
		if !ok || n == nil {
			return nil, &DanglingAssignmentError{ID: assignment}
		}
		assigned = n
	}
	if err := validateGlobs(idx); err != nil {
		return nil, err
	}
	out := make([]FileResult, 0, len(files))
	for _, path := range files {
		res, err := judgePath(assignment, assigned, toSlash(path), idx)
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, nil
}

func validateGlobs(idx *Index) error {
	for _, id := range idx.Order {
		n := idx.Nodes[id]
		if n == nil {
			continue
		}
		if err := validateNodeGlobs(id, "implementation", n.Implementation); err != nil {
			return err
		}
		if err := validateNodeGlobs(id, "scope", n.Scope); err != nil {
			return err
		}
	}
	return nil
}

func validateNodeGlobs(id NodeID, field string, patterns []string) error {
	for _, p := range patterns {
		p = toSlash(p)
		if p == "" {
			continue
		}
		if _, err := doublestar.Match(p, "x"); err != nil {
			return fmt.Errorf("node %s: %s: invalid glob %q", id, field, p)
		}
	}
	return nil
}

func judgePath(assignment NodeID, assigned *Node, path string, idx *Index) (FileResult, error) {
	var protected []Hit
	var owners []NodeID
	for _, id := range idx.Order {
		n := idx.Nodes[id]
		if n == nil {
			continue
		}
		matched, err := matchGlobs(n.Implementation, path)
		if err != nil {
			return FileResult{}, fmt.Errorf("node %s: implementation: %w", id, err)
		}
		if !matched {
			continue
		}
		if n.Protected {
			protected = append(protected, Hit{
				Rule:   RuleProtected,
				Node:   id,
				Detail: fmt.Sprintf("matches protected node %s", id),
			})
			continue
		}
		owners = append(owners, id)
	}

	res := FileResult{Path: path}
	if assignment == "" {
		if len(protected) > 0 {
			res.Kind = KindViolation
			res.Hits = protected
			return res, nil
		}
		if len(owners) == 0 {
			res.Kind = KindInformational
			return res, nil
		}
		res.Kind = KindAllowed
		return res, nil
	}

	inScope, err := matchGlobs(EffectiveScope(assigned), path)
	if err != nil {
		return FileResult{}, fmt.Errorf("node %s: scope: %w", assignment, err)
	}
	if inScope && len(protected) == 0 {
		res.Kind = KindAllowed
		return res, nil
	}
	res.Kind = KindViolation
	res.Hits = append(res.Hits, protected...)
	if !inScope {
		unmapped := len(protected) == 0 && len(owners) == 0
		res.Hits = append(res.Hits, outsideScopeHits(assignment, owners, unmapped)...)
	}
	return res, nil
}

func outsideScopeHits(assignment NodeID, owners []NodeID, unmapped bool) []Hit {
	if len(owners) == 0 {
		detail := fmt.Sprintf("not in the scope declared for %s", assignment)
		if unmapped {
			detail = "matches no node; " + detail
		}
		return []Hit{{
			Rule:   RuleOutsideScope,
			Node:   assignment,
			Detail: detail,
		}}
	}
	hits := make([]Hit, 0, len(owners))
	for _, id := range owners {
		hits = append(hits, Hit{
			Rule:   RuleOutsideScope,
			Node:   id,
			Detail: fmt.Sprintf("belongs to %s; not in the scope declared for %s", id, assignment),
		})
	}
	return hits
}

func matchGlobs(patterns []string, path string) (bool, error) {
	for _, p := range patterns {
		p = toSlash(p)
		if p == "" {
			continue
		}
		ok, err := doublestar.Match(p, path)
		if err != nil {
			return false, fmt.Errorf("invalid glob %q: %w", p, err)
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func toSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}
