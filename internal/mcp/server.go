// Package mcp is the stdio MCP server spawned by a coding harness.
package mcp

import (
	"context"
	"fmt"
	"os"

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
	go func() {
		_ = idx.WatchNotices(watchCtx, func(n core.WatchNotice) {
			if n.Err != nil {
				fmt.Fprintf(os.Stderr, "ambit mcp: watch reload: %v\n", n.Err)
			}
		})
	}()

	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "ambit", Version: "v1"}, nil)
	env := &toolEnv{idx: idx, dir: dir}
	registerTools(srv, env)
	return srv.Run(ctx, &sdkmcp.StdioTransport{})
}

type toolEnv struct {
	idx *core.Index
	dir string
}

func registerTools(srv *sdkmcp.Server, env *toolEnv) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "seed_model", Description: seedModelDescription}, env.handleSeedModel)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "create_node", Description: createNodeDescription}, env.handleCreateNode)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "update_node", Description: updateNodeDescription}, env.handleUpdateNode)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "delete_node", Description: "Propose deleting a node (staged, not applied)."}, env.handleDeleteNode)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "set_relationship", Description: setRelationshipDescription}, env.handleSetRelationship)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "delete_relationship", Description: deleteRelationshipDescription}, env.handleDeleteRelationship)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "get_context", Description: "Return the imperative task brief for a node."}, env.handleGetContext)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "check_scope", Description: "Check whether files are in scope for a node."}, env.handleCheckScope)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "get_graph", Description: "Return the whole model compactly: nodes (id, name, type, parent, status), relationships, and a summary of pending proposals. Read this before authoring so you reuse ids and avoid duplicates."}, env.handleGetGraph)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "list_proposals", Description: "List pending proposals (id, source, summary, operations)."}, env.handleListProposals)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "list_icons", Description: listIconsDescription}, env.handleListIcons)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "check_drift", Description: "Report model/code drift using git-tracked files: files owned by no node, implementation or scope globs matching no files, and files owned by multiple non-nested nodes."}, env.handleCheckDrift)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "update_node_status", Description: "Set a node status directly on the canonical model."}, env.handleUpdateNodeStatus)
}
