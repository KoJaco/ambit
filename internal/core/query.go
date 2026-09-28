package core

import "fmt"

// Level is one drill-down: a node, its direct children, and the relationships
// whose endpoints are both in that child set. Node is nil at the root.
// Relationships that leave the level are Crossings, not edges, and the other
// endpoint is an id only.
type Level struct {
	Node          *NodeView
	Children      []NodeView
	Relationships []EdgeView
	Crossings     []Crossing
	Warnings      []Diagnostic
}

// NodeView is a copy of one node, safe to read after the index lock is released.
type NodeView struct {
	ID             NodeID
	Name           string
	Type           string
	Status         Status
	ParentID       NodeID
	RelationshipID RelID
	Implementation []string
	Scope          []string
	Protected      bool
	Spec           string
}

// EdgeView is a relationship with both endpoints on the current level.
type EdgeView struct {
	ID        RelID
	From      NodeID
	To        NodeID
	Label     string
	Kind      string
	Drillable bool
}

// Crossing is a relationship with exactly one endpoint on the current level.
// Direction is "out" when the on-screen node is the source, and "in" when it
// is the target. OtherID is not a node in this response.
type Crossing struct {
	NodeID    NodeID
	Direction string
	Label     string
	Kind      string
	OtherID   NodeID
	RelID     RelID
	Drillable bool
}

// Warnings copies the integrity diagnostics. They are warnings, not load failures.
func (idx *Index) Warnings() []Diagnostic {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	out := append([]Diagnostic{}, idx.Diagnostics...)
	if out == nil {
		out = []Diagnostic{}
	}
	return out
}

// Node returns one node, including its markdown. An unknown id is ErrNotFound.
func (idx *Index) Node(id NodeID) (NodeView, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	n, ok := idx.Nodes[id]
	if !ok || n == nil {
		return NodeView{}, fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	return snapshot(n), nil
}

// Level returns the drill-down for parent. An empty parent is the root: nodes
// with no parent_id. An unknown parent is ErrNotFound.
func (idx *Index) Level(parent NodeID) (Level, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	var lvl Level
	lvl.Warnings = append([]Diagnostic{}, idx.Diagnostics...)
	if lvl.Warnings == nil {
		lvl.Warnings = []Diagnostic{}
	}

	var ids []NodeID
	if parent == "" {
		ids = idx.Roots
	} else {
		n, ok := idx.Nodes[parent]
		if !ok || n == nil {
			return Level{}, fmt.Errorf("%w: %q", ErrNotFound, parent)
		}
		view := snapshot(n)
		lvl.Node = &view
		ids = idx.Children[parent]
	}

	set := map[NodeID]bool{}
	lvl.Children = make([]NodeView, 0, len(ids))
	for _, id := range ids {
		n := idx.Nodes[id]
		if n == nil {
			continue
		}
		set[id] = true
		lvl.Children = append(lvl.Children, snapshot(n))
	}

	lvl.Relationships = []EdgeView{}
	lvl.Crossings = []Crossing{}
	for _, rel := range idx.ActiveEdges {
		fromIn := set[rel.From]
		toIn := set[rel.To]
		switch {
		case fromIn && toIn:
			ev := edgeView(rel)
			ev.Drillable = idx.isDrillable(rel.ID)
			lvl.Relationships = append(lvl.Relationships, ev)
		case fromIn:
			lvl.Crossings = append(lvl.Crossings, Crossing{
				NodeID: rel.From, Direction: "out", Label: rel.Label, Kind: rel.Kind, OtherID: rel.To,
				RelID: rel.ID, Drillable: idx.isDrillable(rel.ID),
			})
		case toIn:
			lvl.Crossings = append(lvl.Crossings, Crossing{
				NodeID: rel.To, Direction: "in", Label: rel.Label, Kind: rel.Kind, OtherID: rel.From,
				RelID: rel.ID, Drillable: idx.isDrillable(rel.ID),
			})
		}
	}
	return lvl, nil
}

func snapshot(n *Node) NodeView {
	return NodeView{
		ID:             n.ID,
		Name:           n.Name,
		Type:           n.Type,
		Status:         n.Status,
		ParentID:       n.ParentID,
		RelationshipID: n.RelationshipID,
		Implementation: cloneStrings(n.Implementation),
		Scope:          cloneStrings(n.Scope),
		Protected:      n.Protected,
		Spec:           n.Spec,
	}
}
