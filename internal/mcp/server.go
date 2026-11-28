// Package mcp is the stdio MCP server spawned by a coding harness.
package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/KoJaco/ambit/internal/core"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Run serves MCP over stdio for the model in dir. It loads .arch through core and
// watches the filesystem until ctx is cancelled or the client disconnects.
func Run(ctx context.Context, dir string) error {
	if dir == "" {
		dir = "."
	}
	idx, err := core.Open(dir)
	if err != nil {
		return err
	}
	if w := core.IgnoreWarning(dir); w != "" {
		fmt.Fprintln(os.Stderr, w)
	}
	watchCtx, cancelWatch := context.WithCancel(ctx)
	defer cancelWatch()
	if err := idx.WatchNotices(watchCtx, func(n core.WatchNotice) {
		if n.Err != nil {
			fmt.Fprintf(os.Stderr, "ambit mcp: watch reload: %v\n", n.Err)
		}
	}); err != nil {
		fmt.Fprintf(os.Stderr, "ambit mcp: warning: file watcher not started (model changes made outside this session will not be seen until restart): %v\n", err)
	}

	// Instructions are fixed for the life of the session (the SDK reads them at
	// initialize), so a later method change is only visible through get_started.
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "ambit", Version: "v1"},
		&sdkmcp.ServerOptions{Instructions: idx.ActiveMethod().Guidance})
	env := &toolEnv{idx: idx, dir: dir}
	registerTools(srv, env)
	return srv.Run(ctx, &sdkmcp.StdioTransport{})
}

type toolEnv struct {
	idx *core.Index
	dir string

	originOnce sync.Once
	origin     string
}

// originID identifies this MCP session as the staging caller. Only the session that staged
// a proposal may withdraw it or supersede it.
func (e *toolEnv) originID() string {
	e.originOnce.Do(func() {
		var b [8]byte
		if _, err := rand.Read(b[:]); err != nil {
			e.origin = fmt.Sprintf("mcp-%d", time.Now().UnixNano())
			return
		}
		e.origin = "mcp-" + hex.EncodeToString(b[:])
	})
	return e.origin
}

// readOnly marks tools that never change the model or the repository (ReadOnlyHint). run_verify
// and check_dependents run the node's committed verify commands, which can have side effects
// of their own, but they change nothing in the model.
var readOnly = &sdkmcp.ToolAnnotations{ReadOnlyHint: true}

func registerTools(srv *sdkmcp.Server, env *toolEnv) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "seed_model", Description: seedModelDescription}, env.handleSeedModel)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "create_node", Description: createNodeDescription}, env.handleCreateNode)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "update_node", Description: updateNodeDescription}, env.handleUpdateNode)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "rename_node", Description: renameNodeDescription}, env.handleRenameNode)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "withdraw_proposal", Description: withdrawProposalDescription}, env.handleWithdrawProposal)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "delete_node", Description: "Propose deleting a node (staged, not applied)."}, env.handleDeleteNode)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "set_relationship", Description: setRelationshipDescription}, env.handleSetRelationship)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "delete_relationship", Description: deleteRelationshipDescription}, env.handleDeleteRelationship)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "get_context", Annotations: readOnly, Description: "Return the imperative task brief for a node."}, env.handleGetContext)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "check_scope", Annotations: readOnly, Description: "Check whether files are in scope for a node."}, env.handleCheckScope)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "get_graph", Annotations: readOnly, Description: "Return the whole model compactly: nodes (id, name, type, parent, status, depends_on, has_contract), relationships (has_contract), and a summary of pending proposals. Contract bodies are left out unless include_contracts is true. Read this before authoring so you reuse ids and avoid duplicates."}, env.handleGetGraph)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "list_proposals", Annotations: readOnly, Description: "List pending proposals (id, source, summary, operations)."}, env.handleListProposals)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "check_done", Annotations: readOnly, Description: "One pass/fail answer to 'am I done?' for your node. Takes the files changed on your branch (committed, uncommitted and untracked, compared with the merge-base of main, or with base when you pass it, for example the integration branch your work started from), checks them against the node's scope, runs drift limited to your node, checks the node's declared deliverables, flags contracts that changed under you, lists the node's verify commands, and says which status you may set next under the completion policy: finish_status is needs_review (never done) when the node has milestones that are not all done (partial: true) or its risk is high, otherwise needs_review or done per the policy, or none. A scope glob that names a declared deliverable that does not exist yet is not an unmatched-glob failure; the missing deliverable is reported instead. Pass worktree if you work in a git worktree. It does not run tests: call run_verify for the verify commands, check_dependents for the nodes that depend on yours and check_contract_sync for the docs your node references. " + subagentGuidance}, env.handleCheckDone)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "get_proposal", Annotations: readOnly, Description: "Report what happened to a proposal by id: pending (with per-operation status), accepted, rejected, partial (some accepted, some rejected) or withdrawn. Finished proposals stay queryable (the 100 most recently resolved)."}, env.handleGetProposal)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "wait_for_proposal", Annotations: readOnly, Description: "Block until the architect has accepted or rejected every operation of a proposal you staged, or the timeout passes (default 60 s, max 300 s; some MCP clients give up on a call sooner, so pass a smaller timeout_s and call again), then return its state. Use it instead of asking the architect to reply 'accepted'. If timed_out is true (state pending, with a hint) the proposal is still pending: tell the architect and call it again."}, env.handleWaitForProposal)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "list_icons", Annotations: readOnly, Description: listIconsDescription}, env.handleListIcons)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "check_drift", Annotations: readOnly, Description: "Report model/code drift using the files git can see (tracked, plus untracked and not gitignored; tooling files such as go.mod are skipped via drift_ignore; add globs for other files no node should own to drift_ignore in .arch/config.json): files owned by no node, implementation or scope globs matching no files, and files owned by multiple non-nested nodes. Pass node_id to limit the report to one node and worktree (path of a git worktree of this repository) to check work that is not in the main checkout. unmatched_globs is reduced to unmatched_globs_count unless verbose_globs is true."}, env.handleCheckDrift)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "set_constraints", Description: setConstraintsDescription}, env.handleSetConstraints)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "get_build_order", Annotations: readOnly, Description: getBuildOrderDescription}, env.handleGetBuildOrder)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "update_node_status", Description: "Set a node status directly on the canonical model. Refused with a stage-gate error when the active method forbids that status in the current stage (design-first: assigned, in_progress, needs_review and done while the stage is design)."}, env.handleUpdateNodeStatus)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "get_started", Annotations: readOnly, Description: getStartedDescription}, env.handleGetStarted)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "check_model", Annotations: readOnly, Description: checkModelDescription}, env.handleCheckModel)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "check_deliverables", Annotations: readOnly, Description: checkDeliverablesDescription}, env.handleCheckDeliverables)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "run_verify", Annotations: readOnly, Description: runVerifyDescription}, env.handleRunVerify)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "check_dependents", Annotations: readOnly, Description: checkDependentsDescription}, env.handleCheckDependents)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "check_contract_sync", Annotations: readOnly, Description: checkContractSyncDescription}, env.handleCheckContractSync)
}
