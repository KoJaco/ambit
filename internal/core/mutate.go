package core

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Create derives an id from the name, writes the node pair, and records membership.
// The caller cannot supply an id.
func (idx *Index) Create(in CreateInput) (NodeID, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if strings.TrimSpace(in.Name) == "" {
		return "", fmt.Errorf("name must be non-empty")
	}
	if strings.TrimSpace(in.Type) == "" {
		return "", fmt.Errorf("type must be non-empty")
	}
	status := in.Status
	if status == "" {
		status = StatusDraft
	}
	if !validStatus(status) {
		return "", fmt.Errorf("%w: %q (valid: %s)", errBadStatus, status, statusList())
	}
	if in.ParentID != "" {
		if _, ok := idx.Nodes[in.ParentID]; !ok {
			return "", fmt.Errorf("%w: %q", errMissingParent, in.ParentID)
		}
	}
	base, err := slugify(in.Name)
	if err != nil {
		return "", err
	}
	id := idx.allocate(base)
	node := &Node{
		ID:             id,
		Name:           in.Name,
		Type:           in.Type,
		Status:         status,
		ParentID:       in.ParentID,
		Implementation: cloneStrings(in.Implementation),
		Scope:          cloneStrings(in.Scope),
		Protected:      in.Protected,
		Spec:           in.Spec,
	}
	if err := idx.writeNode(node); err != nil {
		_ = idx.removeNodeFiles(id)
		return "", err
	}
	idx.Nodes[id] = node
	idx.Order = append(idx.Order, id)
	if err := idx.saveIndex(); err != nil {
		delete(idx.Nodes, id)
		idx.Order = idx.Order[:len(idx.Order)-1]
		_ = idx.removeNodeFiles(id)
		return "", err
	}
	if err := idx.reload(); err != nil {
		return "", err
	}
	return id, nil
}

// Update changes mutable fields. Renaming changes name only; id and the filename stay.
func (idx *Index) Update(id NodeID, in UpdateInput) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	node, ok := idx.Nodes[id]
	if !ok {
		return fmt.Errorf("%w: %q", errNotFound, id)
	}
	next := *node
	next.Implementation = cloneStrings(node.Implementation)
	next.Scope = cloneStrings(node.Scope)
	next.Unknown = cloneRaw(node.Unknown)

	if in.Name != nil {
		if strings.TrimSpace(*in.Name) == "" {
			return fmt.Errorf("name must be non-empty")
		}
		next.Name = *in.Name
	}
	if in.Type != nil {
		if strings.TrimSpace(*in.Type) == "" {
			return fmt.Errorf("type must be non-empty")
		}
		next.Type = *in.Type
	}
	if in.Status != nil {
		if !validStatus(*in.Status) {
			return fmt.Errorf("%w: %q (valid: %s)", errBadStatus, *in.Status, statusList())
		}
		next.Status = *in.Status
	}
	if in.Parent != nil {
		parent := *in.Parent
		if parent != "" {
			if _, ok := idx.Nodes[parent]; !ok {
				return fmt.Errorf("%w: %q", errMissingParent, parent)
			}
			if idx.parentCycles(id, parent) {
				return fmt.Errorf("%w: %s -> %s", errCycle, id, parent)
			}
		}
		next.ParentID = parent
	}
	if in.Implementation != nil {
		next.Implementation = cloneStrings(*in.Implementation)
	}
	if in.Scope != nil {
		next.Scope = cloneStrings(*in.Scope)
	}
	if in.Protected != nil {
		next.Protected = *in.Protected
	}
	if in.Spec != nil {
		next.Spec = *in.Spec
	}
	if err := idx.writeNode(&next); err != nil {
		return err
	}
	return idx.reload()
}

// Delete removes a node that has no children, and drops relationships that name it.
func (idx *Index) Delete(id NodeID) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, ok := idx.Nodes[id]; !ok {
		return fmt.Errorf("%w: %q", errNotFound, id)
	}
	kids := idx.childIDs(id)
	if len(kids) > 0 {
		return fmt.Errorf("%w: %s has children: %s", errHasChildren, id, strings.Join(kids, ", "))
	}
	var order []NodeID
	for _, existing := range idx.Order {
		if existing != id {
			order = append(order, existing)
		}
	}
	var rels []Relationship
	for _, rel := range idx.Relationships {
		if rel.From == id || rel.To == id {
			continue
		}
		rels = append(rels, rel)
	}
	prevOrder, prevRels := idx.Order, idx.Relationships
	idx.Order = order
	idx.Relationships = rels
	if err := idx.saveIndex(); err != nil {
		idx.Order, idx.Relationships = prevOrder, prevRels
		return err
	}
	if err := idx.removeNodeFiles(id); err != nil {
		return err
	}
	return idx.reload()
}

// SetRelationship creates or updates the directed edge from -> to.
// The opposite direction is a different edge.
func (idx *Index) SetRelationship(from, to NodeID, label, kind string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, ok := idx.Nodes[from]; !ok {
		return fmt.Errorf("%w: %q", errMissingEndpoint, from)
	}
	if _, ok := idx.Nodes[to]; !ok {
		return fmt.Errorf("%w: %q", errMissingEndpoint, to)
	}
	if kind != "" && !validKind(kind) {
		return fmt.Errorf("%w: %s -> %s kind %q", errBadKind, from, to, kind)
	}
	prev := append([]Relationship{}, idx.Relationships...)
	found := false
	for i, rel := range idx.Relationships {
		if rel.From == from && rel.To == to {
			rel.Label = label
			rel.Kind = kind
			idx.Relationships[i] = rel
			found = true
		}
	}
	if !found {
		idx.Relationships = append(idx.Relationships, Relationship{From: from, To: to, Label: label, Kind: kind})
	}
	if err := idx.saveIndex(); err != nil {
		idx.Relationships = prev
		return err
	}
	return idx.reload()
}

// SetAssignment writes the active assignment into local.json.
func (idx *Index) SetAssignment(id NodeID, at time.Time) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, ok := idx.Nodes[id]; !ok {
		return fmt.Errorf("%w: %q", errNotFound, id)
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	prev := idx.Assignment
	idx.Assignment = &Assignment{
		NodeID:     id,
		AssignedAt: at.UTC().Format(time.RFC3339),
		Unknown:    idx.assignmentUnknown,
	}
	if err := idx.saveLocal(); err != nil {
		idx.Assignment = prev
		return err
	}
	return idx.reload()
}

// ClearAssignment removes the assignment. Other local.json keys are kept.
func (idx *Index) ClearAssignment() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	prev := idx.Assignment
	idx.Assignment = nil
	if err := idx.saveLocal(); err != nil {
		idx.Assignment = prev
		return err
	}
	return idx.reload()
}

func (idx *Index) parentCycles(id, parent NodeID) bool {
	if parent == "" {
		return false
	}
	if parent == id {
		return true
	}
	seen := map[NodeID]bool{id: true}
	cur := parent
	for cur != "" {
		if cur == id {
			return true
		}
		if seen[cur] {
			return false
		}
		seen[cur] = true
		n, ok := idx.Nodes[cur]
		if !ok {
			return false
		}
		cur = n.ParentID
	}
	return false
}

func (idx *Index) childIDs(id NodeID) []string {
	var out []string
	for cid, n := range idx.Nodes {
		if n.ParentID == id {
			out = append(out, string(cid))
		}
	}
	sort.Strings(out)
	return out
}
