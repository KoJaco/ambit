package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// applyRename changes a node id on disk: its files, parent_id and depends_on of other
// nodes, index.json (order and relationship endpoints), the machine-local assignment and
// the cached layouts. Implementation globs and prose are left alone. Any failure restores
// every file it touched. Caller holds idx.mu.
func (idx *Index) applyRename(op operation) error {
	old, nw := NodeID(op.NodeID), NodeID(op.NewID)
	if _, ok := idx.Nodes[old]; !ok {
		return fmt.Errorf("%w: %q", ErrNotFound, old)
	}
	if len(nw) > maxExplicitIDLen || !slugPattern.MatchString(string(nw)) {
		return fmt.Errorf("%w: new id %q must be a lowercase slug of at most %d characters", ErrBadOperation, nw, maxExplicitIDLen)
	}
	if idx.taken(string(nw)) {
		return fmt.Errorf("%w: rename_node %s -> %q", ErrExists, old, nw)
	}
	if _, i := idx.relByID(RelID(nw)); i >= 0 {
		return fmt.Errorf("%w: rename_node %s -> %q is already a relationship id", ErrExists, old, nw)
	}
	snap, err := idx.snapshotModel()
	if err != nil {
		return err
	}
	fail := func(err error) error {
		if rerr := idx.restoreFiles(snap); rerr != nil {
			return fmt.Errorf("%w (restore failed: %v)", err, rerr)
		}
		_ = idx.reload()
		return err
	}

	// Node files: the renamed node under its new id, and every node that names it.
	for _, n := range idx.Nodes {
		cp := cloneNode(n)
		touched := false
		if cp.ID == old {
			cp.ID = nw
			touched = true
		}
		if cp.ParentID == old {
			cp.ParentID = nw
			touched = true
		}
		if renamed := renameIDs(cp.DependsOn, old, nw); !slicesEqualIDsOrdered(renamed, cp.DependsOn) {
			cp.DependsOn = renamed
			touched = true
		}
		if renamed := renameIDs(cp.Gates, old, nw); !slicesEqualIDsOrdered(renamed, cp.Gates) {
			cp.Gates = renamed
			touched = true
		}
		if !touched {
			continue
		}
		if err := idx.writeNode(cp); err != nil {
			return fail(err)
		}
	}
	if err := failApply("node-json"); err != nil {
		return fail(err)
	}
	if err := idx.removeNodeFiles(old); err != nil {
		return fail(err)
	}

	for i, id := range idx.Order {
		if id == old {
			idx.Order[i] = nw
		}
	}
	for i := range idx.Relationships {
		if idx.Relationships[i].From == old {
			idx.Relationships[i].From = nw
		}
		if idx.Relationships[i].To == old {
			idx.Relationships[i].To = nw
		}
	}
	if err := idx.saveIndex(); err != nil {
		return fail(err)
	}
	if err := failApply("index"); err != nil {
		return fail(err)
	}
	if idx.Assignment != nil && idx.Assignment.NodeID == old {
		idx.Assignment.NodeID = nw
		if err := idx.saveLocal(); err != nil {
			return fail(err)
		}
	}
	if err := failApply("local"); err != nil {
		return fail(err)
	}
	if err := idx.renameLayouts(old, nw); err != nil {
		return fail(err)
	}
	if err := failApply("layouts"); err != nil {
		return fail(err)
	}
	return idx.reload()
}

// slicesEqualIDsOrdered compares two id lists element by element.
func slicesEqualIDsOrdered(a, b []NodeID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// renameLayouts moves the layout cache of old (and its hashed variants are dropped) to nw
// and rewrites node ids inside every cached layout. Edge attachments of a layout that
// contained the node are dropped; the canvas recomputes them.
func (idx *Index) renameLayouts(old, nw NodeID) error {
	dir := idx.layoutDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		key := strings.TrimSuffix(name, ".json")
		path := filepath.Join(dir, name)
		if strings.HasPrefix(key, string(old)+"~") {
			// Hashed variants are keyed by the member ids; drop them.
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var layout Layout
		if json.Unmarshal(data, &layout) != nil {
			continue
		}
		changed := false
		for i := range layout.Positions {
			if layout.Positions[i].ID == string(old) {
				layout.Positions[i].ID = string(nw)
				changed = true
			}
		}
		if ports, ok := layout.Ports[string(old)]; ok {
			delete(layout.Ports, string(old))
			layout.Ports[string(nw)] = ports
			changed = true
		}
		if changed {
			layout.Attachments = nil
			out, err := json.Marshal(layout)
			if err != nil {
				return err
			}
			if err := writeAtomic(path, append(out, '\n')); err != nil {
				return err
			}
		}
		if key == string(old) {
			if err := os.Rename(path, filepath.Join(dir, string(nw)+".json")); err != nil {
				return err
			}
		}
	}
	return nil
}
