package core

import (
	"fmt"
	"sort"
)

// Defaults for the config.json view settings (decision 0010).
const (
	DefaultGroupMax    = 8
	DefaultLevelBudget = 24
	// maxExpandDepth is the deepest level (children of the level are 1) at which
	// a group may be expanded.
	maxExpandDepth = 2
)

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
	ViewDefaults  ViewDefaults
}

// ViewDefaults are the effective config.json view settings.
type ViewDefaults struct {
	GroupMax    int
	LevelBudget int
}

// TypeCount is one entry of a node's child type mix.
type TypeCount struct {
	Type  string
	Count int
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
	Icon           string
	Color          string
	Renderer       string
	Schema         []SchemaColumn
	ChildCount     int
	// GroupID is the parent when this node is shown as a member of an expanded group.
	GroupID NodeID
	// Expanded is true on a node shown as an open group.
	Expanded bool
	// InternalRelCount counts active relationships hidden inside a collapsed node.
	InternalRelCount int
	// TypeMix is the direct children's types by count desc, then name.
	TypeMix []TypeCount
}

// EdgeView is a relationship with both endpoints on the current level.
type EdgeView struct {
	ID        RelID
	From      NodeID
	To        NodeID
	Label     string
	Kind      string
	Drillable bool
	// Aggregated edges merge rolled-up relationships between one anchor pair.
	Aggregated bool
	Members    []RelID
	Kinds      []string
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
//
// expanded lists group nodes the client shows open. See visibleSet for which
// ids are honored. Relationships are rolled up onto the nearest visible
// ancestor (the anchor) of each endpoint.
func (idx *Index) Level(parent NodeID, expanded []NodeID) (Level, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	var lvl Level
	lvl.Warnings = append([]Diagnostic{}, idx.Diagnostics...)
	if lvl.Warnings == nil {
		lvl.Warnings = []Diagnostic{}
	}
	lvl.ViewDefaults = ViewDefaults{GroupMax: idx.GroupMax, LevelBudget: idx.LevelBudget}
	if lvl.ViewDefaults.GroupMax <= 0 {
		lvl.ViewDefaults.GroupMax = DefaultGroupMax
	}
	if lvl.ViewDefaults.LevelBudget <= 0 {
		lvl.ViewDefaults.LevelBudget = DefaultLevelBudget
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

	lvl.Children = idx.visibleSet(ids, expanded)
	visible := make(map[NodeID]*NodeView, len(lvl.Children))
	for i := range lvl.Children {
		visible[lvl.Children[i].ID] = &lvl.Children[i]
	}

	type pairKey struct{ from, to NodeID }
	type aggregate struct {
		edge int // index into lvl.Relationships
		rels []Relationship
	}
	aggs := map[pairKey]*aggregate{}
	anchors := map[NodeID]NodeID{}
	anchor := func(id NodeID) NodeID {
		if a, ok := anchors[id]; ok {
			return a
		}
		a := idx.anchorOf(id, visible)
		anchors[id] = a
		return a
	}

	lvl.Relationships = []EdgeView{}
	lvl.Crossings = []Crossing{}
	for _, rel := range idx.ActiveEdges {
		from, to := anchor(rel.From), anchor(rel.To)
		switch {
		case from != "" && to != "" && from != to:
			if from == rel.From && to == rel.To {
				ev := edgeView(rel)
				ev.Drillable = idx.isDrillable(rel.ID)
				lvl.Relationships = append(lvl.Relationships, ev)
				continue
			}
			key := pairKey{from, to}
			agg := aggs[key]
			if agg == nil {
				agg = &aggregate{edge: len(lvl.Relationships)}
				aggs[key] = agg
				lvl.Relationships = append(lvl.Relationships, EdgeView{
					ID: RelID("agg:" + string(from) + ">" + string(to)), From: from, To: to, Aggregated: true,
				})
			}
			agg.rels = append(agg.rels, rel)
		case from != "" && to != "":
			if v := visible[from]; v != nil && !v.Expanded && v.ChildCount > 0 {
				v.InternalRelCount++
			}
		case from != "":
			lvl.Crossings = append(lvl.Crossings, Crossing{
				NodeID: from, Direction: "out", Label: rel.Label, Kind: rel.Kind, OtherID: rel.To,
				RelID: rel.ID, Drillable: idx.isDrillable(rel.ID),
			})
		case to != "":
			lvl.Crossings = append(lvl.Crossings, Crossing{
				NodeID: to, Direction: "in", Label: rel.Label, Kind: rel.Kind, OtherID: rel.From,
				RelID: rel.ID, Drillable: idx.isDrillable(rel.ID),
			})
		}
	}
	for _, agg := range aggs {
		ev := &lvl.Relationships[agg.edge]
		kinds := map[string]bool{}
		for _, rel := range agg.rels {
			ev.Members = append(ev.Members, rel.ID)
			if rel.Kind != "" {
				kinds[rel.Kind] = true
			}
		}
		sort.Slice(ev.Members, func(i, j int) bool { return ev.Members[i] < ev.Members[j] })
		for k := range kinds {
			ev.Kinds = append(ev.Kinds, k)
		}
		sort.Strings(ev.Kinds)
		if len(ev.Members) == 1 {
			ev.Label = "1 relationship"
			ev.Drillable = idx.isDrillable(ev.Members[0])
		} else {
			ev.Label = fmt.Sprintf("%d relationships", len(ev.Members))
		}
		if len(ev.Kinds) == 1 {
			ev.Kind = ev.Kinds[0]
		}
	}
	return lvl, nil
}

// visibleSet returns the level's children followed, depth first, by the members
// of every honored expanded group, as a flat list. An expanded id is honored
// only when the node is already visible, has children, and sits at most two
// levels below the level. Everything else is ignored, so duplicates and order
// do not matter.
func (idx *Index) visibleSet(roots []NodeID, expanded []NodeID) []NodeView {
	want := make(map[NodeID]bool, len(expanded))
	for _, id := range expanded {
		want[id] = true
	}
	out := make([]NodeView, 0, len(roots))
	var visit func(id, group NodeID, depth int)
	visit = func(id, group NodeID, depth int) {
		n := idx.Nodes[id]
		if n == nil {
			return
		}
		view := snapshot(n)
		kids := idx.Children[id]
		view.GroupID = group
		view.ChildCount = len(kids)
		view.TypeMix = idx.typeMix(kids)
		view.Expanded = len(kids) > 0 && depth <= maxExpandDepth && want[id]
		out = append(out, view)
		if view.Expanded {
			for _, kid := range kids {
				visit(kid, id, depth+1)
			}
		}
	}
	for _, id := range roots {
		visit(id, "", 1)
	}
	return out
}

// anchorOf is the nearest ancestor-or-self of id that is visible, or "" when
// id is outside the visible subtree.
func (idx *Index) anchorOf(id NodeID, visible map[NodeID]*NodeView) NodeID {
	cur := id
	for steps := 0; cur != "" && steps <= len(idx.Nodes); steps++ {
		if visible[cur] != nil {
			return cur
		}
		n := idx.Nodes[cur]
		if n == nil {
			return ""
		}
		cur = n.ParentID
	}
	return ""
}

func (idx *Index) typeMix(kids []NodeID) []TypeCount {
	if len(kids) == 0 {
		return nil
	}
	counts := map[string]int{}
	for _, id := range kids {
		if n := idx.Nodes[id]; n != nil {
			counts[n.Type]++
		}
	}
	mix := make([]TypeCount, 0, len(counts))
	for t, c := range counts {
		mix = append(mix, TypeCount{Type: t, Count: c})
	}
	sort.Slice(mix, func(i, j int) bool {
		if mix[i].Count != mix[j].Count {
			return mix[i].Count > mix[j].Count
		}
		return mix[i].Type < mix[j].Type
	})
	return mix
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
		Icon:           n.Icon,
		Color:          n.Color,
		Renderer:       n.Renderer,
		Schema:         append([]SchemaColumn(nil), n.Schema...),
	}
}
