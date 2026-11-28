package mcp

import (
	"context"
	"time"

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
	if err := e.idx.UpdateStatusGated(core.NodeID(in.NodeID), st); err != nil {
		return nil, updateStatusOut{}, err
	}
	return nil, updateStatusOut{NodeID: in.NodeID, Status: in.Status}, nil
}

type getGraphIn struct {
	IncludeContracts bool `json:"include_contracts,omitempty" jsonschema:"also return every node and relationship contract body (can be large)"`
}

type pendingProposal struct {
	ProposalID string `json:"proposal_id"`
	Source     string `json:"source,omitempty"`
	Summary    string `json:"summary,omitempty"`
	Operations int    `json:"operations"`
	Unreadable bool   `json:"unreadable,omitempty"`
}

type getGraphOut struct {
	Method        methodRef         `json:"method"`
	Stage         *stageRef         `json:"stage"`
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

func (e *toolEnv) handleGetGraph(_ context.Context, _ *sdkmcp.CallToolRequest, in getGraphIn) (*sdkmcp.CallToolResult, getGraphOut, error) {
	view := e.idx.GraphViewWith(in.IncludeContracts)
	m, err := e.idx.MethodByID(view.Method)
	if err != nil {
		m = e.idx.ActiveMethod()
	}
	pending, err := pendingSummaries(e.idx)
	if err != nil {
		return nil, getGraphOut{}, err
	}
	return nil, getGraphOut{
		Method: methodRefOf(m), Stage: stageRefOf(m, view.Stage),
		Nodes: view.Nodes, Relationships: view.Edges, Pending: pending,
	}, nil
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

type checkDoneIn struct {
	NodeID   string `json:"node_id" jsonschema:"the node you were assigned"`
	Worktree string `json:"worktree,omitempty" jsonschema:"path of the git worktree you work in; omit when you work in the main checkout"`
	Base     string `json:"base,omitempty" jsonschema:"git ref your branch started from; default is the merge-base with main or master"`
}

func (e *toolEnv) handleCheckDone(_ context.Context, _ *sdkmcp.CallToolRequest, in checkDoneIn) (*sdkmcp.CallToolResult, core.DoneReport, error) {
	rep, err := core.CheckDone(e.idx, core.DoneOptions{Root: e.dir, FilesDir: in.Worktree, NodeID: core.NodeID(in.NodeID), Base: in.Base})
	if err != nil {
		return nil, core.DoneReport{}, err
	}
	return nil, rep, nil
}

type getProposalIn struct {
	ProposalID string `json:"proposal_id" jsonschema:"id returned when the proposal was staged"`
}

func (e *toolEnv) handleGetProposal(_ context.Context, _ *sdkmcp.CallToolRequest, in getProposalIn) (*sdkmcp.CallToolResult, core.ProposalStateInfo, error) {
	st, err := e.idx.ProposalState(in.ProposalID)
	if err != nil {
		return nil, core.ProposalStateInfo{}, err
	}
	return nil, st, nil
}

const (
	defaultProposalWait = 60
	maxProposalWait     = 300
)

type waitForProposalIn struct {
	ProposalID string `json:"proposal_id" jsonschema:"id returned when the proposal was staged"`
	TimeoutS   int    `json:"timeout_s,omitempty" jsonschema:"seconds to wait for the architect (default 60, at most 300)"`
}

type waitForProposalOut struct {
	core.ProposalStateInfo
	// State repeats Status ("pending" when the wait timed out).
	State string `json:"state"`
	// TimedOut is true when the proposal was still pending after the wait.
	TimedOut bool `json:"timed_out"`
	// Hint is set when TimedOut: the call did not fail, the architect has not decided yet.
	Hint string `json:"hint,omitempty"`
}

const waitPendingHint = "still pending, call again"

func (e *toolEnv) handleWaitForProposal(ctx context.Context, _ *sdkmcp.CallToolRequest, in waitForProposalIn) (*sdkmcp.CallToolResult, waitForProposalOut, error) {
	secs := in.TimeoutS
	if secs <= 0 {
		secs = defaultProposalWait
	}
	if secs > maxProposalWait {
		secs = maxProposalWait
	}
	st, timedOut, err := e.idx.WaitForProposal(ctx, in.ProposalID, time.Duration(secs)*time.Second, 0)
	if err != nil {
		return nil, waitForProposalOut{}, err
	}
	out := waitForProposalOut{ProposalStateInfo: st, State: st.Status, TimedOut: timedOut}
	if timedOut {
		out.State = core.ProposalPending
		out.Hint = waitPendingHint
	}
	return nil, out, nil
}

type withdrawProposalIn struct {
	ProposalID string `json:"proposal_id" jsonschema:"id of a pending proposal you staged in this session"`
}

type withdrawProposalOut struct {
	ProposalID string `json:"proposal_id"`
	Status     string `json:"status"`
}

func (e *toolEnv) handleWithdrawProposal(_ context.Context, _ *sdkmcp.CallToolRequest, in withdrawProposalIn) (*sdkmcp.CallToolResult, withdrawProposalOut, error) {
	if err := e.idx.WithdrawProposal(in.ProposalID, e.originID()); err != nil {
		return nil, withdrawProposalOut{}, err
	}
	return nil, withdrawProposalOut{ProposalID: in.ProposalID, Status: core.ProposalWithdrawn}, nil
}

type listIconsIn struct{}

type listIconsOut struct {
	Icons []core.IconInfo `json:"icons"`
}

func (e *toolEnv) handleListIcons(_ context.Context, _ *sdkmcp.CallToolRequest, _ listIconsIn) (*sdkmcp.CallToolResult, listIconsOut, error) {
	return nil, listIconsOut{Icons: core.Icons()}, nil
}

type checkDriftIn struct {
	VerboseGlobs bool   `json:"verbose_globs,omitempty" jsonschema:"list every unmatched glob instead of only unmatched_globs_count"`
	NodeID       string `json:"node_id,omitempty" jsonschema:"limit the report to this node's globs and overlaps; omit for the whole model"`
	Worktree     string `json:"worktree,omitempty" jsonschema:"path of a git worktree of this repository to read files from, so an agent can verify its own work before merging"`
}

// driftOut is core.DriftReport as the MCP tools return it: the unmatched globs are a count
// unless the caller asks for them (a design-stage model has no files yet, so the list is
// usually long and uninformative).
type driftOut struct {
	Unowned        []string             `json:"unowned_files"`
	Unmatched      []core.UnmatchedGlob `json:"unmatched_globs,omitempty"`
	UnmatchedCount int                  `json:"unmatched_globs_count"`
	Overlaps       []core.Overlap       `json:"overlaps"`
	Ignored        int                  `json:"ignored_files"`
	Hint           string               `json:"hint,omitempty"`
}

func driftView(r core.DriftReport, verbose bool) driftOut {
	out := driftOut{Unowned: r.Unowned, UnmatchedCount: len(r.Unmatched), Overlaps: r.Overlaps, Ignored: r.Ignored, Hint: r.Hint}
	if verbose {
		out.Unmatched = r.Unmatched
	}
	return out
}

func (e *toolEnv) handleCheckDrift(_ context.Context, _ *sdkmcp.CallToolRequest, in checkDriftIn) (*sdkmcp.CallToolResult, driftOut, error) {
	report, err := core.DriftWith(e.idx, core.DriftOptions{Root: e.dir, FilesDir: in.Worktree, NodeID: core.NodeID(in.NodeID)})
	if err != nil {
		return nil, driftOut{}, err
	}
	return nil, driftView(report, in.VerboseGlobs), nil
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
