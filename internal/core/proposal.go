package core

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	// OpCreateNode is a proposed node that does not exist yet.
	OpCreateNode = "create_node"
	// OpUpdateNode replaces the fields named on the operation.
	OpUpdateNode = "update_node"
	// OpDeleteNode removes a node that has no children.
	OpDeleteNode = "delete_node"
	// OpSetRelationship upserts one directed edge. node_id is the from node.
	OpSetRelationship = "set_relationship"
	// OpDeleteRelationship removes one edge by id (and, like DeleteRelationship, any
	// interior nodes the edge owns). node_id is the edge's from node.
	OpDeleteRelationship = "delete_relationship"

	// SourceSeedModel is the proposal source used when many operations arrive together.
	SourceSeedModel = "seed_model"
	// SourceGroupNodes, SourceUngroupNode and SourceReparentNode mark proposals staged
	// from the canvas (group selection, ungroup, drag out of a group).
	SourceGroupNodes   = "group_nodes"
	SourceUngroupNode  = "ungroup_node"
	SourceReparentNode = "reparent_node"

	opStatusPending  = "pending"
	opStatusAccepted = "accepted"
	opStatusRejected = "rejected"

	proposalManifestVersion = 1
)

// applyHook, when set, fails an accept after the named step has been written.
// Steps are "node-json", "node-md", and "index". Tests set it; production leaves it nil.
var applyHook func(step string) error

var proposalIDPattern = regexp.MustCompile(`^p-[0-9]{8}-[0-9]{4}-[0-9a-f]{4}$`)

// StageOp is one mutation inside a proposal. Create carries Create; its ID is optional
// and derived from the name when empty. Update and delete carry NodeID. A
// delete_relationship carries RelID only. A relationship carries From, To, Label, and Kind.
type StageOp struct {
	Op     string
	Create CreateInput
	NodeID NodeID
	Update UpdateInput
	RelID  RelID
	From   NodeID
	To     NodeID
	Label  string
	Kind   string
}

// StageInput is one authoring call: a source tool name, an optional summary, and
// the operations in apply order.
type StageInput struct {
	Source  string
	Summary string
	Ops     []StageOp
}

// ProposalSummary is one proposal as listed, with staleness computed at read time.
type ProposalSummary struct {
	ID         string      `json:"proposal_id"`
	CreatedAt  string      `json:"created_at,omitempty"`
	Source     string      `json:"source,omitempty"`
	Summary    string      `json:"summary,omitempty"`
	Unreadable bool        `json:"unreadable"`
	Error      string      `json:"error,omitempty"`
	Operations []OpSummary `json:"operations,omitempty"`
}

// OpSummary is one operation without the file diff.
type OpSummary struct {
	Index       int      `json:"index"`
	Op          string   `json:"op"`
	NodeID      string   `json:"node_id"`
	Status      string   `json:"status"`
	Stale       bool     `json:"stale"`
	StaleReason string   `json:"stale_reason,omitempty"`
	Fields      []string `json:"fields,omitempty"`
}

// ProposalDiff is the current and proposed state of each operation.
type ProposalDiff struct {
	ID         string   `json:"proposal_id"`
	CreatedAt  string   `json:"created_at"`
	Source     string   `json:"source"`
	Summary    string   `json:"summary,omitempty"`
	Operations []OpDiff `json:"operations"`
}

// OpDiff is one operation ready to render. Stale is computed here, not stored.
type OpDiff struct {
	Index               int           `json:"index"`
	Op                  string        `json:"op"`
	NodeID              string        `json:"node_id"`
	Status              string        `json:"status"`
	Stale               bool          `json:"stale"`
	StaleReason         string        `json:"stale_reason,omitempty"`
	Fields              []string      `json:"fields,omitempty"`
	Current             *NodeSnapshot `json:"current"`
	Proposed            *NodeSnapshot `json:"proposed"`
	Relationship        *RelSnapshot  `json:"relationship"`
	CurrentRelationship *RelSnapshot  `json:"current_relationship"`
	// RemovesNodes lists the interior nodes a delete_relationship also deletes.
	RemovesNodes []RemovedNode `json:"removes_nodes,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// RemovedNode names an interior node that a delete_relationship cascades to.
type RemovedNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// NodeSnapshot is the node body a diff row renders.
type NodeSnapshot struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Type           string         `json:"type"`
	Status         string         `json:"status"`
	ParentID       string         `json:"parent_id,omitempty"`
	RelationshipID string         `json:"relationship_id,omitempty"`
	Implementation []string       `json:"implementation"`
	Scope          []string       `json:"scope"`
	Protected      bool           `json:"protected"`
	Markdown       string         `json:"markdown"`
	Icon           string         `json:"icon,omitempty"`
	Color          string         `json:"color,omitempty"`
	Renderer       string         `json:"renderer,omitempty"`
	Schema         []SchemaColumn `json:"schema,omitempty"`
}

// RelSnapshot is one directed edge.
type RelSnapshot struct {
	ID    string `json:"id,omitempty"`
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}

// SkippedOp is an operation accept-all did not apply because it was stale.
type SkippedOp struct {
	Index  int    `json:"index"`
	Op     string `json:"op"`
	NodeID string `json:"node_id"`
	Reason string `json:"reason"`
}

// FailedOp is an operation accept-all left pending because validation failed.
type FailedOp struct {
	Index  int    `json:"index"`
	Op     string `json:"op"`
	NodeID string `json:"node_id"`
	Error  string `json:"error"`
}

// AcceptAllResult reports which operations were applied and which stale ones were skipped.
type AcceptAllResult struct {
	Applied []int       `json:"applied"`
	Skipped []SkippedOp `json:"skipped"`
	Failed  []FailedOp  `json:"failed"`
}

type manifest struct {
	ManifestVersion int         `json:"manifest_version"`
	ProposalID      string      `json:"proposal_id"`
	CreatedAt       string      `json:"created_at"`
	Source          string      `json:"source"`
	Summary         string      `json:"summary,omitempty"`
	Operations      []operation `json:"operations"`
}

type operation struct {
	Op       string   `json:"op"`
	NodeID   string   `json:"node_id"`
	BaseHash *string  `json:"base_hash"`
	Fields   []string `json:"fields,omitempty"`
	From     string   `json:"from,omitempty"`
	To       string   `json:"to,omitempty"`
	RelID    string   `json:"relationship_id,omitempty"`
	Label    *string  `json:"label,omitempty"`
	Kind     *string  `json:"kind,omitempty"`
	Status   string   `json:"status"`
}

type materialBody struct {
	json []byte
	md   []byte
}

// contentHash is sha256: plus the hex digest of the on-disk node pair.
// The bytes are length-prefixed so the boundary between JSON and markdown is fixed.
func contentHash(jsonBytes, mdBytes []byte) string {
	h := sha256.New()
	var lenb [8]byte
	binary.BigEndian.PutUint64(lenb[:], uint64(len(jsonBytes)))
	h.Write(lenb[:])
	h.Write(jsonBytes)
	binary.BigEndian.PutUint64(lenb[:], uint64(len(mdBytes)))
	h.Write(lenb[:])
	h.Write(mdBytes)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// Stage writes a proposal and returns its id. It does not modify .arch/nodes/.
func (idx *Index) Stage(in StageInput) (string, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.stageLocked(in)
}

// stageLocked is Stage for callers that already hold idx.mu.
func (idx *Index) stageLocked(in StageInput) (string, error) {
	if !validProposalSource(in.Source) {
		return "", fmt.Errorf("%w: source %q", ErrBadOperation, in.Source)
	}
	if len(in.Ops) == 0 {
		return "", fmt.Errorf("%w: proposal has no operations", ErrBadOperation)
	}

	nodes := map[NodeID]*Node{}
	for id, n := range idx.Nodes {
		nodes[id] = cloneNode(n)
	}
	material := map[NodeID]materialBody{}
	ops := make([]operation, 0, len(in.Ops))
	files := map[NodeID]materialBody{}
	stagedRels := []Relationship{}

	for _, inOp := range in.Ops {
		op, body, err := idx.stageOne(inOp, nodes, material, &stagedRels)
		if err != nil {
			return "", err
		}
		ops = append(ops, op)
		if body != nil {
			files[NodeID(op.NodeID)] = *body
			material[NodeID(op.NodeID)] = *body
		}
	}

	now := time.Now().UTC()
	id, err := idx.allocProposalID(now)
	if err != nil {
		return "", err
	}
	dir, err := idx.proposalPath(id)
	if err != nil {
		return "", err
	}
	m := manifest{
		ManifestVersion: proposalManifestVersion,
		ProposalID:      id,
		CreatedAt:       now.Format(time.RFC3339),
		Source:          in.Source,
		Summary:         in.Summary,
		Operations:      ops,
	}
	if err := os.MkdirAll(filepath.Join(dir, "nodes"), 0o755); err != nil {
		return "", err
	}
	for nodeID, body := range files {
		if err := writeAtomic(filepath.Join(dir, "nodes", string(nodeID)+".json"), body.json); err != nil {
			os.RemoveAll(dir)
			return "", err
		}
		if err := writeAtomic(filepath.Join(dir, "nodes", string(nodeID)+".md"), body.md); err != nil {
			os.RemoveAll(dir)
			return "", err
		}
	}
	if err := writeManifest(dir, m); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return id, nil
}

func (idx *Index) stageOne(in StageOp, nodes map[NodeID]*Node, material map[NodeID]materialBody, stagedRels *[]Relationship) (operation, *materialBody, error) {
	switch in.Op {
	case OpCreateNode:
		return idx.stageCreate(in, nodes, material)
	case OpUpdateNode:
		return idx.stageUpdate(in, nodes, material)
	case OpDeleteNode:
		return idx.stageDelete(in, nodes, material)
	case OpSetRelationship:
		return idx.stageRelationship(in, nodes, material, stagedRels)
	case OpDeleteRelationship:
		return idx.stageDeleteRelationship(in, material)
	default:
		return operation{}, nil, fmt.Errorf("%w: unknown operation %q", ErrBadOperation, in.Op)
	}
}

func (idx *Index) stageCreate(in StageOp, nodes map[NodeID]*Node, material map[NodeID]materialBody) (operation, *materialBody, error) {
	if strings.TrimSpace(in.Create.Name) == "" {
		return operation{}, nil, ErrEmptyName
	}
	if strings.TrimSpace(in.Create.Type) == "" {
		return operation{}, nil, ErrEmptyType
	}
	status := in.Create.Status
	if status == "" {
		status = StatusDraft
	}
	if !validStatus(status) {
		return operation{}, nil, fmt.Errorf("%w: %q (valid: %s)", ErrBadStatus, status, statusList())
	}
	var id NodeID
	if in.Create.ID != "" {
		if err := idx.checkExplicitID(in.Create.ID, nodes); err != nil {
			return operation{}, nil, err
		}
		id = in.Create.ID
	} else {
		base, err := slugify(in.Create.Name)
		if err != nil {
			return operation{}, nil, err
		}
		id = idx.allocateDuring(base, nodes)
	}
	if in.Create.ParentID != "" && in.Create.RelationshipID != "" {
		return operation{}, nil, ErrContainerConflict
	}
	if in.Create.ParentID != "" {
		if _, ok := nodes[in.Create.ParentID]; !ok {
			return operation{}, nil, fmt.Errorf("%w: %q", ErrMissingParent, in.Create.ParentID)
		}
		if parentCyclesIn(nodes, id, in.Create.ParentID) {
			return operation{}, nil, fmt.Errorf("%w: %s -> %s", ErrCycle, id, in.Create.ParentID)
		}
	}
	if in.Create.RelationshipID != "" {
		found := false
		for _, rel := range idx.Relationships {
			if rel.ID == in.Create.RelationshipID {
				found = true
				break
			}
		}
		if !found {
			return operation{}, nil, fmt.Errorf("%w: %q", ErrRelationshipNotFound, in.Create.RelationshipID)
		}
	}
	if _, ok := material[id]; ok {
		return operation{}, nil, fmt.Errorf("%w: %s already has materialised files in this proposal", ErrBadOperation, id)
	}
	node := &Node{
		ID:             id,
		Name:           in.Create.Name,
		Type:           in.Create.Type,
		Status:         status,
		ParentID:       in.Create.ParentID,
		RelationshipID: in.Create.RelationshipID,
		Implementation: cloneStrings(in.Create.Implementation),
		Scope:          cloneStrings(in.Create.Scope),
		Protected:      in.Create.Protected,
		Spec:           in.Create.Spec,
		Icon:           strings.TrimSpace(in.Create.Icon),
		Color:          strings.TrimSpace(in.Create.Color),
		Renderer:       strings.TrimSpace(in.Create.Renderer),
		Schema:         normalizeSchema(in.Create.Schema),
	}
	if err := validateCreatePresentation(node.Icon, node.Color, node.Renderer, node.Schema); err != nil {
		return operation{}, nil, err
	}
	body, err := materialise(node)
	if err != nil {
		return operation{}, nil, err
	}
	nodes[id] = node
	op := operation{Op: OpCreateNode, NodeID: string(id), Status: opStatusPending}
	return op, &body, nil
}

func (idx *Index) stageUpdate(in StageOp, nodes map[NodeID]*Node, material map[NodeID]materialBody) (operation, *materialBody, error) {
	cur, ok := nodes[in.NodeID]
	if !ok {
		return operation{}, nil, fmt.Errorf("%w: %q", ErrNotFound, in.NodeID)
	}
	if _, ok := material[in.NodeID]; ok {
		return operation{}, nil, fmt.Errorf("%w: %s already has materialised files in this proposal", ErrBadOperation, in.NodeID)
	}
	next := cloneNode(cur)
	fields, err := applyStageUpdate(next, in.Update, nodes, idx)
	if err != nil {
		return operation{}, nil, err
	}
	hash, err := idx.hashFor(in.NodeID, material)
	if err != nil {
		return operation{}, nil, err
	}
	body, err := materialise(next)
	if err != nil {
		return operation{}, nil, err
	}
	nodes[in.NodeID] = next
	op := operation{
		Op:       OpUpdateNode,
		NodeID:   string(in.NodeID),
		BaseHash: &hash,
		Fields:   fields,
		Status:   opStatusPending,
	}
	return op, &body, nil
}

func (idx *Index) stageDelete(in StageOp, nodes map[NodeID]*Node, material map[NodeID]materialBody) (operation, *materialBody, error) {
	if _, ok := nodes[in.NodeID]; !ok {
		return operation{}, nil, fmt.Errorf("%w: %q", ErrNotFound, in.NodeID)
	}
	names := childNames(nodes, in.NodeID)
	if len(names) > 0 {
		return operation{}, nil, fmt.Errorf("%w: delete_node %s has %d children: %s", ErrHasChildren, in.NodeID, len(names), strings.Join(names, ", "))
	}
	hash, err := idx.hashFor(in.NodeID, material)
	if err != nil {
		return operation{}, nil, err
	}
	delete(nodes, in.NodeID)
	op := operation{Op: OpDeleteNode, NodeID: string(in.NodeID), BaseHash: &hash, Status: opStatusPending}
	return op, nil, nil
}

func (idx *Index) stageRelationship(in StageOp, nodes map[NodeID]*Node, material map[NodeID]materialBody, stagedRels *[]Relationship) (operation, *materialBody, error) {
	if in.From == "" || in.To == "" {
		return operation{}, nil, fmt.Errorf("%w: set_relationship needs from and to", ErrBadOperation)
	}
	if _, ok := nodes[in.From]; !ok {
		return operation{}, nil, fmt.Errorf("%w: %q", ErrMissingEndpoint, in.From)
	}
	if _, ok := nodes[in.To]; !ok {
		return operation{}, nil, fmt.Errorf("%w: %q", ErrMissingEndpoint, in.To)
	}
	if in.Kind != "" && !validKind(in.Kind) {
		return operation{}, nil, fmt.Errorf("%w: %s -> %s kind %q", ErrBadKind, in.From, in.To, in.Kind)
	}
	if in.RelID != "" {
		found := false
		for _, rel := range idx.Relationships {
			if rel.ID == in.RelID {
				found = true
				break
			}
		}
		if !found {
			for _, rel := range *stagedRels {
				if rel.ID == in.RelID {
					found = true
					break
				}
			}
		}
		if !found {
			return operation{}, nil, fmt.Errorf("%w: %q", ErrRelationshipNotFound, in.RelID)
		}
	}
	hash, err := idx.hashFor(in.From, material)
	if err != nil {
		return operation{}, nil, err
	}
	label, kind := in.Label, in.Kind
	relID := in.RelID
	if relID == "" {
		id, err := idx.allocateRelationshipIDStaged(label, *stagedRels)
		if err != nil {
			return operation{}, nil, err
		}
		relID = id
		*stagedRels = append(*stagedRels, Relationship{ID: relID, From: in.From, To: in.To, Label: label, Kind: kind})
	}
	op := operation{
		Op:       OpSetRelationship,
		NodeID:   string(in.From),
		BaseHash: &hash,
		From:     string(in.From),
		To:       string(in.To),
		RelID:    string(relID),
		Label:    &label,
		Kind:     &kind,
		Status:   opStatusPending,
	}
	return op, nil, nil
}

func (idx *Index) stageDeleteRelationship(in StageOp, material map[NodeID]materialBody) (operation, *materialBody, error) {
	if in.RelID == "" {
		return operation{}, nil, fmt.Errorf("%w: delete_relationship needs relationship_id", ErrBadOperation)
	}
	rel, _ := idx.relByID(in.RelID)
	if rel == nil {
		return operation{}, nil, fmt.Errorf("%w: %q", ErrRelationshipNotFound, in.RelID)
	}
	hash, err := idx.deleteRelHash(rel, func(id NodeID) (string, error) { return idx.hashFor(id, material) })
	if err != nil {
		return operation{}, nil, err
	}
	label, kind := rel.Label, rel.Kind
	op := operation{
		Op:       OpDeleteRelationship,
		NodeID:   string(rel.From),
		BaseHash: &hash,
		From:     string(rel.From),
		To:       string(rel.To),
		RelID:    string(rel.ID),
		Label:    &label,
		Kind:     &kind,
		Status:   opStatusPending,
	}
	return op, nil, nil
}

// removedByRelationship lists the interior nodes (sorted) that deleting rel also removes.
func (idx *Index) removedByRelationship(rel *Relationship) []string {
	var out []string
	for nid := range idx.relationshipClosure(rel.ID).nodes {
		out = append(out, string(nid))
	}
	sort.Strings(out)
	return out
}

// deleteRelHash folds the source node and every interior node the delete would
// remove into one hash, so a change to any of them makes the operation stale.
func (idx *Index) deleteRelHash(rel *Relationship, hashOf func(NodeID) (string, error)) (string, error) {
	h, err := hashOf(rel.From)
	if err != nil {
		return "", err
	}
	interior := idx.removedByRelationship(rel)
	if len(interior) == 0 {
		return h, nil
	}
	sum := sha256.New()
	sum.Write([]byte(h))
	for _, nid := range interior {
		ih, err := hashOf(NodeID(nid))
		if err != nil {
			return "", err
		}
		sum.Write([]byte("\x00" + nid + "\x00" + ih))
	}
	return "sha256:" + hex.EncodeToString(sum.Sum(nil)), nil
}

func (idx *Index) hashFor(id NodeID, material map[NodeID]materialBody) (string, error) {
	if body, ok := material[id]; ok {
		return contentHash(body.json, body.md), nil
	}
	hash, ok, err := idx.hashOnDisk(id)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	return hash, nil
}

func (idx *Index) hashOnDisk(id NodeID) (string, bool, error) {
	jsonBytes, err := os.ReadFile(idx.nodeJSON(id))
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	mdBytes, err := os.ReadFile(idx.nodeMD(id))
	if err != nil {
		if os.IsNotExist(err) {
			mdBytes = []byte{}
		} else {
			return "", false, err
		}
	}
	return contentHash(jsonBytes, mdBytes), true, nil
}

// ListProposals returns every proposal directory, oldest id first.
// Staleness is computed from the node on disk and is not read from the manifest.
func (idx *Index) ListProposals() ([]ProposalSummary, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	ids, err := idx.proposalIDs()
	if err != nil {
		return nil, err
	}
	out := make([]ProposalSummary, 0, len(ids))
	for _, id := range ids {
		summary, _, _ := idx.inspect(id)
		out = append(out, summary)
	}
	return out, nil
}

// ProposalDiff returns the current and proposed state of one readable proposal.
func (idx *Index) ProposalDiff(id string) (*ProposalDiff, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	summary, m, err := idx.inspect(id)
	if err != nil {
		return nil, err
	}
	if summary.Unreadable {
		return nil, fmt.Errorf("%w: %s", ErrUnreadableProposal, summary.Error)
	}
	diff := &ProposalDiff{
		ID:         m.ProposalID,
		CreatedAt:  m.CreatedAt,
		Source:     m.Source,
		Summary:    m.Summary,
		Operations: make([]OpDiff, len(m.Operations)),
	}
	for i, op := range m.Operations {
		diff.Operations[i] = idx.diffOp(m.ProposalID, i, op, m.Operations)
	}
	return diff, nil
}

// AcceptOperation applies one pending operation. A stale hash mismatch requires
// confirmStale. A missing node is not applied. Validation failure leaves the
// operation pending.
func (idx *Index) AcceptOperation(id string, index int, confirmStale bool) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	m, err := idx.loadReadable(id)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(m.Operations) {
		return fmt.Errorf("%w: operation index %d", ErrBadOperation, index)
	}
	op := m.Operations[index]
	if op.Status != opStatusPending {
		return fmt.Errorf("%w: %s %s is %s", ErrBadOperation, op.Op, op.NodeID, op.Status)
	}
	if err := idx.rejectIfStale(m.Operations, op, confirmStale); err != nil {
		return err
	}
	if reason := idx.createDependencyUnmet(id, m.Operations, op); reason != "" {
		return fmt.Errorf("%w: %s %s: %s", ErrStale, op.Op, op.NodeID, reason)
	}
	if err := idx.applyOp(id, op); err != nil {
		return err
	}
	return idx.finishOp(&m, index, opStatusAccepted)
}

// RejectOperation marks one operation rejected and leaves the directory in place
// until every operation is resolved.
func (idx *Index) RejectOperation(id string, index int) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	m, err := idx.loadReadable(id)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(m.Operations) {
		return fmt.Errorf("%w: operation index %d", ErrBadOperation, index)
	}
	op := m.Operations[index]
	if op.Status != opStatusPending {
		return fmt.Errorf("%w: %s %s is %s", ErrBadOperation, op.Op, op.NodeID, op.Status)
	}
	return idx.finishOp(&m, index, opStatusRejected)
}

// AcceptAll applies pending operations that are not stale, in order, and reports
// the ones it skipped. Stale operations (and operations that depend on a create that
// was not accepted) are skipped and the rest still apply. Application is
// all-or-nothing for failures: if any operation fails validation, every operation
// this call already applied is rolled back (node files, index.json and the manifest
// are restored), nothing is reported as applied, and the failing operation is
// reported in Failed. Operations after the failing one are not attempted. The cached
// layout files are restored too, so user-arranged positions survive a rollback.
func (idx *Index) AcceptAll(id string) (AcceptAllResult, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	result := AcceptAllResult{Applied: []int{}, Skipped: []SkippedOp{}, Failed: []FailedOp{}}
	m, err := idx.loadReadable(id)
	if err != nil {
		return result, err
	}
	snap, err := idx.snapshotModel()
	if err != nil {
		return result, err
	}
	original := append([]operation(nil), m.Operations...)
	for i := range m.Operations {
		op := m.Operations[i]
		if op.Status != opStatusPending {
			continue
		}
		stale, reason := idx.staleness(m.Operations, op)
		if stale {
			result.Skipped = append(result.Skipped, SkippedOp{
				Index: i, Op: op.Op, NodeID: op.NodeID, Reason: reason,
			})
			continue
		}
		if reason := idx.createDependencyUnmet(id, m.Operations, op); reason != "" {
			result.Skipped = append(result.Skipped, SkippedOp{
				Index: i, Op: op.Op, NodeID: op.NodeID, Reason: reason,
			})
			continue
		}
		opErr := idx.applyOp(id, op)
		if opErr == nil {
			opErr = idx.finishOp(&m, i, opStatusAccepted)
		}
		if opErr != nil {
			// A failed op may have written partway, so always restore the snapshot.
			if rerr := idx.rollbackAccept(snap, &m, original); rerr != nil {
				return result, fmt.Errorf("rollback after failed %s %s: %w", op.Op, op.NodeID, rerr)
			}
			result.Applied = []int{}
			result.Failed = append(result.Failed, FailedOp{
				Index: i, Op: op.Op, NodeID: op.NodeID, Error: opErr.Error(),
			})
			return result, nil
		}
		result.Applied = append(result.Applied, i)
		if _, err := os.Stat(filepath.Join(idx.mustProposalPath(id), "manifest.json")); err != nil {
			break
		}
	}
	return result, nil
}

// modelSnapshot is the model on disk: index.json, every node file and the cached layouts.
type modelSnapshot struct {
	index   []byte
	nodes   map[string][]byte
	layouts map[string][]byte
}

func readDirFiles(dir string) (map[string][]byte, error) {
	out := map[string][]byte{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		out[e.Name()] = data
	}
	return out, nil
}

// restoreDirFiles makes the files directly in dir equal want.
func restoreDirFiles(dir string, want map[string][]byte) error {
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, e := range entries {
		if _, ok := want[e.Name()]; !ok && !e.IsDir() {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	for name, data := range want {
		path := filepath.Join(dir, name)
		if cur, err := os.ReadFile(path); err == nil && bytes.Equal(cur, data) {
			continue
		}
		if err := writeAtomic(path, data); err != nil {
			return err
		}
	}
	return nil
}

func (idx *Index) layoutDir() string {
	return filepath.Join(idx.arch(), ".cache", "layout")
}

func (idx *Index) snapshotModel() (modelSnapshot, error) {
	index, err := os.ReadFile(filepath.Join(idx.arch(), "index.json"))
	if err != nil {
		return modelSnapshot{}, err
	}
	nodes, err := readDirFiles(filepath.Join(idx.arch(), "nodes"))
	if err != nil {
		return modelSnapshot{}, err
	}
	layouts, err := readDirFiles(idx.layoutDir())
	if err != nil {
		return modelSnapshot{}, err
	}
	return modelSnapshot{index: index, nodes: nodes, layouts: layouts}, nil
}

// rollbackAccept restores the model and the proposal manifest to their state before
// an AcceptAll call started.
func (idx *Index) rollbackAccept(snap modelSnapshot, m *manifest, original []operation) error {
	if err := restoreDirFiles(filepath.Join(idx.arch(), "nodes"), snap.nodes); err != nil {
		return err
	}
	if err := restoreDirFiles(idx.layoutDir(), snap.layouts); err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(idx.arch(), "index.json"), snap.index); err != nil {
		return err
	}
	m.Operations = original
	if err := writeManifest(idx.mustProposalPath(m.ProposalID), *m); err != nil {
		return err
	}
	return idx.reload()
}

// RejectAll marks every still-pending operation rejected, including stale ones.
func (idx *Index) RejectAll(id string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	m, err := idx.loadReadable(id)
	if err != nil {
		return err
	}
	for i := range m.Operations {
		if m.Operations[i].Status != opStatusPending {
			continue
		}
		if err := idx.finishOp(&m, i, opStatusRejected); err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(idx.mustProposalPath(id), "manifest.json")); err != nil {
			return nil
		}
	}
	return nil
}

// DeleteProposal removes a proposal directory, including one that is unreadable.
func (idx *Index) DeleteProposal(id string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	dir, err := idx.proposalPath(id)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrProposalNotFound, id)
		}
		return err
	}
	return os.RemoveAll(dir)
}

func (idx *Index) rejectIfStale(ops []operation, op operation, confirm bool) error {
	stale, reason := idx.staleness(ops, op)
	if !stale {
		return nil
	}
	if reason == staleMissing {
		return fmt.Errorf("%w: %s %s is stale because the node is missing and cannot be applied", ErrStale, op.Op, op.NodeID)
	}
	if !confirm {
		return fmt.Errorf("%w: %s %s is stale; confirm_stale is required to apply over the architect's newer edit", ErrStale, op.Op, op.NodeID)
	}
	return nil
}

const (
	staleMissing  = "node is missing"
	staleMismatch = "base_hash mismatch"

	staleCreateNotAccepted   = "container create_node was not accepted"
	staleEndpointNotAccepted = "edge endpoint create_node was not accepted"
)

// createDependencyUnmet returns a reason when op refers to a node that a create op in the
// same proposal proposes and that create has not been accepted: a create or update whose
// parent_id names it, or an edge with it as an endpoint. Applying such an op early would
// fail (or reparent under a same-id node from elsewhere). Empty means no unmet dependency.
func (idx *Index) createDependencyUnmet(proposalID string, ops []operation, op operation) string {
	notAccepted := func(id string) bool {
		for _, other := range ops {
			if other.Op == OpCreateNode && other.NodeID == id && other.Status != opStatusAccepted {
				return true
			}
		}
		return false
	}
	switch op.Op {
	case OpSetRelationship:
		if notAccepted(op.From) || notAccepted(op.To) {
			return staleEndpointNotAccepted
		}
	case OpCreateNode, OpUpdateNode:
		jsonBytes, _, err := readPair(filepath.Join(idx.mustProposalPath(proposalID), "nodes", op.NodeID))
		if err != nil {
			return ""
		}
		node, err := decodeNode(jsonBytes)
		if err != nil || node.ParentID == "" {
			return ""
		}
		if notAccepted(string(node.ParentID)) {
			return staleCreateNotAccepted
		}
	}
	return ""
}

// pendingCreateIDs lists node ids reserved by pending create ops in staged proposals.
func (idx *Index) pendingCreateIDs() map[string]bool {
	out := map[string]bool{}
	ids, err := idx.proposalIDs()
	if err != nil {
		return out
	}
	for _, pid := range ids {
		summary, m, err := idx.inspect(pid)
		if err != nil || summary.Unreadable {
			continue
		}
		for _, op := range m.Operations {
			if op.Op == OpCreateNode && op.Status == opStatusPending {
				out[op.NodeID] = true
			}
		}
	}
	return out
}

func (idx *Index) staleness(ops []operation, op operation) (bool, string) {
	if op.Op == OpCreateNode || op.Status != opStatusPending {
		return false, ""
	}
	if op.Op == OpDeleteRelationship {
		rel, _ := idx.relByID(RelID(op.RelID))
		if rel == nil {
			return true, staleMissing
		}
		if rel.Label != deref(op.Label) || rel.Kind != deref(op.Kind) {
			return true, staleMismatch
		}
		hash, err := idx.deleteRelHash(rel, func(id NodeID) (string, error) {
			h, ok, err := idx.hashOnDisk(id)
			if err == nil && !ok {
				err = ErrNotFound
			}
			return h, err
		})
		if err != nil {
			return true, staleMissing
		}
		if op.BaseHash == nil || *op.BaseHash != hash {
			return true, staleMismatch
		}
		return false, ""
	}
	hash, ok, err := idx.hashOnDisk(NodeID(op.NodeID))
	if err != nil || !ok {
		if op.Op == OpSetRelationship && pendingCreate(ops, op.NodeID) {
			return false, ""
		}
		if op.Op == OpUpdateNode || op.Op == OpDeleteNode || op.Op == OpSetRelationship || op.Op == OpDeleteRelationship {
			return true, staleMissing
		}
		return false, ""
	}
	if op.BaseHash == nil || *op.BaseHash != hash {
		return true, staleMismatch
	}
	return false, ""
}

func (idx *Index) applyOp(proposalID string, op operation) error {
	switch op.Op {
	case OpCreateNode, OpUpdateNode:
		return idx.applyNodeFiles(proposalID, op)
	case OpDeleteNode:
		return idx.applyDelete(op)
	case OpSetRelationship:
		in := SetRelationshipInput{
			ID:    RelID(op.RelID),
			From:  NodeID(op.From),
			To:    NodeID(op.To),
			Label: deref(op.Label),
			Kind:  deref(op.Kind),
		}
		if _, err := idx.setRelationshipLocked(in); err != nil {
			return err
		}
		return idx.reload()
	case OpDeleteRelationship:
		return idx.deleteRelationshipLocked(RelID(op.RelID))
	default:
		return fmt.Errorf("%w: unknown operation %q", ErrBadOperation, op.Op)
	}
}

func (idx *Index) applyNodeFiles(proposalID string, op operation) error {
	if !slugPattern.MatchString(op.NodeID) {
		return fmt.Errorf("%w: node id %q", ErrBadOperation, op.NodeID)
	}
	dir := idx.mustProposalPath(proposalID)
	jsonBytes, mdBytes, err := readPair(filepath.Join(dir, "nodes", op.NodeID))
	if err != nil {
		return err
	}
	node, err := decodeNode(jsonBytes)
	if err != nil {
		return fmt.Errorf("%w: %s %s: %v", ErrUnreadableProposal, op.Op, op.NodeID, err)
	}
	if string(node.ID) != op.NodeID {
		return fmt.Errorf("%w: %s %s id does not match its file", ErrUnreadableProposal, op.Op, op.NodeID)
	}
	node.Spec = string(mdBytes)
	creating := op.Op == OpCreateNode
	oldJSON, jsonExisted, err := readOptional(idx.nodeJSON(node.ID))
	if err != nil {
		return err
	}
	oldMD, mdExisted, err := readOptional(idx.nodeMD(node.ID))
	if err != nil {
		return err
	}
	oldIndex, err := os.ReadFile(filepath.Join(idx.arch(), "index.json"))
	if err != nil {
		return err
	}
	if creating && (jsonExisted || idx.Nodes[node.ID] != nil) {
		return fmt.Errorf("%w: create_node %s", ErrExists, node.ID)
	}
	if !creating && idx.Nodes[node.ID] == nil {
		return fmt.Errorf("%w: %q", ErrNotFound, node.ID)
	}
	prevOrder := append([]NodeID{}, idx.Order...)
	restore := func() {
		idx.Order = prevOrder
		if creating {
			delete(idx.Nodes, node.ID)
		}
		_ = restoreFile(idx.nodeJSON(node.ID), oldJSON, jsonExisted)
		_ = restoreFile(idx.nodeMD(node.ID), oldMD, mdExisted)
		_ = writeAtomic(filepath.Join(idx.arch(), "index.json"), oldIndex)
	}
	if err := writeAtomic(idx.nodeJSON(node.ID), jsonBytes); err != nil {
		restore()
		return err
	}
	if err := failApply("node-json"); err != nil {
		restore()
		return err
	}
	if err := writeAtomic(idx.nodeMD(node.ID), mdBytes); err != nil {
		restore()
		return err
	}
	if err := failApply("node-md"); err != nil {
		restore()
		return err
	}
	if err := idx.validateMaterialised(node, creating); err != nil {
		restore()
		return err
	}
	var oldParent NodeID
	var oldRel RelID
	if !creating {
		if cur := idx.Nodes[node.ID]; cur != nil {
			oldParent = cur.ParentID
			oldRel = cur.RelationshipID
		}
	}
	if creating {
		idx.Nodes[node.ID] = node
		idx.Order = append(idx.Order, node.ID)
		if err := idx.saveIndex(); err != nil {
			restore()
			return err
		}
		if err := failApply("index"); err != nil {
			restore()
			return err
		}
	}
	if err := idx.reload(); err != nil {
		return err
	}
	if creating {
		if node.RelationshipID != "" {
			idx.invalidateLayoutKeyLocked(RelationshipLayoutKey(node.RelationshipID))
		} else {
			idx.invalidateLayoutLocked(node.ParentID)
		}
	} else {
		if node.ParentID != oldParent {
			idx.invalidateLayoutLocked(oldParent)
			idx.invalidateLayoutLocked(node.ParentID)
			idx.invalidateAncestorVariantsLocked(oldParent)
			idx.invalidateAncestorVariantsLocked(node.ParentID)
		}
		if node.RelationshipID != oldRel {
			if oldRel != "" {
				idx.invalidateLayoutKeyLocked(RelationshipLayoutKey(oldRel))
			}
			if node.RelationshipID != "" {
				idx.invalidateLayoutKeyLocked(RelationshipLayoutKey(node.RelationshipID))
			}
		}
	}
	return nil
}

func (idx *Index) applyDelete(op operation) error {
	id := NodeID(op.NodeID)
	node, ok := idx.Nodes[id]
	if !ok {
		return fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	parent := node.ParentID
	relParent := node.RelationshipID
	names := childNames(idx.Nodes, id)
	if len(names) > 0 {
		return fmt.Errorf("%w: delete_node %s has %d children: %s", ErrHasChildren, id, len(names), strings.Join(names, ", "))
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
	oldIndex, err := os.ReadFile(filepath.Join(idx.arch(), "index.json"))
	if err != nil {
		return err
	}
	oldFiles := map[NodeID]struct {
		json []byte
		md   []byte
		jOk  bool
		mOk  bool
	}{}
	for nid := range removeNodes {
		j, jOk, _ := readOptional(idx.nodeJSON(nid))
		m, mOk, _ := readOptional(idx.nodeMD(nid))
		oldFiles[nid] = struct {
			json []byte
			md   []byte
			jOk  bool
			mOk  bool
		}{j, m, jOk, mOk}
	}
	prevOrder := append([]NodeID{}, idx.Order...)
	prevRels := append([]Relationship{}, idx.Relationships...)
	restore := func() {
		idx.Order = prevOrder
		idx.Relationships = prevRels
		_ = writeAtomic(filepath.Join(idx.arch(), "index.json"), oldIndex)
		for nid, snap := range oldFiles {
			_ = restoreFile(idx.nodeJSON(nid), snap.json, snap.jOk)
			_ = restoreFile(idx.nodeMD(nid), snap.md, snap.mOk)
		}
	}
	order := make([]NodeID, 0, len(idx.Order))
	for _, existing := range idx.Order {
		if !removeNodes[existing] {
			order = append(order, existing)
		}
	}
	rels := make([]Relationship, 0, len(idx.Relationships))
	for _, rel := range idx.Relationships {
		if removeRels[rel.ID] {
			continue
		}
		rels = append(rels, rel)
	}
	idx.Order = order
	idx.Relationships = rels
	if err := idx.saveIndex(); err != nil {
		restore()
		return err
	}
	if err := failApply("index"); err != nil {
		restore()
		return err
	}
	for nid := range removeNodes {
		if err := idx.removeNodeFiles(nid); err != nil {
			restore()
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

func (idx *Index) validateMaterialised(n *Node, creating bool) error {
	if strings.TrimSpace(n.Name) == "" {
		return ErrEmptyName
	}
	if strings.TrimSpace(n.Type) == "" {
		return ErrEmptyType
	}
	if !validStatus(n.Status) {
		return fmt.Errorf("%w: %q (valid: %s)", ErrBadStatus, n.Status, statusList())
	}
	if creating {
		if _, ok := idx.Nodes[n.ID]; ok {
			return fmt.Errorf("%w: create_node %s", ErrExists, n.ID)
		}
	} else if _, ok := idx.Nodes[n.ID]; !ok {
		return fmt.Errorf("%w: %q", ErrNotFound, n.ID)
	}
	if n.ParentID != "" && n.RelationshipID != "" {
		return ErrContainerConflict
	}
	if n.ParentID != "" {
		if _, ok := idx.Nodes[n.ParentID]; !ok && n.ParentID != n.ID {
			return fmt.Errorf("%w: %q", ErrMissingParent, n.ParentID)
		}
		if parentCyclesIn(idx.Nodes, n.ID, n.ParentID) {
			return fmt.Errorf("%w: %s -> %s", ErrCycle, n.ID, n.ParentID)
		}
	}
	if n.RelationshipID != "" {
		if _, i := idx.relByID(n.RelationshipID); i < 0 {
			return fmt.Errorf("%w: %q", ErrRelationshipNotFound, n.RelationshipID)
		}
	}
	return nil
}

func (idx *Index) finishOp(m *manifest, index int, status string) error {
	m.Operations[index].Status = status
	dir := idx.mustProposalPath(m.ProposalID)
	if allResolved(m.Operations) {
		return os.RemoveAll(dir)
	}
	return writeManifest(dir, *m)
}

func (idx *Index) inspect(id string) (ProposalSummary, manifest, error) {
	dir, err := idx.proposalPath(id)
	if err != nil {
		return ProposalSummary{}, manifest{}, err
	}
	if _, statErr := os.Stat(dir); statErr != nil {
		if os.IsNotExist(statErr) {
			return ProposalSummary{}, manifest{}, fmt.Errorf("%w: %s", ErrProposalNotFound, id)
		}
		return ProposalSummary{}, manifest{}, statErr
	}
	m, err := readManifest(dir)
	if err != nil {
		return ProposalSummary{ID: id, Unreadable: true, Error: err.Error()}, manifest{}, nil
	}
	if m.ManifestVersion != proposalManifestVersion {
		msg := fmt.Sprintf("manifest_version %d is not supported", m.ManifestVersion)
		return ProposalSummary{ID: id, Unreadable: true, Error: msg}, manifest{}, nil
	}
	if m.ProposalID != id {
		msg := fmt.Sprintf("proposal_id %q does not match the directory", m.ProposalID)
		return ProposalSummary{ID: id, Unreadable: true, Error: msg}, manifest{}, nil
	}
	summary := ProposalSummary{
		ID:         m.ProposalID,
		CreatedAt:  m.CreatedAt,
		Source:     m.Source,
		Summary:    m.Summary,
		Operations: make([]OpSummary, len(m.Operations)),
	}
	for i, op := range m.Operations {
		stale, reason := idx.staleness(m.Operations, op)
		summary.Operations[i] = OpSummary{
			Index:       i,
			Op:          op.Op,
			NodeID:      op.NodeID,
			Status:      op.Status,
			Stale:       stale,
			StaleReason: reason,
			Fields:      op.Fields,
		}
	}
	return summary, m, nil
}

func (idx *Index) loadReadable(id string) (manifest, error) {
	summary, m, err := idx.inspect(id)
	if err != nil {
		return manifest{}, err
	}
	if summary.Unreadable {
		return manifest{}, fmt.Errorf("%w: %s: %s", ErrUnreadableProposal, id, summary.Error)
	}
	return m, nil
}

func (idx *Index) diffOp(proposalID string, index int, op operation, ops []operation) OpDiff {
	stale, reason := idx.staleness(ops, op)
	row := OpDiff{
		Index:       index,
		Op:          op.Op,
		NodeID:      op.NodeID,
		Status:      op.Status,
		Stale:       stale,
		StaleReason: reason,
		Fields:      op.Fields,
	}
	if cur := idx.Nodes[NodeID(op.NodeID)]; cur != nil && op.Op != OpCreateNode {
		row.Current = snapshotNode(cur)
	}
	switch op.Op {
	case OpCreateNode, OpUpdateNode:
		proposed, err := idx.readProposed(proposalID, op.NodeID)
		if err != nil {
			row.Error = err.Error()
			row.Proposed = nil
		} else {
			row.Proposed = proposed
		}
	case OpDeleteRelationship:
		label, kind := deref(op.Label), deref(op.Kind)
		row.Relationship = &RelSnapshot{ID: op.RelID, From: op.From, To: op.To, Label: label, Kind: kind}
		if rel, _ := idx.relByID(RelID(op.RelID)); rel != nil {
			row.CurrentRelationship = &RelSnapshot{ID: string(rel.ID), From: string(rel.From), To: string(rel.To), Label: rel.Label, Kind: rel.Kind}
			for _, nid := range idx.removedByRelationship(rel) {
				removed := RemovedNode{ID: nid}
				if n := idx.Nodes[NodeID(nid)]; n != nil {
					removed.Name = n.Name
				}
				row.RemovesNodes = append(row.RemovesNodes, removed)
			}
		}
	case OpSetRelationship:
		label, kind := deref(op.Label), deref(op.Kind)
		row.Relationship = &RelSnapshot{ID: op.RelID, From: op.From, To: op.To, Label: label, Kind: kind}
		if op.RelID != "" {
			if rel, _ := idx.relByID(RelID(op.RelID)); rel != nil {
				row.CurrentRelationship = &RelSnapshot{ID: string(rel.ID), From: string(rel.From), To: string(rel.To), Label: rel.Label, Kind: rel.Kind}
			}
		} else {
			for _, rel := range idx.Relationships {
				if string(rel.From) == op.From && string(rel.To) == op.To {
					row.CurrentRelationship = &RelSnapshot{ID: string(rel.ID), From: string(rel.From), To: string(rel.To), Label: rel.Label, Kind: rel.Kind}
					break
				}
			}
		}
	}
	return row
}

func (idx *Index) readProposed(proposalID, nodeID string) (*NodeSnapshot, error) {
	if !slugPattern.MatchString(nodeID) {
		return nil, fmt.Errorf("%w: node id %q", ErrBadOperation, nodeID)
	}
	jsonBytes, mdBytes, err := readPair(filepath.Join(idx.mustProposalPath(proposalID), "nodes", nodeID))
	if err != nil {
		return nil, err
	}
	node, err := decodeNode(jsonBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrUnreadableProposal, nodeID, err)
	}
	node.Spec = string(mdBytes)
	if string(node.ID) != nodeID {
		return nil, fmt.Errorf("%w: %s id does not match its file", ErrUnreadableProposal, nodeID)
	}
	return snapshotNode(node), nil
}

func (idx *Index) proposalIDs() ([]string, error) {
	entries, err := os.ReadDir(idx.proposalsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, entry := range entries {
		if entry.IsDir() && proposalIDPattern.MatchString(entry.Name()) {
			ids = append(ids, entry.Name())
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func (idx *Index) allocProposalID(now time.Time) (string, error) {
	for range 8 {
		id, err := newProposalID(now)
		if err != nil {
			return "", err
		}
		dir := filepath.Join(idx.proposalsDir(), id)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return id, nil
		}
	}
	return "", fmt.Errorf("could not allocate a proposal id")
}

func (idx *Index) proposalsDir() string {
	return filepath.Join(idx.arch(), ".proposals")
}

func (idx *Index) proposalPath(id string) (string, error) {
	if !proposalIDPattern.MatchString(id) {
		return "", fmt.Errorf("%w: %q", ErrProposalNotFound, id)
	}
	return filepath.Join(idx.proposalsDir(), id), nil
}

func (idx *Index) mustProposalPath(id string) string {
	dir, err := idx.proposalPath(id)
	if err != nil {
		return filepath.Join(idx.proposalsDir(), id)
	}
	return dir
}

func (idx *Index) allocateDuring(base string, extra map[NodeID]*Node) NodeID {
	reserved := idx.pendingCreateIDs()
	if !idx.idTaken(base, extra) && !reserved[base] {
		return NodeID(base)
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if !idx.idTaken(candidate, extra) && !reserved[candidate] {
			return NodeID(candidate)
		}
	}
}

const maxExplicitIDLen = 64

// checkExplicitID validates a caller-supplied node id: it must be a slug and must not
// collide with an existing node, a node earlier in the same proposal (extra), or an id
// reserved by a pending create in any staged proposal.
func (idx *Index) checkExplicitID(id NodeID, extra map[NodeID]*Node) error {
	if len(id) > maxExplicitIDLen {
		return fmt.Errorf("%w: id must be at most %d characters", ErrBadOperation, maxExplicitIDLen)
	}
	if !slugPattern.MatchString(string(id)) {
		return fmt.Errorf("%w: id %q must be lowercase letters and digits separated by single hyphens", ErrBadOperation, id)
	}
	if idx.idTaken(string(id), extra) || idx.pendingCreateIDs()[string(id)] {
		return fmt.Errorf("%w: %q", ErrExists, id)
	}
	return nil
}

func (idx *Index) idTaken(id string, extra map[NodeID]*Node) bool {
	if _, ok := extra[NodeID(id)]; ok {
		return true
	}
	return idx.taken(id)
}

func failApply(step string) error {
	if applyHook == nil {
		return nil
	}
	return applyHook(step)
}

func newProposalID(now time.Time) (string, error) {
	var buf [2]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("p-%s-%x", now.UTC().Format("20060102-1504"), buf[:]), nil
}

func validProposalSource(source string) bool {
	switch source {
	case SourceSeedModel, SourceGroupNodes, SourceUngroupNode, SourceReparentNode, OpCreateNode, OpUpdateNode, OpDeleteNode, OpSetRelationship, OpDeleteRelationship:
		return true
	default:
		return false
	}
}

func cloneNode(n *Node) *Node {
	if n == nil {
		return nil
	}
	cp := *n
	cp.Implementation = cloneStrings(n.Implementation)
	cp.Scope = cloneStrings(n.Scope)
	cp.Unknown = cloneRaw(n.Unknown)
	return &cp
}

func materialise(n *Node) (materialBody, error) {
	raw, err := encodeNode(n)
	if err != nil {
		return materialBody{}, err
	}
	return materialBody{json: raw, md: []byte(n.Spec)}, nil
}

func applyStageUpdate(n *Node, in UpdateInput, nodes map[NodeID]*Node, idx *Index) ([]string, error) {
	var fields []string
	if in.Name != nil {
		if strings.TrimSpace(*in.Name) == "" {
			return nil, ErrEmptyName
		}
		n.Name = *in.Name
		fields = append(fields, "name")
	}
	if in.Type != nil {
		if strings.TrimSpace(*in.Type) == "" {
			return nil, ErrEmptyType
		}
		n.Type = *in.Type
		fields = append(fields, "type")
	}
	if in.Spec != nil {
		n.Spec = *in.Spec
		fields = append(fields, "spec")
	}
	if in.Status != nil {
		return nil, fmt.Errorf("%w: status must be set with update_node_status, not update_node", ErrBadOperation)
	}
	if in.Parent != nil {
		parent := *in.Parent
		if parent != "" {
			if _, ok := nodes[parent]; !ok {
				return nil, fmt.Errorf("%w: %q", ErrMissingParent, parent)
			}
			if parentCyclesIn(nodes, n.ID, parent) {
				return nil, fmt.Errorf("%w: %s -> %s", ErrCycle, n.ID, parent)
			}
		}
		n.ParentID = parent
		if parent != "" {
			n.RelationshipID = ""
		}
		fields = append(fields, "parent_id")
	}
	if in.Relationship != nil {
		rid := *in.Relationship
		if rid != "" {
			found := false
			for _, rel := range idx.Relationships {
				if rel.ID == rid {
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("%w: %q", ErrRelationshipNotFound, rid)
			}
			if n.ParentID != "" {
				return nil, ErrContainerConflict
			}
		}
		n.RelationshipID = rid
		if rid != "" {
			n.ParentID = ""
		}
		fields = append(fields, "relationship_id")
	}
	if in.Implementation != nil {
		n.Implementation = cloneStrings(*in.Implementation)
		fields = append(fields, "implementation")
	}
	if in.Scope != nil {
		n.Scope = cloneStrings(*in.Scope)
		fields = append(fields, "scope")
	}
	if in.Protected != nil {
		n.Protected = *in.Protected
		fields = append(fields, "protected")
	}
	if in.Icon != nil {
		if err := applyIconPatch(n, in.Icon); err != nil {
			return nil, err
		}
		fields = append(fields, "icon")
	}
	if in.Color != nil {
		if err := applyColorPatch(n, in.Color); err != nil {
			return nil, err
		}
		fields = append(fields, "color")
	}
	if in.Renderer != nil {
		if err := applyRendererPatch(n, in.Renderer); err != nil {
			return nil, err
		}
		fields = append(fields, "renderer")
	}
	if in.Schema != nil {
		if err := applySchemaPatch(n, in.Schema); err != nil {
			return nil, err
		}
		fields = append(fields, "schema")
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("%w: update_node %s changes no fields", ErrBadOperation, n.ID)
	}
	return fields, nil
}

func parentCyclesIn(nodes map[NodeID]*Node, id, parent NodeID) bool {
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
		n, ok := nodes[cur]
		if !ok {
			return false
		}
		cur = n.ParentID
	}
	return false
}

func childNames(nodes map[NodeID]*Node, id NodeID) []string {
	var names []string
	for _, n := range nodes {
		if n != nil && n.ParentID == id {
			names = append(names, n.Name)
		}
	}
	sort.Strings(names)
	return names
}

func pendingCreate(ops []operation, id string) bool {
	for _, op := range ops {
		if op.Op == OpCreateNode && op.NodeID == id && op.Status == opStatusPending {
			return true
		}
	}
	return false
}

func allResolved(ops []operation) bool {
	if len(ops) == 0 {
		return false
	}
	for _, op := range ops {
		if op.Status != opStatusAccepted && op.Status != opStatusRejected {
			return false
		}
	}
	return true
}

func writeManifest(dir string, m manifest) error {
	data, err := json.MarshalIndent(m, "", "    ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeAtomic(filepath.Join(dir, "manifest.json"), data)
}

func readManifest(dir string) (manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return manifest{}, err
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return manifest{}, err
	}
	return m, nil
}

func readPair(prefix string) ([]byte, []byte, error) {
	jsonBytes, err := os.ReadFile(prefix + ".json")
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %s", ErrUnreadableProposal, err.Error())
	}
	mdBytes, err := os.ReadFile(prefix + ".md")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("%w: %s is missing its .md and cannot be applied as empty prose", ErrUnreadableProposal, filepath.Base(prefix))
		}
		return nil, nil, err
	}
	return jsonBytes, mdBytes, nil
}

func readOptional(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}

func restoreFile(path string, data []byte, existed bool) error {
	if !existed {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return writeAtomic(path, data)
}

func snapshotNode(n *Node) *NodeSnapshot {
	impl := n.Implementation
	if impl == nil {
		impl = []string{}
	}
	scope := n.Scope
	if scope == nil {
		scope = []string{}
	}
	return &NodeSnapshot{
		ID:             string(n.ID),
		Name:           n.Name,
		Type:           n.Type,
		Status:         string(n.Status),
		ParentID:       string(n.ParentID),
		RelationshipID: string(n.RelationshipID),
		Implementation: impl,
		Scope:          scope,
		Protected:      n.Protected,
		Markdown:       n.Spec,
		Icon:           n.Icon,
		Color:          n.Color,
		Renderer:       n.Renderer,
		Schema:         append([]SchemaColumn(nil), n.Schema...),
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
