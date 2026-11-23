package mcp

import (
	"context"

	"github.com/KoJaco/ambit/internal/core"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type getContextIn struct {
	NodeID string `json:"node_id" jsonschema:"required"`
}

type getContextOut struct {
	Brief string `json:"brief"`
}

func (e *toolEnv) handleGetContext(_ context.Context, _ *sdkmcp.CallToolRequest, in getContextIn) (*sdkmcp.CallToolResult, getContextOut, error) {
	text, err := e.idx.Brief(core.NodeID(in.NodeID))
	if err != nil {
		return nil, getContextOut{}, err
	}
	text = appendWarnings(text, e.idx)
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: text}},
	}, getContextOut{Brief: text}, nil
}

type checkScopeIn struct {
	NodeID string   `json:"node_id" jsonschema:"required"`
	Files  []string `json:"files" jsonschema:"required"`
}

type checkScopeHit struct {
	Rule   string `json:"rule,omitempty"`
	Node   string `json:"node,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type checkScopeFile struct {
	Path    string          `json:"path"`
	Allowed bool            `json:"allowed"`
	Rule    string          `json:"rule,omitempty"`
	Node    string          `json:"node,omitempty"`
	Detail  string          `json:"detail,omitempty"`
	Hits    []checkScopeHit `json:"hits,omitempty"`
}

type checkScopeOut struct {
	Results []checkScopeFile `json:"results"`
}

// CheckScopeForTool runs the same logic as the check_scope MCP tool.
func CheckScopeForTool(idx *core.Index, nodeID string, files []string) ([]core.FileResult, error) {
	return core.CheckScope(core.NodeID(nodeID), files, idx)
}

func fileResultsToMCP(results []core.FileResult) checkScopeOut {
	out := checkScopeOut{Results: make([]checkScopeFile, 0, len(results))}
	for _, r := range results {
		row := checkScopeFile{Path: r.Path, Allowed: r.Kind != core.KindViolation}
		if len(r.Hits) > 0 {
			row.Hits = make([]checkScopeHit, 0, len(r.Hits))
			for _, h := range r.Hits {
				row.Hits = append(row.Hits, checkScopeHit{
					Rule: h.Rule, Node: string(h.Node), Detail: h.Detail,
				})
			}
			h := r.Hits[0]
			row.Rule = h.Rule
			row.Node = string(h.Node)
			row.Detail = h.Detail
		}
		out.Results = append(out.Results, row)
	}
	return out
}

func (e *toolEnv) handleCheckScope(_ context.Context, _ *sdkmcp.CallToolRequest, in checkScopeIn) (*sdkmcp.CallToolResult, checkScopeOut, error) {
	if _, err := e.idx.Node(core.NodeID(in.NodeID)); err != nil {
		return nil, checkScopeOut{}, err
	}
	results, err := CheckScopeForTool(e.idx, in.NodeID, in.Files)
	if err != nil {
		return nil, checkScopeOut{}, err
	}
	out := fileResultsToMCP(results)
	return nil, out, nil
}

type updateStatusIn struct {
	NodeID string `json:"node_id" jsonschema:"required"`
	Status string `json:"status" jsonschema:"required"`
}

type updateStatusOut struct {
	NodeID string `json:"node_id"`
	Status string `json:"status"`
}

func (e *toolEnv) handleUpdateNodeStatus(_ context.Context, _ *sdkmcp.CallToolRequest, in updateStatusIn) (*sdkmcp.CallToolResult, updateStatusOut, error) {
	st := core.Status(in.Status)
	if err := e.idx.Update(core.NodeID(in.NodeID), core.UpdateInput{Status: &st}); err != nil {
		return nil, updateStatusOut{}, err
	}
	return nil, updateStatusOut{NodeID: in.NodeID, Status: in.Status}, nil
}

type getGraphIn struct{}

type pendingProposal struct {
	ProposalID string `json:"proposal_id"`
	Source     string `json:"source,omitempty"`
	Summary    string `json:"summary,omitempty"`
	Operations int    `json:"operations"`
	Unreadable bool   `json:"unreadable,omitempty"`
}

type getGraphOut struct {
	Nodes         []core.GraphNode  `json:"nodes"`
	Relationships []core.GraphEdge  `json:"relationships"`
	Pending       []pendingProposal `json:"pending_proposals"`
}

func pendingSummaries(idx *core.Index) ([]pendingProposal, error) {
	list, err := idx.ListProposals()
	if err != nil {
		return nil, err
	}
	out := make([]pendingProposal, 0, len(list))
	for _, p := range list {
		out = append(out, pendingProposal{
			ProposalID: p.ID, Source: p.Source, Summary: p.Summary,
			Operations: len(p.Operations), Unreadable: p.Unreadable,
		})
	}
	return out, nil
}

func (e *toolEnv) handleGetGraph(_ context.Context, _ *sdkmcp.CallToolRequest, _ getGraphIn) (*sdkmcp.CallToolResult, getGraphOut, error) {
	nodes, edges := e.idx.Graph()
	pending, err := pendingSummaries(e.idx)
	if err != nil {
		return nil, getGraphOut{}, err
	}
	return nil, getGraphOut{Nodes: nodes, Relationships: edges, Pending: pending}, nil
}

type listProposalsIn struct{}

type listProposalsOut struct {
	Proposals []core.ProposalSummary `json:"proposals"`
}

func (e *toolEnv) handleListProposals(_ context.Context, _ *sdkmcp.CallToolRequest, _ listProposalsIn) (*sdkmcp.CallToolResult, listProposalsOut, error) {
	list, err := e.idx.ListProposals()
	if err != nil {
		return nil, listProposalsOut{}, err
	}
	if list == nil {
		list = []core.ProposalSummary{}
	}
	return nil, listProposalsOut{Proposals: list}, nil
}

type listIconsIn struct{}

type listIconsOut struct {
	Icons []core.IconInfo `json:"icons"`
}

func (e *toolEnv) handleListIcons(_ context.Context, _ *sdkmcp.CallToolRequest, _ listIconsIn) (*sdkmcp.CallToolResult, listIconsOut, error) {
	return nil, listIconsOut{Icons: core.Icons()}, nil
}

type checkDriftIn struct{}

func (e *toolEnv) handleCheckDrift(_ context.Context, _ *sdkmcp.CallToolRequest, _ checkDriftIn) (*sdkmcp.CallToolResult, core.DriftReport, error) {
	report, err := core.Drift(e.idx, e.dir)
	if err != nil {
		return nil, core.DriftReport{}, err
	}
	return nil, report, nil
}

type getBuildOrderIn struct {
	ScopeNodeID string `json:"scope_node_id,omitempty"`
}

func (e *toolEnv) handleGetBuildOrder(_ context.Context, _ *sdkmcp.CallToolRequest, in getBuildOrderIn) (*sdkmcp.CallToolResult, core.BuildOrderResult, error) {
	res, err := core.BuildOrder(e.idx, core.NodeID(in.ScopeNodeID))
	if err != nil {
		return nil, core.BuildOrderResult{}, err
	}
	return nil, res, nil
}
