package mcp

import (
	"context"

	"github.com/KoJaco/ambit/internal/core"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type nodeCheckIn struct {
	NodeID   string `json:"node_id" jsonschema:"the node to check"`
	Worktree string `json:"worktree,omitempty" jsonschema:"path of the git worktree you work in; omit when you work in the main checkout"`
}

type runVerifyIn struct {
	NodeID   string `json:"node_id" jsonschema:"the node whose verify commands to run"`
	Worktree string `json:"worktree,omitempty" jsonschema:"path of the git worktree to run the commands in; omit for the main checkout"`
	TimeoutS int    `json:"timeout_s,omitempty" jsonschema:"per-command timeout in seconds (default 120, at most 600)"`
}

func (e *toolEnv) handleCheckDeliverables(_ context.Context, _ *sdkmcp.CallToolRequest, in nodeCheckIn) (*sdkmcp.CallToolResult, core.DeliverablesReport, error) {
	rep, err := core.CheckDeliverables(e.idx, core.DeliverablesOptions{Root: e.dir, FilesDir: in.Worktree, NodeID: core.NodeID(in.NodeID)})
	if err != nil {
		return nil, core.DeliverablesReport{}, err
	}
	return nil, rep, nil
}

func (e *toolEnv) handleRunVerify(ctx context.Context, _ *sdkmcp.CallToolRequest, in runVerifyIn) (*sdkmcp.CallToolResult, core.VerifyReport, error) {
	rep, err := core.RunVerify(ctx, e.idx, core.VerifyOptions{Root: e.dir, FilesDir: in.Worktree, NodeID: core.NodeID(in.NodeID), TimeoutS: in.TimeoutS})
	if err != nil {
		return nil, core.VerifyReport{}, err
	}
	return nil, rep, nil
}

func (e *toolEnv) handleCheckDependents(ctx context.Context, _ *sdkmcp.CallToolRequest, in runVerifyIn) (*sdkmcp.CallToolResult, core.DependentsReport, error) {
	rep, err := core.CheckDependents(ctx, e.idx, core.VerifyOptions{Root: e.dir, FilesDir: in.Worktree, NodeID: core.NodeID(in.NodeID), TimeoutS: in.TimeoutS})
	if err != nil {
		return nil, core.DependentsReport{}, err
	}
	return nil, rep, nil
}

func (e *toolEnv) handleCheckContractSync(_ context.Context, _ *sdkmcp.CallToolRequest, in nodeCheckIn) (*sdkmcp.CallToolResult, core.ContractSyncReport, error) {
	rep, err := core.CheckContractSync(e.idx, core.ContractSyncOptions{Root: e.dir, FilesDir: in.Worktree, NodeID: core.NodeID(in.NodeID)})
	if err != nil {
		return nil, core.ContractSyncReport{}, err
	}
	return nil, rep, nil
}
