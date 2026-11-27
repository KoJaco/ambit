package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/KoJaco/ambit/internal/core"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// proposalMetaIn is the optional proposal-level fields every staging tool accepts.
type proposalMetaIn struct {
	// ChangeSet groups related proposals (a lowercase slug) for display and review.
	ChangeSet string `json:"change_set,omitempty"`
	// Supersedes is a pending proposal staged by this session; it is withdrawn once this one is staged.
	Supersedes string `json:"supersedes,omitempty"`
	// DependsOnProposal is a pending proposal this one is validated against.
	DependsOnProposal string `json:"depends_on_proposal,omitempty"`
}

func (m proposalMetaIn) apply(in core.StageInput) core.StageInput {
	in.ChangeSet, in.Supersedes, in.DependsOnProposal = m.ChangeSet, m.Supersedes, m.DependsOnProposal
	return in
}

type seedModelIn struct {
	proposalMetaIn
	Transcript    string       `json:"transcript" jsonschema:"required"`
	ParentID      string       `json:"parent_id,omitempty"`
	Nodes         []seedNodeIn `json:"nodes,omitempty"`
	Relationships []seedRelIn  `json:"relationships,omitempty"`
	// Updates, DeleteRelationships and DeleteNodes let one proposal restructure a model.
	// Order of staging: creates, updates, relationships, relationship deletes, node deletes.
	Updates             []updateNodeIn `json:"updates,omitempty"`
	DeleteRelationships []string       `json:"delete_relationships,omitempty"`
	DeleteNodes         []string       `json:"delete_nodes,omitempty"`
}

type seedNodeIn struct {
	ID             string   `json:"id,omitempty"`
	Name           string   `json:"name" jsonschema:"required"`
	Type           string   `json:"type" jsonschema:"required"`
	ParentID       string   `json:"parent_id,omitempty"`
	RelationshipID string   `json:"relationship_id,omitempty"`
	Implementation []string `json:"implementation,omitempty"`
	Scope          []string `json:"scope,omitempty"`
	Protected      bool     `json:"protected,omitempty"`
	// Risk is low|medium|high, Review is none|required, Verify lists commands that prove the node works.
	Risk   string   `json:"risk,omitempty"`
	Review string   `json:"review,omitempty"`
	Verify []string `json:"verify,omitempty"`
	Spec   string   `json:"spec,omitempty"`
	Icon   string   `json:"icon,omitempty"`
	// DependsOn lists node ids that must be built first (real build ordering only).
	DependsOn []string `json:"depends_on,omitempty"`
	// Contract is the node interface: payloads, invariants, errors, owns_data.
	Contract *core.NodeContract `json:"contract,omitempty"`
}

type seedRelIn struct {
	From           string `json:"from" jsonschema:"required"`
	To             string `json:"to" jsonschema:"required"`
	Label          string `json:"label,omitempty"`
	Kind           string `json:"kind,omitempty"`
	RelationshipID string `json:"relationship_id,omitempty"`
	// Contract is what is on the wire: format (openapi|jsonschema|text), ref, text.
	Contract *core.EdgeContract `json:"contract,omitempty"`
}

// seedSummary builds the proposal summary, keeping up to summaryTranscriptMax
// runes of the transcript.
func seedSummary(ops int, transcript string) string {
	summary := fmt.Sprintf("%d operation(s) from transcript: ", ops)
	if r := []rune(transcript); len(r) > summaryTranscriptMax {
		return summary + string(r[:summaryTranscriptMax]) + "..."
	}
	return summary + transcript
}

// summaryTranscriptMax caps the transcript text kept in a proposal summary.
const summaryTranscriptMax = 500

func (e *toolEnv) handleSeedModel(ctx context.Context, _ *sdkmcp.CallToolRequest, in seedModelIn) (*sdkmcp.CallToolResult, stageResponse, error) {
	if strings.TrimSpace(in.Transcript) == "" {
		return toolErr(fmt.Errorf("%w: transcript is required", core.ErrBadOperation))
	}
	if len(in.Nodes) == 0 && len(in.Updates) == 0 && len(in.DeleteRelationships) == 0 && len(in.DeleteNodes) == 0 {
		return toolErr(fmt.Errorf("%w: proposal must contain at least one node, update, or delete", core.ErrBadOperation))
	}
	defaultParent := core.NodeID(in.ParentID)
	var ops []core.StageOp
	for _, n := range in.Nodes {
		parent := core.NodeID(n.ParentID)
		if parent == "" && defaultParent != "" && n.RelationshipID == "" {
			parent = defaultParent
		}
		ops = append(ops, core.StageOp{
			Op: core.OpCreateNode,
			Create: core.CreateInput{
				ID:             core.NodeID(n.ID),
				Name:           n.Name,
				Type:           n.Type,
				Spec:           n.Spec,
				ParentID:       parent,
				RelationshipID: core.RelID(n.RelationshipID),
				Implementation: n.Implementation,
				Scope:          n.Scope,
				Protected:      n.Protected,
				Risk:           n.Risk,
				Review:         n.Review,
				Verify:         n.Verify,
				Icon:           n.Icon,
				DependsOn:      nodeIDs(n.DependsOn),
				Contract:       n.Contract,
			},
		})
	}
	for _, u := range in.Updates {
		upd, err := updateOps(u)
		if err != nil {
			return toolErr(err)
		}
		ops = append(ops, upd...)
	}
	for _, r := range in.Relationships {
		ops = append(ops, core.StageOp{
			Op:       core.OpSetRelationship,
			From:     core.NodeID(r.From),
			To:       core.NodeID(r.To),
			Label:    r.Label,
			Kind:     r.Kind,
			RelID:    core.RelID(r.RelationshipID),
			Contract: r.Contract,
		})
	}
	for _, rid := range in.DeleteRelationships {
		ops = append(ops, core.StageOp{Op: core.OpDeleteRelationship, RelID: core.RelID(rid)})
	}
	for _, nid := range in.DeleteNodes {
		ops = append(ops, core.StageOp{Op: core.OpDeleteNode, NodeID: core.NodeID(nid)})
	}
	summary := seedSummary(len(ops), in.Transcript)
	_, out, err := e.stage(ctx, in.apply(core.StageInput{
		Source:  core.SourceSeedModel,
		Summary: summary,
		Ops:     ops,
	}))
	return nil, out, err
}

type createNodeIn struct {
	proposalMetaIn
	ID             string   `json:"id,omitempty"`
	Name           string   `json:"name" jsonschema:"required"`
	Type           string   `json:"type" jsonschema:"required"`
	ParentID       string   `json:"parent_id,omitempty"`
	RelationshipID string   `json:"relationship_id,omitempty"`
	Implementation []string `json:"implementation,omitempty"`
	Scope          []string `json:"scope,omitempty"`
	Protected      bool     `json:"protected,omitempty"`
	// Risk is low|medium|high, Review is none|required, Verify lists commands that prove the node works.
	Risk      string             `json:"risk,omitempty"`
	Review    string             `json:"review,omitempty"`
	Verify    []string           `json:"verify,omitempty"`
	Spec      string             `json:"spec,omitempty"`
	Icon      string             `json:"icon,omitempty"`
	DependsOn []string           `json:"depends_on,omitempty"`
	Contract  *core.NodeContract `json:"contract,omitempty"`
}

func (e *toolEnv) handleCreateNode(ctx context.Context, _ *sdkmcp.CallToolRequest, in createNodeIn) (*sdkmcp.CallToolResult, stageResponse, error) {
	_, out, err := e.stage(ctx, in.apply(core.StageInput{
		Source:  core.OpCreateNode,
		Summary: fmt.Sprintf("1 node (%s)", in.Name),
		Ops: []core.StageOp{{
			Op: core.OpCreateNode,
			Create: core.CreateInput{
				ID:             core.NodeID(in.ID),
				Name:           in.Name,
				Type:           in.Type,
				Spec:           in.Spec,
				ParentID:       core.NodeID(in.ParentID),
				RelationshipID: core.RelID(in.RelationshipID),
				Implementation: in.Implementation,
				Scope:          in.Scope,
				Protected:      in.Protected,
				Risk:           in.Risk,
				Review:         in.Review,
				Verify:         in.Verify,
				Icon:           in.Icon,
				DependsOn:      nodeIDs(in.DependsOn),
				Contract:       in.Contract,
			},
		}},
	}))
	return nil, out, err
}

type updateNodeIn struct {
	proposalMetaIn
	NodeID string `json:"node_id" jsonschema:"required"`
	// ID is not settable; it exists so a mistaken id gets an error that names rename_node.
	ID string `json:"id,omitempty"`
	// NewID renames the node (same as rename_node); other fields in the call apply first.
	NewID          string    `json:"new_id,omitempty"`
	Status         *string   `json:"status,omitempty"`
	Name           *string   `json:"name,omitempty"`
	Type           *string   `json:"type,omitempty"`
	ParentID       *string   `json:"parent_id,omitempty"`
	RelationshipID *string   `json:"relationship_id,omitempty"`
	Implementation *[]string `json:"implementation,omitempty"`
	Scope          *[]string `json:"scope,omitempty"`
	Protected      *bool     `json:"protected,omitempty"`
	Risk           *string   `json:"risk,omitempty"`
	Review         *string   `json:"review,omitempty"`
	Verify         *[]string `json:"verify,omitempty"`
	Spec           *string   `json:"spec,omitempty"`
	Icon           *string   `json:"icon,omitempty"`
	// DependsOn replaces the list when present; [] clears it. Omitted leaves it unchanged.
	DependsOn *[]string `json:"depends_on,omitempty"`
	// Contract replaces the contract when present; {} clears it. Omitted leaves it unchanged.
	Contract *core.NodeContract `json:"contract,omitempty"`
}

func (e *toolEnv) handleUpdateNode(ctx context.Context, _ *sdkmcp.CallToolRequest, in updateNodeIn) (*sdkmcp.CallToolResult, stageResponse, error) {
	ops, err := updateOps(in)
	if err != nil {
		return toolErr(err)
	}
	summary := fmt.Sprintf("update %s", in.NodeID)
	if in.NewID != "" {
		summary = fmt.Sprintf("update %s (rename to %s)", in.NodeID, in.NewID)
	}
	_, out, err := e.stage(ctx, in.apply(core.StageInput{
		Source:  core.OpUpdateNode,
		Summary: summary,
		Ops:     ops,
	}))
	return nil, out, err
}

type renameNodeIn struct {
	proposalMetaIn
	NodeID string `json:"node_id" jsonschema:"required"`
	NewID  string `json:"new_id" jsonschema:"required"`
}

func (e *toolEnv) handleRenameNode(ctx context.Context, _ *sdkmcp.CallToolRequest, in renameNodeIn) (*sdkmcp.CallToolResult, stageResponse, error) {
	_, out, err := e.stage(ctx, in.apply(core.StageInput{
		Source:  core.OpRenameNode,
		Summary: fmt.Sprintf("rename %s to %s", in.NodeID, in.NewID),
		Ops:     []core.StageOp{{Op: core.OpRenameNode, NodeID: core.NodeID(in.NodeID), NewID: core.NodeID(in.NewID)}},
	}))
	return nil, out, err
}

// updateOps converts an update_node request to staged operations: the field patch (when it
// changes anything) and, when new_id is set, a rename_node.
func updateOps(in updateNodeIn) ([]core.StageOp, error) {
	patch, err := updatePatch(in)
	if err != nil {
		return nil, err
	}
	var ops []core.StageOp
	if in.NewID == "" || !patch.IsEmpty() {
		ops = append(ops, core.StageOp{Op: core.OpUpdateNode, NodeID: core.NodeID(in.NodeID), Update: patch})
	}
	if in.NewID != "" {
		ops = append(ops, core.StageOp{Op: core.OpRenameNode, NodeID: core.NodeID(in.NodeID), NewID: core.NodeID(in.NewID)})
	}
	return ops, nil
}

// updatePatch validates an update_node request and converts it to a core patch.
func updatePatch(in updateNodeIn) (core.UpdateInput, error) {
	if in.ID != "" {
		return core.UpdateInput{}, fmt.Errorf("%w: id cannot be changed through update_node fields; use new_id (or rename_node) to rename a node, or name to change its display name", core.ErrBadOperation)
	}
	if in.Status != nil {
		return core.UpdateInput{}, fmt.Errorf("%w: update_node cannot set status; use update_node_status", core.ErrBadOperation)
	}
	patch := core.UpdateInput{
		Name:           in.Name,
		Type:           in.Type,
		Spec:           in.Spec,
		Implementation: in.Implementation,
		Scope:          in.Scope,
		Protected:      in.Protected,
		Risk:           in.Risk,
		Review:         in.Review,
		Verify:         in.Verify,
		Icon:           in.Icon,
		Contract:       in.Contract,
	}
	if in.DependsOn != nil {
		deps := nodeIDs(*in.DependsOn)
		if deps == nil {
			deps = []core.NodeID{}
		}
		patch.DependsOn = &deps
	}
	if in.ParentID != nil {
		p := core.NodeID(*in.ParentID)
		patch.Parent = &p
	}
	if in.RelationshipID != nil {
		r := core.RelID(*in.RelationshipID)
		patch.Relationship = &r
	}
	return patch, nil
}

type deleteNodeIn struct {
	proposalMetaIn
	NodeID string `json:"node_id" jsonschema:"required"`
}

func (e *toolEnv) handleDeleteNode(ctx context.Context, _ *sdkmcp.CallToolRequest, in deleteNodeIn) (*sdkmcp.CallToolResult, stageResponse, error) {
	_, out, err := e.stage(ctx, in.apply(core.StageInput{
		Source:  core.OpDeleteNode,
		Summary: fmt.Sprintf("delete %s", in.NodeID),
		Ops: []core.StageOp{{
			Op:     core.OpDeleteNode,
			NodeID: core.NodeID(in.NodeID),
		}},
	}))
	return nil, out, err
}

type setRelationshipIn struct {
	proposalMetaIn
	From           string `json:"from" jsonschema:"required"`
	To             string `json:"to" jsonschema:"required"`
	Label          string `json:"label,omitempty"`
	Kind           string `json:"kind,omitempty"`
	RelationshipID string `json:"relationship_id,omitempty"`
	// Contract omitted leaves an existing edge contract unchanged; {} clears it.
	Contract *core.EdgeContract `json:"contract,omitempty"`
}

func (e *toolEnv) handleSetRelationship(ctx context.Context, _ *sdkmcp.CallToolRequest, in setRelationshipIn) (*sdkmcp.CallToolResult, stageResponse, error) {
	_, out, err := e.stage(ctx, in.apply(core.StageInput{
		Source:  core.OpSetRelationship,
		Summary: fmt.Sprintf("relationship %s -> %s", in.From, in.To),
		Ops: []core.StageOp{{
			Op:       core.OpSetRelationship,
			From:     core.NodeID(in.From),
			To:       core.NodeID(in.To),
			Label:    in.Label,
			Kind:     in.Kind,
			RelID:    core.RelID(in.RelationshipID),
			Contract: in.Contract,
		}},
	}))
	return nil, out, err
}

type deleteRelationshipIn struct {
	proposalMetaIn
	RelationshipID string `json:"relationship_id" jsonschema:"required"`
}

func (e *toolEnv) handleDeleteRelationship(ctx context.Context, _ *sdkmcp.CallToolRequest, in deleteRelationshipIn) (*sdkmcp.CallToolResult, stageResponse, error) {
	_, out, err := e.stage(ctx, in.apply(core.StageInput{
		Source:  core.OpDeleteRelationship,
		Summary: fmt.Sprintf("delete relationship %s", in.RelationshipID),
		Ops: []core.StageOp{{
			Op:    core.OpDeleteRelationship,
			RelID: core.RelID(in.RelationshipID),
		}},
	}))
	return nil, out, err
}

func nodeIDs(in []string) []core.NodeID {
	if len(in) == 0 {
		return nil
	}
	out := make([]core.NodeID, len(in))
	for i, s := range in {
		out[i] = core.NodeID(s)
	}
	return out
}

type constraintIn struct {
	ID       string `json:"id" jsonschema:"required"`
	Text     string `json:"text" jsonschema:"required"`
	Severity string `json:"severity,omitempty"`
}

type setConstraintsIn struct {
	proposalMetaIn
	Constraints []constraintIn `json:"constraints" jsonschema:"required"`
}

func (e *toolEnv) handleSetConstraints(ctx context.Context, _ *sdkmcp.CallToolRequest, in setConstraintsIn) (*sdkmcp.CallToolResult, stageResponse, error) {
	list := make([]core.Constraint, 0, len(in.Constraints))
	for _, c := range in.Constraints {
		list = append(list, core.Constraint{ID: c.ID, Text: c.Text, Severity: c.Severity})
	}
	_, out, err := e.stage(ctx, in.apply(core.StageInput{
		Source:  core.OpSetConstraints,
		Summary: fmt.Sprintf("replace project constraints (%d)", len(list)),
		Ops:     []core.StageOp{{Op: core.OpSetConstraints, Constraints: list}},
	}))
	return nil, out, err
}
