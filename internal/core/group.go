package core

import (
	"fmt"
	"strings"
)

// GroupInput describes a group-selection proposal: NodeIDs move under a new container.
type GroupInput struct {
	NodeIDs []NodeID
	Name    string
	Type    string // defaults to "container"
	Icon    string
	Color   string
}

// UngroupInput describes an ungroup proposal. The container is kept as a leaf unless
// DeleteContainer is set.
type UngroupInput struct {
	NodeID          NodeID
	DeleteContainer bool
}

// DefaultContainerType is the type given to a container created by grouping.
const DefaultContainerType = "container"

// StageGroup stages one proposal: create the container under the shared parent of the
// selection, then update parent_id on each selected node. It returns the proposal id.
func (idx *Index) StageGroup(in GroupInput) (string, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if strings.TrimSpace(in.Name) == "" {
		return "", ErrEmptyName
	}
	if len(in.NodeIDs) < 2 {
		return "", fmt.Errorf("%w: group needs at least 2 nodes, got %d", ErrBadOperation, len(in.NodeIDs))
	}
	typ := strings.TrimSpace(in.Type)
	if typ == "" {
		typ = DefaultContainerType
	}
	seen := map[NodeID]bool{}
	for _, id := range in.NodeIDs {
		if seen[id] {
			return "", fmt.Errorf("%w: duplicate node %q in group", ErrBadOperation, id)
		}
		seen[id] = true
		if _, ok := idx.Nodes[id]; !ok {
			return "", fmt.Errorf("%w: %q", ErrNotFound, id)
		}
	}
	for _, id := range in.NodeIDs {
		for anc := idx.Nodes[id].ParentID; anc != ""; anc = idx.Nodes[anc].ParentID {
			if seen[anc] {
				return "", fmt.Errorf("%w: %s is an ancestor of %s; select only one of them", ErrBadOperation, anc, id)
			}
		}
	}
	first := idx.Nodes[in.NodeIDs[0]]
	parent := first.ParentID
	for _, id := range in.NodeIDs {
		n := idx.Nodes[id]
		if n.RelationshipID != "" {
			return "", fmt.Errorf("%w: %s lives inside a relationship and cannot be grouped", ErrBadOperation, id)
		}
		if n.ParentID != parent {
			return "", fmt.Errorf("%w: selected nodes must share one parent (%s is under %q, %s is under %q)",
				ErrBadOperation, first.ID, parent, id, n.ParentID)
		}
	}
	base, err := slugify(in.Name)
	if err != nil {
		return "", err
	}
	containerID := idx.allocateDuring(base, nil)

	ops := []StageOp{{
		Op: OpCreateNode,
		Create: CreateInput{
			Name:     in.Name,
			Type:     typ,
			Status:   StatusDraft,
			ParentID: parent,
			Icon:     in.Icon,
			Color:    in.Color,
		},
	}}
	for _, id := range in.NodeIDs {
		to := containerID
		ops = append(ops, StageOp{Op: OpUpdateNode, NodeID: id, Update: UpdateInput{Parent: &to}})
	}
	return idx.stageLocked(StageInput{
		Source:  SourceGroupNodes,
		Summary: fmt.Sprintf("Group %d nodes into %q", len(in.NodeIDs), in.Name),
		Ops:     ops,
	})
}

// StageUngroup stages one proposal: move each direct child of the container to the
// container's parent (or into its relationship), then optionally delete the container.
// Deleting a container that has relationships attached is refused, because deleting a
// node drops its relationships; keep the container or remove the relationships first.
func (idx *Index) StageUngroup(in UngroupInput) (string, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	c, ok := idx.Nodes[in.NodeID]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrNotFound, in.NodeID)
	}
	kids := idx.childIDs(in.NodeID)
	if len(kids) == 0 {
		return "", fmt.Errorf("%w: %s has no children to ungroup", ErrBadOperation, in.NodeID)
	}
	if in.DeleteContainer {
		for _, rel := range idx.Relationships {
			if rel.From == in.NodeID || rel.To == in.NodeID {
				return "", fmt.Errorf("%w: %s has relationships attached; deleting it would drop them (keep the container instead)",
					ErrBadOperation, in.NodeID)
			}
		}
	}
	var ops []StageOp
	for _, kid := range kids {
		up := UpdateInput{}
		parent := c.ParentID
		up.Parent = &parent
		if c.RelationshipID != "" {
			rel := c.RelationshipID
			up.Relationship = &rel
		}
		ops = append(ops, StageOp{Op: OpUpdateNode, NodeID: NodeID(kid), Update: up})
	}
	summary := fmt.Sprintf("Ungroup %q", c.Name)
	if in.DeleteContainer {
		ops = append(ops, StageOp{Op: OpDeleteNode, NodeID: in.NodeID})
		summary += " and delete the container"
	}
	return idx.stageLocked(StageInput{Source: SourceUngroupNode, Summary: summary, Ops: ops})
}

// StageReparent stages a single-operation proposal moving a node under newParent
// ("" moves it to the root). Used when a node is dragged out of its group.
func (idx *Index) StageReparent(id, newParent NodeID) (string, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	n, ok := idx.Nodes[id]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	if n.ParentID == newParent && n.RelationshipID == "" {
		return "", fmt.Errorf("%w: %s is already under %q", ErrBadOperation, id, newParent)
	}
	return idx.stageLocked(StageInput{
		Source:  SourceReparentNode,
		Summary: fmt.Sprintf("Move %q", n.Name),
		Ops:     []StageOp{{Op: OpUpdateNode, NodeID: id, Update: UpdateInput{Parent: &newParent}}},
	})
}
