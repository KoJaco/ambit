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
	// OpRenameNode changes a node id and rewrites every reference to it. node_id is the
	// current id; new_id is the replacement.
	OpRenameNode = "rename_node"
	// OpSetConstraints replaces the project constraints in config.json (full list).
	// It carries no node id; its base_hash is the hash of the config.json bytes.
	OpSetConstraints = "set_constraints"

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
	// NewID is the replacement id for rename_node.
	NewID NodeID
	// Contract is the edge contract for set_relationship: nil leaves an existing edge's
	// contract unchanged, a non-nil empty contract clears it.
	Contract *EdgeContract
	// Constraints is the full replacement list for set_constraints (nil or empty clears).
	Constraints []Constraint
}

// StageInput is one authoring call: a source tool name, an optional summary, and
// the operations in apply order.
type StageInput struct {
	Source  string
	Summary string
	Ops     []StageOp
	// ChangeSet is an optional slug that groups related proposals for display and review.
	ChangeSet string
	// Origin identifies the staging caller (one MCP session). Only the same origin may
	// withdraw the proposal or supersede it. Empty means nobody but the architect can.
	Origin string
	// Supersedes names a pending proposal with the same Origin that is withdrawn once this
	// one is staged. Its create ids are free for reuse by this proposal.
	Supersedes string
	// DependsOnProposal names a pending proposal whose result this one is validated
	// against. Accepting this proposal is refused while that one is still pending.
	DependsOnProposal string
}

// ProposalSummary is one proposal as listed, with staleness computed at read time.
type ProposalSummary struct {
	ID        string `json:"proposal_id"`
	CreatedAt string `json:"created_at,omitempty"`
	Source    string `json:"source,omitempty"`
	Summary   string `json:"summary,omitempty"`
	ChangeSet string `json:"change_set,omitempty"`
	// DependsOnProposal is a pending proposal this one was validated against.
	DependsOnProposal string      `json:"depends_on_proposal,omitempty"`
	Unreadable        bool        `json:"unreadable"`
	Error             string      `json:"error,omitempty"`
	Operations        []OpSummary `json:"operations,omitempty"`
}

// OpSummary is one operation without the file diff.
type OpSummary struct {
	Index       int      `json:"index"`
	Op          string   `json:"op"`
	NodeID      string   `json:"node_id"`
	NewID       string   `json:"new_id,omitempty"`
	Status      string   `json:"status"`
	Stale       bool     `json:"stale"`
	StaleReason string   `json:"stale_reason,omitempty"`
	Fields      []string `json:"fields,omitempty"`
}

// ProposalDiff is the current and proposed state of each operation.
type ProposalDiff struct {
	ID        string `json:"proposal_id"`
	CreatedAt string `json:"created_at"`
	Source    string `json:"source"`
	Summary   string `json:"summary,omitempty"`
	ChangeSet string `json:"change_set,omitempty"`
	// DependsOnProposal is a pending proposal this one was validated against.
	DependsOnProposal string   `json:"depends_on_proposal,omitempty"`
	Operations        []OpDiff `json:"operations"`
}

// OpDiff is one operation ready to render. Stale is computed here, not stored.
type OpDiff struct {
	Index               int           `json:"index"`
	Op                  string        `json:"op"`
	NodeID              string        `json:"node_id"`
	NewID               string        `json:"new_id,omitempty"`
	Status              string        `json:"status"`
	Stale               bool          `json:"stale"`
	StaleReason         string        `json:"stale_reason,omitempty"`
	Fields              []string      `json:"fields,omitempty"`
	Current             *NodeSnapshot `json:"current"`
	Proposed            *NodeSnapshot `json:"proposed"`
	Relationship        *RelSnapshot  `json:"relationship"`
	CurrentRelationship *RelSnapshot  `json:"current_relationship"`
	// CurrentConstraints and ProposedConstraints are set only for set_constraints
	// (absent means an empty list).
	CurrentConstraints  []Constraint `json:"current_constraints,omitempty"`
	ProposedConstraints []Constraint `json:"proposed_constraints,omitempty"`
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
	Risk           string         `json:"risk,omitempty"`
	Review         string         `json:"review,omitempty"`
	Verify         []string       `json:"verify,omitempty"`
	Markdown       string         `json:"markdown"`
	Icon           string         `json:"icon,omitempty"`
	Color          string         `json:"color,omitempty"`
	Renderer       string         `json:"renderer,omitempty"`
	Schema         []SchemaColumn `json:"schema,omitempty"`
	DependsOn      []string       `json:"depends_on,omitempty"`
	Contract       *NodeContract  `json:"contract,omitempty"`
	External       bool           `json:"external,omitempty"`
	HumanGates     []HumanGate    `json:"human_gates,omitempty"`
	Deliverables   []string       `json:"deliverables,omitempty"`
	References     []string       `json:"references,omitempty"`
	Shared         bool           `json:"shared,omitempty"`
	Milestones     []Milestone    `json:"milestones,omitempty"`
	Gates          []string       `json:"gates,omitempty"`
}

// RelSnapshot is one directed edge.
type RelSnapshot struct {
	ID       string        `json:"id,omitempty"`
	From     string        `json:"from"`
	To       string        `json:"to"`
	Label    string        `json:"label"`
	Kind     string        `json:"kind"`
	Contract *EdgeContract `json:"contract,omitempty"`
	// ContractCleared is true on a set_relationship row whose staged contract is
	// explicitly empty (it clears the edge's contract). Contract is then absent.
	ContractCleared bool `json:"contract_cleared,omitempty"`
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
	ManifestVersion int    `json:"manifest_version"`
	ProposalID      string `json:"proposal_id"`
	CreatedAt       string `json:"created_at"`
	Source          string `json:"source"`
	Summary         string `json:"summary,omitempty"`
	ChangeSet       string `json:"change_set,omitempty"`
	Origin          string `json:"origin,omitempty"`
	// DependsOnProposal is a proposal this one was staged against; see StageInput.
	DependsOnProposal string      `json:"depends_on_proposal,omitempty"`
	Operations        []operation `json:"operations"`
}

type operation struct {
	Op       string   `json:"op"`
	NodeID   string   `json:"node_id"`
	BaseHash *string  `json:"base_hash"`
	Fields   []string `json:"fields,omitempty"`
	From     string   `json:"from,omitempty"`
	To       string   `json:"to,omitempty"`
	RelID    string   `json:"relationship_id,omitempty"`
	NewID    string   `json:"new_id,omitempty"`
	Label    *string  `json:"label,omitempty"`
	Kind     *string  `json:"kind,omitempty"`
	// Contract is the staged edge contract (set_relationship); nil means unchanged and a
	// non-nil empty contract clears it.
	Contract *EdgeContract `json:"contract,omitempty"`
	// RelCovered is true when base_hash also covers the relationship's own JSON (the edge
	// existed on disk at stage time and was not already touched by an earlier op).
	RelCovered bool `json:"rel_covered,omitempty"`
	// Constraints is the full replacement list for set_constraints (non-nil, possibly empty).
	Constraints *[]Constraint `json:"constraints,omitempty"`
	Status      string        `json:"status"`
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
	id, _, err := idx.StageWithWarning(in)
	return id, err
}

// StageWithWarning is Stage that also returns a non-fatal warning (the proposal was staged,
// but e.g. the superseded one could not be removed).
func (idx *Index) StageWithWarning(in StageInput) (string, string, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.stageWarning = ""
	id, err := idx.stageLocked(in)
	w := idx.stageWarning
	idx.stageWarning = ""
	return id, w, err
}

// stagingCtx is state that only exists while stageLocked runs (idx.mu is held).
type stagingCtx struct {
	// skipPending lists proposals whose pending create ids are free to reuse.
	skipPending map[string]bool
	// predMaterial holds the staged node bodies of a depends_on_proposal predecessor, so
	// base hashes match the files once that predecessor is accepted.
	predMaterial map[NodeID]materialBody
}

var changeSetPattern = slugPattern

// stageLocked is Stage for callers that already hold idx.mu.
func (idx *Index) stageLocked(in StageInput) (string, error) {
	if !validProposalSource(in.Source) {
		return "", fmt.Errorf("%w: source %q", ErrBadOperation, in.Source)
	}
	if len(in.Ops) == 0 {
		return "", fmt.Errorf("%w: proposal has no operations", ErrBadOperation)
	}
	if in.ChangeSet != "" && (len(in.ChangeSet) > maxExplicitIDLen || !changeSetPattern.MatchString(in.ChangeSet)) {
		return "", fmt.Errorf("%w: change_set %q must be a lowercase slug of at most %d characters", ErrBadOperation, in.ChangeSet, maxExplicitIDLen)
	}
	if in.Supersedes != "" && in.Supersedes == in.DependsOnProposal {
		return "", fmt.Errorf("%w: a proposal cannot both supersede and depend on %s", ErrBadOperation, in.Supersedes)
	}

	nodes := map[NodeID]*Node{}
	for id, n := range idx.Nodes {
		nodes[id] = cloneNode(n)
	}
	material := map[NodeID]materialBody{}
	ops := make([]operation, 0, len(in.Ops))
	files := map[NodeID]materialBody{}
	stagedRels := []Relationship{}

	idx.staging = &stagingCtx{skipPending: map[string]bool{}, predMaterial: map[NodeID]materialBody{}}
	defer func() { idx.staging = nil }()

	var superseded *manifest
	if in.Supersedes != "" {
		m, err := idx.ownedPending(in.Supersedes, in.Origin)
		if err != nil {
			return "", fmt.Errorf("supersedes: %w", err)
		}
		superseded = &m
		idx.staging.skipPending[in.Supersedes] = true
	}
	if in.DependsOnProposal != "" {
		m, err := idx.loadReadable(in.DependsOnProposal)
		if err != nil {
			return "", fmt.Errorf("depends_on_proposal: %w", err)
		}
		if err := idx.seedFromPredecessor(m, nodes, &stagedRels); err != nil {
			return "", err
		}
	}

	sawConstraints := false
	for _, inOp := range orderOps(in.Ops) {
		if inOp.Op == OpSetConstraints {
			if sawConstraints {
				return "", fmt.Errorf("%w: set_constraints may appear once per proposal", ErrBadOperation)
			}
			sawConstraints = true
		}
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
	if err := idx.validateFinalState(nodes); err != nil {
		return "", err
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
		ManifestVersion:   proposalManifestVersion,
		ProposalID:        id,
		CreatedAt:         now.Format(time.RFC3339),
		Source:            in.Source,
		Summary:           in.Summary,
		ChangeSet:         in.ChangeSet,
		Origin:            in.Origin,
		DependsOnProposal: in.DependsOnProposal,
		Operations:        ops,
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
	if superseded != nil {
		idx.archiveResolved(*superseded, ProposalWithdrawn)
		if err := os.RemoveAll(idx.mustProposalPath(superseded.ProposalID)); err != nil {
			// The new proposal is staged; report success with a warning, not an error.
			idx.stageWarning = fmt.Sprintf("staged %s but could not withdraw superseded %s: %v; withdraw it manually", id, superseded.ProposalID, err)
		}
	}
	return id, nil
}

// ownedPending loads a pending proposal and checks that origin staged it.
func (idx *Index) ownedPending(id, origin string) (manifest, error) {
	m, err := idx.loadReadable(id)
	if err != nil {
		return manifest{}, err
	}
	if origin == "" || m.Origin != origin {
		return manifest{}, fmt.Errorf("%w: proposal %s was not staged by this caller; only the architect (or the session that staged it) can withdraw it", ErrBadOperation, id)
	}
	return m, nil
}

// WithdrawProposal removes a pending proposal on behalf of the caller that staged it
// (origin). The proposal stays queryable as withdrawn.
func (idx *Index) WithdrawProposal(id, origin string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	m, err := idx.ownedPending(id, origin)
	if err != nil {
		return err
	}
	idx.archiveResolved(m, ProposalWithdrawn)
	return os.RemoveAll(idx.mustProposalPath(id))
}

// seedFromPredecessor applies a pending proposal's operations to the scratch state, so a
// dependent proposal validates against the model as it will be once that one is accepted.
func (idx *Index) seedFromPredecessor(m manifest, nodes map[NodeID]*Node, stagedRels *[]Relationship) error {
	dir := idx.mustProposalPath(m.ProposalID)
	for _, op := range m.Operations {
		if op.Status != opStatusPending {
			continue
		}
		switch op.Op {
		case OpCreateNode, OpUpdateNode:
			jsonBytes, mdBytes, err := readPair(filepath.Join(dir, "nodes", op.NodeID))
			if err != nil {
				return err
			}
			node, err := decodeNode(jsonBytes)
			if err != nil {
				return fmt.Errorf("%w: %s %s: %v", ErrUnreadableProposal, op.Op, op.NodeID, err)
			}
			node.Spec = string(mdBytes)
			nodes[node.ID] = node
			idx.staging.predMaterial[node.ID] = materialBody{json: jsonBytes, md: mdBytes}
		case OpDeleteNode:
			delete(nodes, NodeID(op.NodeID))
		case OpRenameNode:
			renameScratch(nodes, stagedRels, NodeID(op.NodeID), NodeID(op.NewID))
		case OpSetRelationship:
			*stagedRels = append(*stagedRels, Relationship{
				ID: RelID(op.RelID), From: NodeID(op.From), To: NodeID(op.To),
				Label: deref(op.Label), Kind: deref(op.Kind), Contract: op.Contract,
			})
		}
	}
	return nil
}

// dependsOnPending is an error while the proposal m depends on is still pending.
func (idx *Index) dependsOnPending(m manifest) error {
	if m.DependsOnProposal == "" {
		return nil
	}
	dir, err := idx.proposalPath(m.DependsOnProposal)
	if err != nil {
		return nil
	}
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%w: proposal %s depends on %s, which is still pending; accept or reject that one first", ErrStale, m.ProposalID, m.DependsOnProposal)
	}
	// No longer pending: refuse when it was withdrawn (or superseded) or fully rejected,
	// because the model it was validated against will never exist.
	if info, err := idx.resolvedStateLocked(m.DependsOnProposal); err == nil &&
		(info.Status == ProposalWithdrawn || info.Status == ProposalRejected) {
		return fmt.Errorf("%w: proposal %s depends on %s, which was %s; re-stage this proposal against the current model", ErrStale, m.ProposalID, m.DependsOnProposal, info.Status)
	}
	return nil
}

// validateFinalState checks the hierarchy and dependency graph of every node a proposal
// added or changed against the scratch state after all of its operations, so the answer
// does not depend on the order the operations were written in.
func (idx *Index) validateFinalState(nodes map[NodeID]*Node) error {
	ids := make([]NodeID, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sortNodeIDs(ids)
	for _, id := range ids {
		n := nodes[id]
		cur := idx.Nodes[id]
		if cur != nil && cur.ParentID == n.ParentID && slicesEqualIDs(cur.DependsOn, n.DependsOn) && slicesEqualIDs(cur.Gates, n.Gates) {
			continue
		}
		if parentCyclesIn(nodes, id, n.ParentID) {
			return fmt.Errorf("%w: %s -> %s", ErrCycle, id, n.ParentID)
		}
		if err := validateDependsOnIn(nodes, id, n.ParentID, n.DependsOn); err != nil {
			return err
		}
		if err := validateGatesIn(nodes, id, n.ParentID, n.DependsOn, n.Gates); err != nil {
			return err
		}
	}
	return nil
}

func (idx *Index) stageOne(in StageOp, nodes map[NodeID]*Node, material map[NodeID]materialBody, stagedRels *[]Relationship) (operation, *materialBody, error) {
	switch in.Op {
	case OpCreateNode:
		return idx.stageCreate(in, nodes, material)
	case OpUpdateNode:
		return idx.stageUpdate(in, nodes, material)
	case OpDeleteNode:
		return idx.stageDelete(in, nodes, material)
	case OpRenameNode:
		return idx.stageRename(in, nodes, material, stagedRels)
	case OpSetRelationship:
		return idx.stageRelationship(in, nodes, material, stagedRels)
	case OpDeleteRelationship:
		return idx.stageDeleteRelationship(in, nodes, material, stagedRels)
	case OpSetConstraints:
		return idx.stageConstraints(in)
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
	contract, err := patchNodeContract(in.Create.Contract, nil)
	if err != nil {
		return operation{}, nil, err
	}
	if err := validateDependsOnIn(nodes, id, in.Create.ParentID, in.Create.DependsOn); err != nil {
		return operation{}, nil, err
	}
	metaRisk, metaReview, metaVerify, err := normalizeNodeMeta(in.Create.Risk, in.Create.Review, in.Create.Verify)
	if err != nil {
		return operation{}, nil, err
	}
	extras, err := normalizeExtras(in.Create.NodeExtras)
	if err != nil {
		return operation{}, nil, err
	}
	if err := validateGatesIn(nodes, id, in.Create.ParentID, in.Create.DependsOn, extras.Gates); err != nil {
		return operation{}, nil, err
	}
	node := &Node{
		NodeExtras:     extras,
		ID:             id,
		Name:           in.Create.Name,
		Type:           in.Create.Type,
		Status:         status,
		ParentID:       in.Create.ParentID,
		RelationshipID: in.Create.RelationshipID,
		Implementation: cloneStrings(in.Create.Implementation),
		Scope:          cloneStrings(in.Create.Scope),
		Protected:      in.Create.Protected,
		Risk:           metaRisk,
		Review:         metaReview,
		Verify:         metaVerify,
		Spec:           in.Create.Spec,
		Icon:           strings.TrimSpace(in.Create.Icon),
		Color:          strings.TrimSpace(in.Create.Color),
		Renderer:       strings.TrimSpace(in.Create.Renderer),
		Schema:         normalizeSchema(in.Create.Schema),
		DependsOn:      sortedDeps(in.Create.DependsOn),
		Contract:       contract,
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
	if err := checkNoDependentsIn(nodes, map[NodeID]bool{in.NodeID: true}); err != nil {
		return operation{}, nil, err
	}
	hash, err := idx.hashFor(in.NodeID, material)
	if err != nil {
		return operation{}, nil, err
	}
	delete(nodes, in.NodeID)
	op := operation{Op: OpDeleteNode, NodeID: string(in.NodeID), BaseHash: &hash, Status: opStatusPending}
	return op, nil, nil
}

// effectiveRels is the on-disk relationships with later staged entries (same id) applied,
// then staged-only ones appended.
func effectiveRels(disk, staged []Relationship) []Relationship {
	out := append([]Relationship(nil), disk...)
	for _, st := range staged {
		replaced := false
		for i := range out {
			if out[i].ID == st.ID {
				out[i] = st
				replaced = true
			}
		}
		if !replaced {
			out = append(out, st)
		}
	}
	return out
}

// findEdge returns the relationship with the same endpoints and label, or nil.
func findEdge(rels []Relationship, from, to NodeID, label string) *Relationship {
	for i := range rels {
		if rels[i].From == from && rels[i].To == to && rels[i].Label == label {
			return &rels[i]
		}
	}
	return nil
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
	all := effectiveRels(idx.Relationships, *stagedRels)
	label, kind := in.Label, in.Kind
	relID := in.RelID
	if relID != "" {
		found := false
		for _, rel := range all {
			if rel.ID == relID {
				found = true
				break
			}
		}
		if !found {
			return operation{}, nil, fmt.Errorf("%w: %q (to create an edge omit relationship_id; to change one, pass the id of an existing edge)", ErrRelationshipNotFound, in.RelID)
		}
	} else if match := findEdge(all, in.From, in.To, label); match != nil {
		// Upsert: the same (from, to, label) edge is updated in place, not duplicated.
		relID = match.ID
		if kind == "" {
			kind = match.Kind
		}
	}
	hash, err := idx.hashFor(in.From, material)
	if err != nil {
		return operation{}, nil, err
	}

	// The contract in effect before this op: an earlier op of this proposal, else disk.
	var current *EdgeContract
	var onDisk *Relationship
	touched := false
	if relID != "" {
		onDisk, _ = idx.relByID(relID)
		if onDisk != nil {
			current = onDisk.Contract
		}
		for _, rel := range *stagedRels {
			if rel.ID == relID {
				touched = true
				current = rel.Contract
			}
		}
	}
	var opContract *EdgeContract
	effective := current
	if in.Contract != nil {
		next, err := patchEdgeContract(in.Contract, current)
		if err != nil {
			return operation{}, nil, err
		}
		effective = next
		opContract = next
		if opContract == nil {
			opContract = &EdgeContract{}
		}
	}
	covered := onDisk != nil && !touched
	if covered {
		hash = relBaseHash(hash, onDisk)
	}
	isNew := relID == ""
	if isNew {
		id, err := idx.allocateRelationshipIDStaged(label, *stagedRels)
		if err != nil {
			return operation{}, nil, err
		}
		relID = id
	}
	*stagedRels = append(*stagedRels, Relationship{ID: relID, From: in.From, To: in.To, Label: label, Kind: kind, Contract: effective})
	op := operation{
		Op:         OpSetRelationship,
		NodeID:     string(in.From),
		BaseHash:   &hash,
		From:       string(in.From),
		To:         string(in.To),
		RelID:      string(relID),
		Label:      &label,
		Kind:       &kind,
		Contract:   opContract,
		RelCovered: covered,
		Status:     opStatusPending,
	}
	return op, nil, nil
}

// stageRename validates a rename against the scratch state and moves the node, and every
// reference to it, to the new id there.
func (idx *Index) stageRename(in StageOp, nodes map[NodeID]*Node, material map[NodeID]materialBody, stagedRels *[]Relationship) (operation, *materialBody, error) {
	if _, ok := nodes[in.NodeID]; !ok {
		return operation{}, nil, fmt.Errorf("%w: %q", ErrNotFound, in.NodeID)
	}
	if in.NewID == "" || in.NewID == in.NodeID {
		return operation{}, nil, fmt.Errorf("%w: rename_node %s needs a new_id that differs from the current id", ErrBadOperation, in.NodeID)
	}
	if err := idx.checkExplicitID(in.NewID, nodes); err != nil {
		return operation{}, nil, fmt.Errorf("rename_node %s -> %s: %w", in.NodeID, in.NewID, err)
	}
	if idx.relationshipIDTaken(string(in.NewID), *stagedRels) {
		return operation{}, nil, fmt.Errorf("%w: rename_node %s -> %q is already a relationship id", ErrExists, in.NodeID, in.NewID)
	}
	hash, err := idx.hashFor(in.NodeID, material)
	if err != nil {
		return operation{}, nil, err
	}
	renameScratch(nodes, stagedRels, in.NodeID, in.NewID)
	op := operation{
		Op:       OpRenameNode,
		NodeID:   string(in.NodeID),
		NewID:    string(in.NewID),
		BaseHash: &hash,
		Fields:   []string{"id"},
		Status:   opStatusPending,
	}
	return op, nil, nil
}

func (idx *Index) relationshipIDTaken(id string, staged []Relationship) bool {
	for _, rel := range effectiveRels(idx.Relationships, staged) {
		if string(rel.ID) == id {
			return true
		}
	}
	return false
}

// renameScratch moves node old to new in a scratch node map and rewrites parent_id,
// depends_on and staged relationship endpoints that name it.
func renameScratch(nodes map[NodeID]*Node, stagedRels *[]Relationship, old, nw NodeID) {
	n, ok := nodes[old]
	if !ok {
		return
	}
	delete(nodes, old)
	n.ID = nw
	nodes[nw] = n
	for id, other := range nodes {
		cp := cloneNode(other)
		if cp.ParentID == old {
			cp.ParentID = nw
		}
		cp.DependsOn = renameIDs(cp.DependsOn, old, nw)
		cp.Gates = renameIDs(cp.Gates, old, nw)
		nodes[id] = cp
	}
	if stagedRels != nil {
		for i := range *stagedRels {
			r := &(*stagedRels)[i]
			if r.From == old {
				r.From = nw
			}
			if r.To == old {
				r.To = nw
			}
		}
	}
}

// renameIDs replaces old with nw in ids and returns the list sorted.
func renameIDs(ids []NodeID, old, nw NodeID) []NodeID {
	changed := false
	out := make([]NodeID, len(ids))
	for i, d := range ids {
		if d == old {
			d = nw
			changed = true
		}
		out[i] = d
	}
	if !changed {
		return ids
	}
	return sortedDeps(out)
}

// relBaseHash folds a relationship's own JSON into a node hash (decision 7).
func relBaseHash(nodeHash string, rel *Relationship) string {
	raw, err := encodeRelationship(*rel)
	if err != nil {
		raw = nil
	}
	sum := sha256.New()
	sum.Write([]byte(nodeHash + "\x00rel\x00"))
	sum.Write(raw)
	return "sha256:" + hex.EncodeToString(sum.Sum(nil))
}

// configHash is the sha256 of the config.json bytes (empty when the file is absent).
func (idx *Index) configHash() (string, error) {
	data, _, err := readOptional(filepath.Join(idx.arch(), "config.json"))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func (idx *Index) stageConstraints(in StageOp) (operation, *materialBody, error) {
	list, err := normalizeConstraintList(in.Constraints)
	if err != nil {
		return operation{}, nil, err
	}
	hash, err := idx.configHash()
	if err != nil {
		return operation{}, nil, err
	}
	return operation{Op: OpSetConstraints, BaseHash: &hash, Constraints: &list, Status: opStatusPending}, nil, nil
}

func (idx *Index) stageDeleteRelationship(in StageOp, nodes map[NodeID]*Node, material map[NodeID]materialBody, stagedRels *[]Relationship) (operation, *materialBody, error) {
	if in.RelID == "" {
		return operation{}, nil, fmt.Errorf("%w: delete_relationship needs relationship_id", ErrBadOperation)
	}
	rel, _ := idx.relByID(in.RelID)
	if rel == nil {
		return operation{}, nil, fmt.Errorf("%w: %q", ErrRelationshipNotFound, in.RelID)
	}
	if interior := idx.removedByRelationship(rel); len(interior) > 0 {
		remove := map[NodeID]bool{}
		for _, nid := range interior {
			remove[NodeID(nid)] = true
		}
		if err := checkNoDependentsIn(nodes, remove); err != nil {
			return operation{}, nil, err
		}
	}
	// An earlier op of this proposal that touched the edge means its on-disk JSON is no
	// longer what the proposal started from, so the hash must not cover it (as in
	// stageRelationship).
	covered := true
	label, kind := rel.Label, rel.Kind
	for _, staged := range *stagedRels {
		if staged.ID == rel.ID {
			covered = false
			label, kind = staged.Label, staged.Kind
		}
	}
	hash, err := idx.deleteRelHash(rel, func(id NodeID) (string, error) { return idx.hashFor(id, material) }, covered)
	if err != nil {
		return operation{}, nil, err
	}
	op := operation{
		Op:         OpDeleteRelationship,
		NodeID:     string(rel.From),
		BaseHash:   &hash,
		From:       string(rel.From),
		To:         string(rel.To),
		RelID:      string(rel.ID),
		Label:      &label,
		Kind:       &kind,
		RelCovered: covered,
		Status:     opStatusPending,
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
func (idx *Index) deleteRelHash(rel *Relationship, hashOf func(NodeID) (string, error), covered bool) (string, error) {
	h, err := hashOf(rel.From)
	if err != nil {
		return "", err
	}
	if covered {
		h = relBaseHash(h, rel)
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
	if idx.staging != nil {
		if body, ok := idx.staging.predMaterial[id]; ok {
			return contentHash(body.json, body.md), nil
		}
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
		ID:                m.ProposalID,
		CreatedAt:         m.CreatedAt,
		Source:            m.Source,
		Summary:           m.Summary,
		ChangeSet:         m.ChangeSet,
		DependsOnProposal: m.DependsOnProposal,
		Operations:        make([]OpDiff, len(m.Operations)),
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
	if err := idx.dependsOnPending(m); err != nil {
		return err
	}
	if op.Op == OpRenameNode {
		for i := 0; i < index; i++ {
			if m.Operations[i].Status == opStatusPending {
				return fmt.Errorf("%w: rename_node %s -> %s cannot be accepted while earlier operation %d (%s) is still pending; resolve it first or use accept_all", ErrStale, op.NodeID, op.NewID, i, m.Operations[i].Op)
			}
		}
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
	if err := idx.dependsOnPending(m); err != nil {
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
	config  []byte
	// configOK is false when config.json did not exist.
	configOK bool
	// local is local.json (the machine-local assignment); localOK is false when absent.
	local   []byte
	localOK bool
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
	config, configOK, err := readOptional(filepath.Join(idx.arch(), "config.json"))
	if err != nil {
		return modelSnapshot{}, err
	}
	local, localOK, err := readOptional(filepath.Join(idx.arch(), "local.json"))
	if err != nil {
		return modelSnapshot{}, err
	}
	return modelSnapshot{index: index, nodes: nodes, layouts: layouts, config: config, configOK: configOK, local: local, localOK: localOK}, nil
}

// restoreFiles writes a snapshot back to disk (it does not reload the index).
func (idx *Index) restoreFiles(snap modelSnapshot) error {
	if err := restoreDirFiles(filepath.Join(idx.arch(), "nodes"), snap.nodes); err != nil {
		return err
	}
	if err := restoreDirFiles(idx.layoutDir(), snap.layouts); err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(idx.arch(), "index.json"), snap.index); err != nil {
		return err
	}
	if err := restoreFile(filepath.Join(idx.arch(), "config.json"), snap.config, snap.configOK); err != nil {
		return err
	}
	return restoreFile(filepath.Join(idx.arch(), "local.json"), snap.local, snap.localOK)
}

// rollbackAccept restores the model and the proposal manifest to their state before
// an AcceptAll call started.
func (idx *Index) rollbackAccept(snap modelSnapshot, m *manifest, original []operation) error {
	if err := idx.restoreFiles(snap); err != nil {
		return err
	}
	m.Operations = original
	// A proposal that finished during the failed call was archived; it is pending again.
	if dir, err := idx.resolvedPath(m.ProposalID); err == nil {
		_ = os.RemoveAll(dir)
	}
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
	if m, err := readManifest(dir); err == nil && m.ManifestVersion == proposalManifestVersion && m.ProposalID == id {
		idx.archiveResolved(m, ProposalWithdrawn)
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
	if reason == staleHasDependents {
		return fmt.Errorf("%w: %s %s is stale because %s and cannot be applied", ErrStale, op.Op, op.NodeID, reason)
	}
	if !confirm {
		return fmt.Errorf("%w: %s %s is stale; confirm_stale is required to apply over the architect's newer edit", ErrStale, op.Op, op.NodeID)
	}
	return nil
}

const (
	staleMissing  = "node is missing"
	staleMismatch = "base_hash mismatch"

	staleHasDependents = "node now has dependents"

	staleCreateNotAccepted   = "container create_node was not accepted"
	staleEndpointNotAccepted = "edge endpoint create_node was not accepted"

	staleDependsOnNotAccepted = "depends_on create_node was not accepted"
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
	case OpRenameNode:
		if notAccepted(op.NodeID) {
			return staleCreateNotAccepted
		}
	case OpCreateNode, OpUpdateNode:
		jsonBytes, _, err := readPair(filepath.Join(idx.mustProposalPath(proposalID), "nodes", op.NodeID))
		if err != nil {
			return ""
		}
		node, err := decodeNode(jsonBytes)
		if err != nil {
			return ""
		}
		if node.ParentID != "" && notAccepted(string(node.ParentID)) {
			return staleCreateNotAccepted
		}
		for _, d := range node.DependsOn {
			if notAccepted(string(d)) {
				return staleDependsOnNotAccepted
			}
		}
		for _, d := range node.Gates {
			if notAccepted(string(d)) {
				return staleDependsOnNotAccepted
			}
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
		if idx.staging != nil && idx.staging.skipPending[pid] {
			continue
		}
		summary, m, err := idx.inspect(pid)
		if err != nil || summary.Unreadable {
			continue
		}
		for _, op := range m.Operations {
			if op.Status != opStatusPending {
				continue
			}
			switch op.Op {
			case OpCreateNode:
				out[op.NodeID] = true
			case OpRenameNode:
				out[op.NewID] = true
			}
		}
	}
	return out
}

func (idx *Index) staleness(ops []operation, op operation) (bool, string) {
	if op.Op == OpCreateNode || op.Status != opStatusPending {
		return false, ""
	}
	if op.Op == OpSetConstraints {
		hash, err := idx.configHash()
		if err != nil || op.BaseHash == nil || *op.BaseHash != hash {
			return true, staleMismatch
		}
		return false, ""
	}
	if op.Op == OpDeleteRelationship {
		rel, _ := idx.relByID(RelID(op.RelID))
		if rel == nil {
			return true, staleMissing
		}
		if interior := idx.removedByRelationship(rel); len(interior) > 0 {
			remove := map[NodeID]bool{}
			for _, nid := range interior {
				remove[NodeID(nid)] = true
			}
			if idx.checkNoDependents(remove) != nil {
				return true, staleHasDependents
			}
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
		}, op.RelCovered)
		if err != nil {
			return true, staleMissing
		}
		if op.BaseHash == nil || *op.BaseHash != hash {
			return true, staleMismatch
		}
		return false, ""
	}
	if op.Op == OpDeleteNode && idx.Nodes[NodeID(op.NodeID)] != nil {
		// Applying would also remove the interior of every edge touching the node.
		remove := map[NodeID]bool{NodeID(op.NodeID): true}
		for _, rel := range idx.Relationships {
			if string(rel.From) == op.NodeID || string(rel.To) == op.NodeID {
				for nid := range idx.relationshipClosure(rel.ID).nodes {
					remove[nid] = true
				}
			}
		}
		if idx.checkNoDependents(remove) != nil {
			return true, staleHasDependents
		}
	}
	hash, ok, err := idx.hashOnDisk(NodeID(op.NodeID))
	if err != nil || !ok {
		if op.Op == OpSetRelationship && pendingCreate(ops, op.NodeID) {
			return false, ""
		}
		if op.Op == OpUpdateNode || op.Op == OpDeleteNode || op.Op == OpRenameNode || op.Op == OpSetRelationship || op.Op == OpDeleteRelationship {
			return true, staleMissing
		}
		return false, ""
	}
	if op.Op == OpSetRelationship && op.RelCovered {
		rel, _ := idx.relByID(RelID(op.RelID))
		if rel == nil {
			return true, staleMissing
		}
		hash = relBaseHash(hash, rel)
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
	case OpRenameNode:
		return idx.applyRename(op)
	case OpSetRelationship:
		in := SetRelationshipInput{
			ID:       RelID(op.RelID),
			From:     NodeID(op.From),
			To:       NodeID(op.To),
			Label:    deref(op.Label),
			Kind:     deref(op.Kind),
			Contract: op.Contract,

			AllowEndpointChange: true,
		}
		if _, err := idx.setRelationshipLocked(in); err != nil {
			return err
		}
		return idx.reload()
	case OpDeleteRelationship:
		return idx.deleteRelationshipLocked(RelID(op.RelID))
	case OpSetConstraints:
		if op.Constraints == nil {
			return fmt.Errorf("%w: set_constraints has no constraints list", ErrUnreadableProposal)
		}
		return idx.setConstraintsLocked(*op.Constraints)
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
	if err := idx.checkNoDependents(removeNodes); err != nil {
		return err
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
	if err := idx.dropGatesCleared(removeNodes); err != nil {
		return err
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
	// Re-validate only a changed list so an unrelated update is not blocked by a
	// dangling entry the architect already has on disk.
	if cur := idx.Nodes[n.ID]; creating || cur == nil || !slicesEqualIDs(cur.DependsOn, n.DependsOn) {
		if err := idx.validateDependsOn(n.ID, n.ParentID, n.DependsOn); err != nil {
			return err
		}
	}
	if cur := idx.Nodes[n.ID]; creating || cur == nil || !slicesEqualIDs(cur.Gates, n.Gates) {
		if err := validateGatesIn(idx.Nodes, n.ID, n.ParentID, n.DependsOn, n.Gates); err != nil {
			return err
		}
	}
	return nil
}

func (idx *Index) finishOp(m *manifest, index int, status string) error {
	m.Operations[index].Status = status
	dir := idx.mustProposalPath(m.ProposalID)
	if allResolved(m.Operations) {
		idx.archiveResolved(*m, outcomeOf(m.Operations))
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
		ID:                m.ProposalID,
		CreatedAt:         m.CreatedAt,
		Source:            m.Source,
		Summary:           m.Summary,
		ChangeSet:         m.ChangeSet,
		DependsOnProposal: m.DependsOnProposal,
		Operations:        make([]OpSummary, len(m.Operations)),
	}
	for i, op := range m.Operations {
		stale, reason := idx.staleness(m.Operations, op)
		summary.Operations[i] = OpSummary{
			Index:       i,
			Op:          op.Op,
			NodeID:      op.NodeID,
			NewID:       op.NewID,
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
		NewID:       op.NewID,
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
			row.CurrentRelationship = relSnapshot(rel)
			row.Relationship.Contract = row.CurrentRelationship.Contract
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
		var cur *Relationship
		if op.RelID != "" {
			cur, _ = idx.relByID(RelID(op.RelID))
		} else {
			for i := range idx.Relationships {
				rel := &idx.Relationships[i]
				if string(rel.From) == op.From && string(rel.To) == op.To {
					cur = rel
					break
				}
			}
		}
		if cur != nil {
			row.CurrentRelationship = relSnapshot(cur)
		}
		if op.Contract != nil {
			row.Relationship.Contract = normalizeEdgeContract(op.Contract)
			row.Relationship.ContractCleared = row.Relationship.Contract == nil
		} else if cur != nil {
			row.Relationship.Contract = row.CurrentRelationship.Contract
		}
	case OpRenameNode:
		if row.Current != nil {
			proposed := *row.Current
			proposed.ID = op.NewID
			row.Proposed = &proposed
		}
	case OpSetConstraints:
		row.CurrentConstraints = append([]Constraint(nil), idx.Constraints...)
		if op.Constraints != nil {
			row.ProposedConstraints = append([]Constraint(nil), *op.Constraints...)
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
	case SourceSeedModel, SourceGroupNodes, SourceUngroupNode, SourceReparentNode, OpCreateNode, OpUpdateNode, OpDeleteNode, OpSetRelationship, OpDeleteRelationship, OpRenameNode, OpSetConstraints:
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
	cp.DependsOn = append([]NodeID(nil), n.DependsOn...)
	cp.NodeExtras = cloneExtras(n.NodeExtras)
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
		return nil, fmt.Errorf("%w: update_node cannot set status; use update_node_status (it writes the status directly and applies the stage gate)", ErrBadOperation)
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
				return nil, fmt.Errorf("%w: %q (relationship_id on update_node puts the node inside an existing edge; to create or change an edge use set_relationship)", ErrRelationshipNotFound, rid)
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
	if in.Risk != nil || in.Review != nil || in.Verify != nil {
		risk, review, verify := n.Risk, n.Review, n.Verify
		if in.Risk != nil {
			risk = *in.Risk
			fields = append(fields, "risk")
		}
		if in.Review != nil {
			review = *in.Review
			fields = append(fields, "review")
		}
		if in.Verify != nil {
			verify = *in.Verify
			fields = append(fields, "verify")
		}
		var err error
		if n.Risk, n.Review, n.Verify, err = normalizeNodeMeta(risk, review, verify); err != nil {
			return nil, err
		}
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
	if in.DependsOn != nil {
		if err := validateDependsOnIn(nodes, n.ID, n.ParentID, *in.DependsOn); err != nil {
			return nil, err
		}
		n.DependsOn = sortedDeps(*in.DependsOn)
		fields = append(fields, "depends_on")
	}
	if extras, extraFields, err := in.NodeExtrasPatch.applyTo(n.NodeExtras); err != nil {
		return nil, err
	} else if len(extraFields) > 0 {
		n.NodeExtras = extras
		if in.Gates != nil {
			if err := validateGatesIn(nodes, n.ID, n.ParentID, n.DependsOn, n.Gates); err != nil {
				return nil, err
			}
		}
		fields = append(fields, extraFields...)
	}
	if in.Contract != nil {
		c, err := patchNodeContract(in.Contract, n.Contract)
		if err != nil {
			return nil, err
		}
		n.Contract = c
		fields = append(fields, "contract")
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
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "    ")
	if err := enc.Encode(m); err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, "manifest.json"), buf.Bytes())
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
		Risk:           n.Risk,
		Review:         n.Review,
		Verify:         append([]string(nil), n.Verify...),
		Markdown:       n.Spec,
		Icon:           n.Icon,
		Color:          n.Color,
		Renderer:       n.Renderer,
		Schema:         append([]SchemaColumn(nil), n.Schema...),
		DependsOn:      depStrings(n.DependsOn),
		Contract:       normalizeNodeContract(n.Contract),
		External:       n.External,
		HumanGates:     append([]HumanGate(nil), n.HumanGates...),
		Deliverables:   append([]string(nil), n.Deliverables...),
		References:     append([]string(nil), n.References...),
		Shared:         n.Shared,
		Milestones:     append([]Milestone(nil), n.Milestones...),
		Gates:          depStrings(n.Gates),
	}
}

func slicesEqualIDs(a, b []NodeID) bool {
	x, y := sortedDeps(a), sortedDeps(b)
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

func depStrings(deps []NodeID) []string {
	if len(deps) == 0 {
		return nil
	}
	out := make([]string, len(deps))
	for i, d := range sortedDeps(deps) {
		out[i] = string(d)
	}
	return out
}

func relSnapshot(rel *Relationship) *RelSnapshot {
	return &RelSnapshot{ID: string(rel.ID), From: string(rel.From), To: string(rel.To), Label: rel.Label, Kind: rel.Kind, Contract: normalizeEdgeContract(rel.Contract)}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
