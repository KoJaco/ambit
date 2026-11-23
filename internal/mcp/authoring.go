package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/KoJaco/ambit/internal/core"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type seedModelIn struct {
	Transcript    string        `json:"transcript" jsonschema:"required"`
	ParentID      string        `json:"parent_id,omitempty"`
	Nodes         []seedNodeIn  `json:"nodes,omitempty"`
	Relationships []seedRelIn   `json:"relationships,omitempty"`
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
	Spec           string   `json:"spec,omitempty"`
	Icon           string   `json:"icon,omitempty"`
}

type seedRelIn struct {
	From           string `json:"from" jsonschema:"required"`
	To             string `json:"to" jsonschema:"required"`
	Label          string `json:"label,omitempty"`
	Kind           string `json:"kind,omitempty"`
	RelationshipID string `json:"relationship_id,omitempty"`
}

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
				Icon:           n.Icon,
			},
		})
	}
	for _, u := range in.Updates {
		patch, err := updatePatch(u)
		if err != nil {
			return toolErr(err)
		}
		ops = append(ops, core.StageOp{Op: core.OpUpdateNode, NodeID: core.NodeID(u.NodeID), Update: patch})
	}
	for _, r := range in.Relationships {
		ops = append(ops, core.StageOp{
			Op:    core.OpSetRelationship,
			From:  core.NodeID(r.From),
			To:    core.NodeID(r.To),
			Label: r.Label,
			Kind:  r.Kind,
			RelID: core.RelID(r.RelationshipID),
		})
	}
	for _, rid := range in.DeleteRelationships {
		ops = append(ops, core.StageOp{Op: core.OpDeleteRelationship, RelID: core.RelID(rid)})
	}
	for _, nid := range in.DeleteNodes {
		ops = append(ops, core.StageOp{Op: core.OpDeleteNode, NodeID: core.NodeID(nid)})
	}
	summary := fmt.Sprintf("%d operation(s) from transcript", len(ops))
	if len(in.Transcript) > 80 {
		summary += ": " + in.Transcript[:80] + "..."
	} else {
		summary += ": " + in.Transcript
	}
	_, out, err := e.stage(ctx, core.StageInput{
		Source:  core.SourceSeedModel,
		Summary: summary,
		Ops:     ops,
	})
	return nil, out, err
}

type createNodeIn struct {
	ID             string   `json:"id,omitempty"`
	Name           string   `json:"name" jsonschema:"required"`
	Type           string   `json:"type" jsonschema:"required"`
	ParentID       string   `json:"parent_id,omitempty"`
	RelationshipID string   `json:"relationship_id,omitempty"`
	Implementation []string `json:"implementation,omitempty"`
	Scope          []string `json:"scope,omitempty"`
	Protected      bool     `json:"protected,omitempty"`
	Spec           string   `json:"spec,omitempty"`
	Icon           string   `json:"icon,omitempty"`
}

func (e *toolEnv) handleCreateNode(ctx context.Context, _ *sdkmcp.CallToolRequest, in createNodeIn) (*sdkmcp.CallToolResult, stageResponse, error) {
	_, out, err := e.stage(ctx, core.StageInput{
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
				Icon:           in.Icon,
			},
		}},
	})
	return nil, out, err
}

type updateNodeIn struct {
	NodeID         string    `json:"node_id" jsonschema:"required"`
	ID             string    `json:"id,omitempty"`
	Status         *string   `json:"status,omitempty"`
	Name           *string   `json:"name,omitempty"`
	Type           *string   `json:"type,omitempty"`
	ParentID       *string   `json:"parent_id,omitempty"`
	RelationshipID *string   `json:"relationship_id,omitempty"`
	Implementation *[]string `json:"implementation,omitempty"`
	Scope          *[]string `json:"scope,omitempty"`
	Protected      *bool     `json:"protected,omitempty"`
	Spec           *string   `json:"spec,omitempty"`
	Icon           *string   `json:"icon,omitempty"`
}

func (e *toolEnv) handleUpdateNode(ctx context.Context, _ *sdkmcp.CallToolRequest, in updateNodeIn) (*sdkmcp.CallToolResult, stageResponse, error) {
	patch, err := updatePatch(in)
	if err != nil {
		return toolErr(err)
	}
	_, out, err := e.stage(ctx, core.StageInput{
		Source:  core.OpUpdateNode,
		Summary: fmt.Sprintf("update %s", in.NodeID),
		Ops: []core.StageOp{{
			Op:     core.OpUpdateNode,
			NodeID: core.NodeID(in.NodeID),
			Update: patch,
		}},
	})
	return nil, out, err
}

// updatePatch validates an update_node request and converts it to a core patch.
func updatePatch(in updateNodeIn) (core.UpdateInput, error) {
	if in.ID != "" {
		return core.UpdateInput{}, fmt.Errorf("%w: id is immutable; rename via name", core.ErrBadOperation)
	}
	if in.Status != nil {
		return core.UpdateInput{}, fmt.Errorf("%w: status must be set with update_node_status, not update_node", core.ErrBadOperation)
	}
	patch := core.UpdateInput{
		Name:           in.Name,
		Type:           in.Type,
		Spec:           in.Spec,
		Implementation: in.Implementation,
		Scope:          in.Scope,
		Protected:      in.Protected,
		Icon:           in.Icon,
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
	NodeID string `json:"node_id" jsonschema:"required"`
}

func (e *toolEnv) handleDeleteNode(ctx context.Context, _ *sdkmcp.CallToolRequest, in deleteNodeIn) (*sdkmcp.CallToolResult, stageResponse, error) {
	_, out, err := e.stage(ctx, core.StageInput{
		Source:  core.OpDeleteNode,
		Summary: fmt.Sprintf("delete %s", in.NodeID),
		Ops: []core.StageOp{{
			Op:     core.OpDeleteNode,
			NodeID: core.NodeID(in.NodeID),
		}},
	})
	return nil, out, err
}

type setRelationshipIn struct {
	From           string `json:"from" jsonschema:"required"`
	To             string `json:"to" jsonschema:"required"`
	Label          string `json:"label,omitempty"`
	Kind           string `json:"kind,omitempty"`
	RelationshipID string `json:"relationship_id,omitempty"`
}

func (e *toolEnv) handleSetRelationship(ctx context.Context, _ *sdkmcp.CallToolRequest, in setRelationshipIn) (*sdkmcp.CallToolResult, stageResponse, error) {
	_, out, err := e.stage(ctx, core.StageInput{
		Source:  core.OpSetRelationship,
		Summary: fmt.Sprintf("relationship %s -> %s", in.From, in.To),
		Ops: []core.StageOp{{
			Op:    core.OpSetRelationship,
			From:  core.NodeID(in.From),
			To:    core.NodeID(in.To),
			Label: in.Label,
			Kind:  in.Kind,
			RelID: core.RelID(in.RelationshipID),
		}},
	})
	return nil, out, err
}

type deleteRelationshipIn struct {
	RelationshipID string `json:"relationship_id" jsonschema:"required"`
}

func (e *toolEnv) handleDeleteRelationship(ctx context.Context, _ *sdkmcp.CallToolRequest, in deleteRelationshipIn) (*sdkmcp.CallToolResult, stageResponse, error) {
	_, out, err := e.stage(ctx, core.StageInput{
		Source:  core.OpDeleteRelationship,
		Summary: fmt.Sprintf("delete relationship %s", in.RelationshipID),
		Ops: []core.StageOp{{
			Op:    core.OpDeleteRelationship,
			RelID: core.RelID(in.RelationshipID),
		}},
	})
	return nil, out, err
}
