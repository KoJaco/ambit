package core

import "fmt"

// RelationshipLevel is one relationship interior: members, edges among them, crossings, endpoints.
type RelationshipLevel struct {
	Relationship RelView
	Members      []NodeView
	Context      [2]NodeView
	Relationships []EdgeView
	Crossings    []Crossing
	Warnings     []Diagnostic
}

// RelView is a relationship summary for API responses.
type RelView struct {
	ID    RelID
	From  NodeID
	To    NodeID
	Label string
	Kind  string
}

// RelationshipLevel returns the drill-down for a relationship with members. Empty interior is ErrEmptyRelationship.
func (idx *Index) RelationshipLevel(relID RelID) (RelationshipLevel, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	var lvl RelationshipLevel
	lvl.Warnings = append([]Diagnostic{}, idx.Diagnostics...)
	if lvl.Warnings == nil {
		lvl.Warnings = []Diagnostic{}
	}

	rel, _ := idx.relByID(relID)
	if rel == nil {
		return RelationshipLevel{}, fmt.Errorf("%w: %q", ErrRelationshipNotFound, relID)
	}
	members := idx.memberIDs(relID)
	if len(members) == 0 {
		return RelationshipLevel{}, fmt.Errorf("%w: %q", ErrEmptyRelationship, relID)
	}
	lvl.Relationship = relView(*rel)
	fromView, ok := idx.Nodes[rel.From]
	if !ok || fromView == nil {
		return RelationshipLevel{}, fmt.Errorf("%w: %q", errMissingEndpoint, rel.From)
	}
	toView, ok := idx.Nodes[rel.To]
	if !ok || toView == nil {
		return RelationshipLevel{}, fmt.Errorf("%w: %q", errMissingEndpoint, rel.To)
	}
	lvl.Context[0] = snapshot(fromView)
	lvl.Context[1] = snapshot(toView)

	set := map[NodeID]bool{}
	for _, id := range members {
		n := idx.Nodes[id]
		if n == nil {
			continue
		}
		set[id] = true
		lvl.Members = append(lvl.Members, snapshot(n))
	}

	lvl.Relationships = []EdgeView{}
	lvl.Crossings = []Crossing{}
	for _, edge := range idx.ActiveEdges {
		fromIn := set[edge.From]
		toIn := set[edge.To]
		ev := edgeView(edge)
		switch {
		case fromIn && toIn:
			ev.Drillable = idx.isDrillable(edge.ID)
			lvl.Relationships = append(lvl.Relationships, ev)
		case fromIn:
			lvl.Crossings = append(lvl.Crossings, Crossing{
				NodeID: edge.From, Direction: "out", Label: edge.Label, Kind: edge.Kind,
				OtherID: edge.To, RelID: edge.ID, Drillable: idx.isDrillable(edge.ID),
			})
		case toIn:
			lvl.Crossings = append(lvl.Crossings, Crossing{
				NodeID: edge.To, Direction: "in", Label: edge.Label, Kind: edge.Kind,
				OtherID: edge.From, RelID: edge.ID, Drillable: idx.isDrillable(edge.ID),
			})
		}
	}
	return lvl, nil
}

func relView(rel Relationship) RelView {
	return RelView{ID: rel.ID, From: rel.From, To: rel.To, Label: rel.Label, Kind: rel.Kind}
}

func edgeView(rel Relationship) EdgeView {
	return EdgeView{
		ID: rel.ID, From: rel.From, To: rel.To, Label: rel.Label, Kind: rel.Kind,
		Drillable: false,
	}
}
