package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Deliverable statuses.
const (
	DeliverableOK      = "ok"
	DeliverableMissing = "missing"
	DeliverableEmpty   = "empty"
	DeliverableUnsafe  = "unsafe"
)

// DeliverablesOptions selects a node and the checkout to look in.
type DeliverablesOptions struct {
	Root     string
	FilesDir string
	NodeID   NodeID
}

// DeliverableItem is one declared deliverable and what was found.
type DeliverableItem struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Bytes  int64  `json:"bytes"`
}

// DeliverablesReport is CheckDeliverables' result.
type DeliverablesReport struct {
	NodeID   NodeID            `json:"node_id"`
	Pass     bool              `json:"pass"`
	Declared int               `json:"declared"`
	Items    []DeliverableItem `json:"items"`
	Problems []string          `json:"problems"`
}

// CheckDeliverables reports whether each deliverable the node declares exists in the
// checkout and is non-empty (a file with bytes, or a directory holding a non-empty file).
// A path that escapes the checkout, including through a symlink, is unsafe and never read.
func CheckDeliverables(idx *Index, opts DeliverablesOptions) (DeliverablesReport, error) {
	rep := DeliverablesReport{NodeID: opts.NodeID, Items: []DeliverableItem{}, Problems: []string{}}
	dir, err := checkoutDir(opts.Root, opts.FilesDir)
	if err != nil {
		return rep, err
	}
	idx.mu.Lock()
	n := idx.Nodes[opts.NodeID]
	var paths []string
	if n != nil {
		paths = cloneStrings(n.Deliverables)
	}
	idx.mu.Unlock()
	if n == nil {
		return rep, fmt.Errorf("%w: %q", ErrNotFound, opts.NodeID)
	}
	rep = checkDeliverablePaths(dir, opts.NodeID, paths)
	return rep, nil
}

func checkDeliverablePaths(dir string, id NodeID, paths []string) DeliverablesReport {
	rep := DeliverablesReport{NodeID: id, Declared: len(paths), Items: []DeliverableItem{}, Problems: []string{}}
	for _, p := range paths {
		item := DeliverableItem{Path: p}
		bytes, status := deliverableState(dir, p)
		item.Bytes, item.Status = bytes, status
		switch status {
		case DeliverableMissing:
			rep.Problems = append(rep.Problems, fmt.Sprintf("deliverable %s does not exist", p))
		case DeliverableEmpty:
			rep.Problems = append(rep.Problems, fmt.Sprintf("deliverable %s is empty", p))
		case DeliverableUnsafe:
			rep.Problems = append(rep.Problems, fmt.Sprintf("deliverable %s is outside the checkout", p))
		}
		rep.Items = append(rep.Items, item)
	}
	rep.Pass = len(rep.Problems) == 0
	return rep
}

// deliverableState stats rel under dir without leaving it.
func deliverableState(dir, rel string) (int64, string) {
	cleaned, err := cleanRelPath("deliverables", rel)
	if err != nil || cleaned == "" {
		return 0, DeliverableUnsafe
	}
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return 0, DeliverableMissing
	}
	full := filepath.Join(root, filepath.FromSlash(cleaned))
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, DeliverableMissing
		}
		return 0, DeliverableUnsafe
	}
	if resolved != root && !strings.HasPrefix(resolved, root+string(os.PathSeparator)) {
		return 0, DeliverableUnsafe
	}
	st, err := os.Stat(resolved)
	if err != nil {
		return 0, DeliverableMissing
	}
	if !st.IsDir() {
		if st.Size() == 0 {
			return 0, DeliverableEmpty
		}
		return st.Size(), DeliverableOK
	}
	var total int64
	_ = filepath.WalkDir(resolved, func(p string, d os.DirEntry, err error) error {
		if err != nil || total > 0 {
			return filepath.SkipAll
		}
		if !d.IsDir() {
			if info, ierr := d.Info(); ierr == nil && info.Mode().IsRegular() {
				total = info.Size()
			}
		}
		return nil
	})
	if total == 0 {
		return 0, DeliverableEmpty
	}
	return total, DeliverableOK
}
