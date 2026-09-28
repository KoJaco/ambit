# Slice log — Agent interface

Completed implementation passes land here. One section per pass. Status stays in
[`tasks.md`](tasks.md) and the [checklist](../../checklist.md).

## 2026-09-28 — MCP stdio server and eight tools

- **Landed:** `ambit mcp` over stdio via `github.com/modelcontextprotocol/go-sdk`. Loads
  `.arch` with `core.Open`, prints `IgnoreWarning` to stderr, and runs `WatchNotices` for
  the process lifetime. Eight tools wrap `Index.Stage`, `CheckScope`, `Brief`, and
  `Update` without a second validation path. Authoring responses always include
  `applied: false` and state the change is not applied. `seed_model` accepts `transcript`,
  optional `parent_id`, and harness-supplied `nodes` / `relationships`. Staged
  `update_node` rejects `status`; core `applyStageUpdate` rejects status and supports
  `relationship_id`. Release gate in `cmd/ambit/mcp_test.go` compares `CheckScope`,
  `ambit check`, and `mcp.CheckScopeForTool`.
- **Deferred:** live Codex and Claude Code brief iteration (task 06.7). Initial record:
  [decision note 0005](../../../decisions/0005-get-context-brief-iteration.md).
- **Verified:** `go test ./...` passes, including MCP stdio tool list, authoring staging,
  brief obligations, and the 06.6 equivalence fixture.
