# Decision Note: get_context Brief Iteration (Stage 06)

## Date

2026-09-28

## Summary

Shipped the first imperative brief in `internal/core/brief.go`, exposed through the
`get_context` MCP tool. Go tests pin the required obligations (scope globs, protected paths,
`check_scope` instruction). MCP integration tests exercise the stdio server the way a
harness would (tool list, staging, direct writes, equivalence gate).

## Decision

Keep the brief as explicit prose sections: allowed globs, protected nodes with
implementation paths, sibling interfaces without implementation detail, stop-and-report
boundary, periodic `check_scope`, then the node specification.

List **all** protected nodes in the model as off-limits, not only siblings, so omission
cannot read as permission.

## Rationale

- Matches [mcp-tools.md](../contracts/mcp-tools.md) framing requirements and ADR-0002.
- Obligations are enforced in `brief_test.go` and `internal/mcp/tools_test.go` so refactors
  cannot drop them silently; full prose remains tunable in 06.7-style passes.
- Protected paths must be named explicitly per the contract (“omission is not prohibition”).

## Impact

- Assigned agents receive text from `get_context`, not a JSON node dump.
- Wording changes should update `brief.go` and this note when harness behaviour shifts.

## Harness notes (task 06.7)

Configure each coding harness with command `ambit mcp`, cwd at the repo root, with
`ambit start` **not** required.

| Harness | Verification |
| --- | --- |
| **Cursor** | MCP client tests in `cmd/ambit/mcp_test.go` and `internal/mcp/tools_test.go` connect over stdio to `ambit mcp` and list eight tools. |
| **Codex** | Same command; confirm tool list, run `create_node` on a scratch model, confirm proposal in review UI. |
| **Claude Code** | Same command; run `get_context` on a node with a protected sibling and confirm the brief names that sibling’s paths. |

Fixture shape used in tests: parent platform, assigned node with `scope` narrower than
`implementation`, protected sibling under the same parent (`brief_test.go`).

Initial observation: no wording change was required after automated obligation tests; live
Codex and Claude Code sessions should still be run on a real engagement and recorded here if
the brief is edited.

## Follow-ups

- [ ] Run the manual rows for Codex and Claude Code on a scratch repo and append what the
      agent did if wording changes.
- [ ] Track violation rate during client work ([open-questions.md](../planning/open-questions.md)).

## References

- Contract: [`docs/contracts/mcp-tools.md`](../contracts/mcp-tools.md)
- Stage: [`docs/planning/stages/06-agent-interface/README.md`](../planning/stages/06-agent-interface/README.md)
- Implementation: [`internal/core/brief.go`](../../internal/core/brief.go)
