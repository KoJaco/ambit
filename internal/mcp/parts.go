package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/KoJaco/ambit/internal/core"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const setPartDescription = "Propose adding or replacing one part of a partial model, or deleting it (staged, not applied; a human accepts it in the ambit UI). A part is {id (lowercase slug), name, include (up to 32 repo-relative globs, for example internal/billing/**), root_node_ids (the part's top-level nodes)}. In a partial-scope project (config.json scope: partial) the model describes only its parts: drift reports an unowned file only when it matches some part's include globs. Pass delete: true with only id to propose removing a part. The scope itself (whole or partial) is chosen by a human and no tool changes it. Read-only tools accept part to limit their answer to one part (get_graph, check_drift, check_model)."

type setPartIn struct {
	proposalMetaIn
	ID          string   `json:"id" jsonschema:"required,lowercase slug, unique among parts"`
	Name        string   `json:"name,omitempty"`
	Include     []string `json:"include,omitempty" jsonschema:"repo-relative doublestar globs (at most 32)"`
	RootNodeIDs []string `json:"root_node_ids,omitempty" jsonschema:"ids of the part's top-level nodes"`
	Delete      bool     `json:"delete,omitempty" jsonschema:"propose deleting the part with this id"`
}

func (e *toolEnv) handleSetPart(ctx context.Context, _ *sdkmcp.CallToolRequest, in setPartIn) (*sdkmcp.CallToolResult, stageResponse, error) {
	part := core.Part{ID: in.ID, Name: in.Name, Include: in.Include}
	for _, r := range in.RootNodeIDs {
		part.RootNodeIDs = append(part.RootNodeIDs, core.NodeID(r))
	}
	op := core.StageOp{Op: core.OpSetPart, Part: &part}
	summary := fmt.Sprintf("set part %s (%d include globs)", in.ID, len(in.Include))
	source := core.OpSetPart
	if in.Delete {
		op = core.StageOp{Op: core.OpDeletePart, Part: &core.Part{ID: in.ID}}
		summary = "delete part " + in.ID
		source = core.OpDeletePart
	}
	_, out, err := e.stage(ctx, in.apply(core.StageInput{Source: source, Summary: summary, Ops: []core.StageOp{op}}))
	return nil, out, err
}

const acceptProposalDescription = "Architect only. Accept a pending proposal that the project's auto-accept policy covers. Works only when a human turned on proposal_policy.auto_accept in config.json and the project stage is build; otherwise it refuses and says why. It refuses proposals staged by this same session (an agent cannot accept its own proposal). The policy accepts only low-risk operations: spec and contract text edits, dependency changes that remove dependencies or add acyclic ones, scope globs that match the node's own deliverables, new non-external nodes under an existing container, and edges between existing nodes without kind or cost implications. Anything touching a protected node, external nodes, stage, risk, review, human gates, shared or protected flags, deletes, renames, constraints or parts, or that would add an enforced lint finding, is never auto-accepted: the result then has accepted: false and lists the blocking operations with reasons, and a human decides in the ambit UI. Every auto-accept is written to an audit log the human reviews."

type acceptProposalIn struct {
	ProposalID string `json:"proposal_id" jsonschema:"required,id of a pending proposal staged by another session or by the UI"`
}

type acceptProposalOut struct {
	core.AutoAcceptResult
	// Message says in one line what happened.
	Message string `json:"message"`
}

func (e *toolEnv) handleAcceptProposal(_ context.Context, _ *sdkmcp.CallToolRequest, in acceptProposalIn) (*sdkmcp.CallToolResult, acceptProposalOut, error) {
	if e.role != RoleArchitect {
		// Defence in depth: the tool is only registered for the architect role.
		return nil, acceptProposalOut{}, errors.New("accept_proposal is only available when the server runs with --role architect")
	}
	res, err := e.idx.AutoAcceptProposal(in.ProposalID, e.originID())
	if err != nil {
		return nil, acceptProposalOut{}, err
	}
	out := acceptProposalOut{AutoAcceptResult: res}
	out.Message = acceptMessage(res)
	return nil, out, nil
}

// acceptMessage is the one-line outcome. A refusal after some operations applied says so.
func acceptMessage(res core.AutoAcceptResult) string {
	if res.Accepted {
		return fmt.Sprintf("Accepted proposal %s (%d operation(s)) under the auto-accept policy; recorded in the audit log.", res.ProposalID, len(res.Applied))
	}
	msg := fmt.Sprintf("Proposal %s was NOT accepted: %d operation(s) need a human decision in the ambit UI.", res.ProposalID, len(res.Blocking))
	if n := len(res.Applied); n > 0 {
		msg += fmt.Sprintf(" WARNING: %d operation(s) were already applied before the refusal (see applied) and are in the audit log; the rest stay pending.", n)
	}
	return msg
}
