package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RootLayoutKey is the layout cache key for `/`, which has no node id.
// Node ids match ^[a-z0-9]+(-[a-z0-9]+)*$, so this cannot collide with one.
const RootLayoutKey = "_root"

// Position is one node's presentation coordinate. It is not part of the model.
type Position struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

// Layout is the cached positions for one drill-down level.
type Layout struct {
	Positions   []Position                  `json:"positions"`
	Ports       map[string]NodePorts        `json:"ports,omitempty"`
	Attachments map[string]EdgeAttachment   `json:"attachments,omitempty"`
}

// NodePorts counts connection points on each side of a node card.
type NodePorts struct {
	Left  int `json:"left"`
	Right int `json:"right"`
}

// EdgeAttachment binds a relationship id to port coordinates on its endpoints.
type EdgeAttachment struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// Layout returns cached positions for key. A missing file is an empty layout.
func (idx *Index) Layout(key string) (Layout, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	path, err := idx.layoutPath(key)
	if err != nil {
		return Layout{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Layout{Positions: []Position{}}, nil
		}
		return Layout{}, err
	}
	var layout Layout
	if err := json.Unmarshal(data, &layout); err != nil {
		return Layout{}, fmt.Errorf("%s: %w", path, err)
	}
	if layout.Positions == nil {
		layout.Positions = []Position{}
	}
	return layout, nil
}

// PutLayout writes positions under .arch/.cache/layout/. It does not touch
// nodes/ or index.json.
func (idx *Index) PutLayout(key string, layout Layout) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	path, err := idx.layoutPath(key)
	if err != nil {
		return err
	}
	if layout.Positions == nil {
		layout.Positions = []Position{}
	}
	data, err := json.Marshal(layout)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeAtomic(path, data)
}

func (idx *Index) layoutPath(key string) (string, error) {
	if key == "" || key == "." || key == ".." || strings.ContainsAny(key, `/\`) {
		return "", fmt.Errorf("%w: %q", ErrInvalidLayoutKey, key)
	}
	return filepath.Join(idx.arch(), ".cache", "layout", key+".json"), nil
}

// invalidateLayoutLocked removes the cache for the level whose parent is parent.
// An empty parent is the root. Caller holds idx.mu.
func (idx *Index) invalidateLayoutLocked(parent NodeID) {
	key := RootLayoutKey
	if parent != "" {
		key = string(parent)
	}
	idx.invalidateLayoutKeyLocked(key)
}

func (idx *Index) invalidateLayoutKeyLocked(key string) {
	path, err := idx.layoutPath(key)
	if err != nil {
		return
	}
	_ = os.Remove(path)
}
