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
		return "", errEmptyName
	}
	if strings.TrimSpace(in.Type) == "" {
		return "", errEmptyType
	}
	status := in.Status
	if status == "" {
		status = StatusDraft
	}
	if !validStatus(status) {
		return "", fmt.Errorf("%w: %q (valid: %s)", errBadStatus, status, statusList())
	}
	if in.ParentID != "" && in.RelationshipID != "" {
		return "", ErrContainerConflict
	}
	if in.ParentID != "" {
		if _, ok := idx.Nodes[in.ParentID]; !ok {
			return "", fmt.Errorf("%w: %q", errMissingParent, in.ParentID)
		}
	}
	if in.RelationshipID != "" {
		if _, i := idx.relByID(in.RelationshipID); i < 0 {
			return "", fmt.Errorf("%w: %q", ErrRelationshipNotFound, in.RelationshipID)
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
		RelationshipID: in.RelationshipID,
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
	if in.RelationshipID != "" {
		idx.invalidateLayoutKeyLocked(RelationshipLayoutKey(in.RelationshipID))
	} else {
		idx.invalidateLayoutLocked(in.ParentID)
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
			return errEmptyName
		}
		next.Name = *in.Name
	}
	if in.Type != nil {
		if strings.TrimSpace(*in.Type) == "" {
			return errEmptyType
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
		if parent != "" {
			next.RelationshipID = ""
		}
	}
	if in.Relationship != nil {
		rid := *in.Relationship
		if rid != "" {
			if _, i := idx.relByID(rid); i < 0 {
				return fmt.Errorf("%w: %q", ErrRelationshipNotFound, rid)
			}
			if next.ParentID != "" {
				return ErrContainerConflict
			}
		}
		next.RelationshipID = rid
		if rid != "" {
			next.ParentID = ""
		}
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
	if err := idx.reload(); err != nil {
		return err
	}
	if in.Parent != nil && *in.Parent != node.ParentID {
		idx.invalidateLayoutLocked(node.ParentID)
		idx.invalidateLayoutLocked(*in.Parent)
	}
	return nil
}

// Delete removes a node that has no children, and drops relationships that name it.
func (idx *Index) Delete(id NodeID) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	node, ok := idx.Nodes[id]
	if !ok {
		return fmt.Errorf("%w: %q", errNotFound, id)
	}
	parent := node.ParentID
	relParent := node.RelationshipID
	kids := idx.childIDs(id)
	if len(kids) > 0 {
		return fmt.Errorf("%w: %s has children: %s", errHasChildren, id, strings.Join(kids, ", "))
	}
	removeNodes := map[NodeID]bool{id: true}
	removeRels := map[RelID]bool{}
	for _, rel := range idx.Relationships {
		if rel.From == id || rel.To == id {
			closure := idx.relationshipClosure(rel.ID)
			for nid := range closure.nodes {
				removeNodes[nid] = true
			}
			for rid := range closure.rels {
				removeRels[rid] = true
			}
		}
	}
	var order []NodeID
	for _, existing := range idx.Order {
		if !removeNodes[existing] {
			order = append(order, existing)
		}
	}
	var rels []Relationship
	for _, rel := range idx.Relationships {
		if removeRels[rel.ID] {
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
	for nid := range removeNodes {
		if err := idx.removeNodeFiles(nid); err != nil {
			return err
		}
	}
	if err := idx.reload(); err != nil {
		return err
	}
	idx.invalidateLayoutLocked(parent)
	if relParent != "" {
		idx.invalidateLayoutKeyLocked(RelationshipLayoutKey(relParent))
	}
	for rid := range removeRels {
		idx.invalidateLayoutKeyLocked(RelationshipLayoutKey(rid))
	}
	return nil
}

// SetRelationship creates when ID is empty, or updates label and kind when ID is set.
func (idx *Index) SetRelationship(in SetRelationshipInput) (Relationship, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	rel, err := idx.setRelationshipLocked(in)
	if err != nil {
		return Relationship{}, err
	}
	if err := idx.reload(); err != nil {
		return Relationship{}, err
	}
	return rel, nil
}

// DeleteRelationship removes the edge and cascades interior members.
func (idx *Index) DeleteRelationship(id RelID) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if _, i := idx.relByID(id); i < 0 {
		return fmt.Errorf("%w: %q", ErrRelationshipNotFound, id)
	}
	toRemove := idx.relationshipClosure(id)
	var order []NodeID
	for _, existing := range idx.Order {
		if !toRemove.nodes[existing] {
			order = append(order, existing)
		}
	}
	var rels []Relationship
	for _, rel := range idx.Relationships {
		if toRemove.rels[rel.ID] {
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
	for nid := range toRemove.nodes {
		if err := idx.removeNodeFiles(nid); err != nil {
			return err
		}
	}
	if err := idx.reload(); err != nil {
		return err
	}
	for rid := range toRemove.rels {
		idx.invalidateLayoutKeyLocked(RelationshipLayoutKey(rid))
	}
	return nil
}

type relClosure struct {
	nodes map[NodeID]bool
	rels  map[RelID]bool
}

func (idx *Index) relationshipClosure(root RelID) relClosure {
	out := relClosure{nodes: map[NodeID]bool{}, rels: map[RelID]bool{}}
	var walk func(id RelID)
	walk = func(id RelID) {
		if out.rels[id] {
			return
		}
		out.rels[id] = true
		rel, _ := idx.relByID(id)
		if rel == nil {
			return
		}
		for _, mid := range idx.memberIDs(id) {
			out.nodes[mid] = true
			idx.collectDescendants(mid, out.nodes)
		}
		for _, edge := range idx.Relationships {
			if out.rels[edge.ID] {
				continue
			}
			touches := out.nodes[edge.From] || out.nodes[edge.To]
			if !touches {
				for _, mid := range idx.memberIDs(edge.ID) {
					if out.nodes[mid] {
						touches = true
						break
					}
				}
			}
			if touches {
				walk(edge.ID)
			}
		}
	}
	walk(root)
	return out
}

func (idx *Index) collectDescendants(id NodeID, set map[NodeID]bool) {
	for _, kid := range idx.Children[id] {
		set[kid] = true
		idx.collectDescendants(kid, set)
	}
}

// setRelationshipLocked updates index.json. Caller holds idx.mu and reloads.
func (idx *Index) setRelationshipLocked(in SetRelationshipInput) (Relationship, error) {
	from, to, label, kind := in.From, in.To, in.Label, in.Kind
	if _, ok := idx.Nodes[from]; !ok {
		return Relationship{}, fmt.Errorf("%w: %q", errMissingEndpoint, from)
	}
	if _, ok := idx.Nodes[to]; !ok {
		return Relationship{}, fmt.Errorf("%w: %q", errMissingEndpoint, to)
	}
	if kind != "" && !validKind(kind) {
		return Relationship{}, fmt.Errorf("%w: %s -> %s kind %q", errBadKind, from, to, kind)
	}
	prev := append([]Relationship{}, idx.Relationships...)
	if in.ID != "" {
		rel, i := idx.relByID(in.ID)
		if rel != nil {
			updated := *rel
			updated.Label = label
			updated.Kind = kind
			idx.Relationships[i] = updated
			if err := idx.saveIndex(); err != nil {
				idx.Relationships = prev
				return Relationship{}, err
			}
			return updated, nil
		}
		created := Relationship{ID: in.ID, From: from, To: to, Label: label, Kind: kind}
		idx.Relationships = append(idx.Relationships, created)
		if err := idx.saveIndex(); err != nil {
			idx.Relationships = prev
			return Relationship{}, err
		}
		return created, nil
	}
	id, err := idx.allocateRelationshipID(label)
	if err != nil {
		return Relationship{}, err
	}
	created := Relationship{ID: id, From: from, To: to, Label: label, Kind: kind}
	idx.Relationships = append(idx.Relationships, created)
	if err := idx.saveIndex(); err != nil {
		idx.Relationships = prev
		return Relationship{}, err
	}
	return created, nil
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
