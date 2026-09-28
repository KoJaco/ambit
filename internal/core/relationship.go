package core

import (
	"fmt"
	"strings"
)

// RelID is the stable identifier for a cross-cutting relationship.
type RelID string

// RelationshipLayoutKey returns the layout cache key for a relationship level.
func RelationshipLayoutKey(id RelID) string {
	return "_rel_" + string(id)
}

// SetRelationshipInput creates or updates a relationship. Empty ID always creates.
type SetRelationshipInput struct {
	ID    RelID
	From  NodeID
	To    NodeID
	Label string
	Kind  string
}

func (idx *Index) relationshipTaken(id string) bool {
	if idx.taken(id) {
		return true
	}
	for _, rel := range idx.Relationships {
		if string(rel.ID) == id {
			return true
		}
	}
	return false
}

func (idx *Index) allocateRelationshipID(label string) (RelID, error) {
	base := "relationship"
	if strings.TrimSpace(label) != "" {
		s, err := slugify(label)
		if err == nil {
			base = s
		}
	}
	if !idx.relationshipTaken(base) {
		return RelID(base), nil
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if !idx.relationshipTaken(candidate) {
			return RelID(candidate), nil
		}
	}
}

// ensureRelationshipIDs assigns ids in memory for relationships loaded without one.
func (idx *Index) ensureRelationshipIDs() {
	for i := range idx.Relationships {
		if idx.Relationships[i].ID != "" {
			continue
		}
		id, err := idx.allocateRelationshipID(idx.Relationships[i].Label)
		if err != nil {
			continue
		}
		idx.Relationships[i].ID = id
	}
}

func (idx *Index) relByID(id RelID) (*Relationship, int) {
	for i := range idx.Relationships {
		if idx.Relationships[i].ID == id {
			return &idx.Relationships[i], i
		}
	}
	return nil, -1
}

func (idx *Index) memberIDs(relID RelID) []NodeID {
	var out []NodeID
	for id, n := range idx.Nodes {
		if n == nil {
			continue
		}
		if n.RelationshipID == relID && n.ParentID == "" {
			out = append(out, id)
		}
	}
	sortNodeIDs(out)
	return out
}

func (idx *Index) memberCount(relID RelID) int {
	return len(idx.memberIDs(relID))
}

func (idx *Index) isDrillable(relID RelID) bool {
	return idx.memberCount(relID) > 0
}

func sortNodeIDs(ids []NodeID) {
	if len(ids) < 2 {
		return
	}
	// small n; simple sort
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[j] < ids[i] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
}

func (idx *Index) allocateRelationshipIDStaged(label string, staged []Relationship) (RelID, error) {
	saved := idx.Relationships
	idx.Relationships = append(append([]Relationship{}, saved...), staged...)
	defer func() { idx.Relationships = saved }()
	return idx.allocateRelationshipID(label)
}

func (idx *Index) validateContainer(n *Node) error {
	if n.ParentID != "" && n.RelationshipID != "" {
		return fmt.Errorf("node %s cannot have both parent_id and relationship_id", n.ID)
	}
	if n.RelationshipID != "" {
		if _, i := idx.relByID(n.RelationshipID); i < 0 {
			return fmt.Errorf("%w: relationship %q", ErrNotFound, n.RelationshipID)
		}
	}
	return nil
}
