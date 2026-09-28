# Stage 06 test plan

Tool behaviour is Go. The brief's effect on a harness is a written note, not an automated
test. Decision: [0004](../../../decisions/0004-testing-strategy.md).

## Release gate — task 06.6

One fixture model, one assignment, one file list that mixes an allowed path, a protected
path, and an unmapped path.

- `CheckScope` on that list and `ambit check` against a git diff that contains exactly
  those paths return the same allowed/violation verdict, the same node, and the same rule
  for each path.
- The MCP handler's result for `check_scope` is that same value, not a reformatted second
  opinion. Assert against the shared function's output.

## Other Go

- Each authoring tool, given a temp `.arch`, leaves `nodes/` unchanged and leaves one
  proposal directory whose `source` is that tool's name.
- Each authoring response has `applied: false` and a message that says the change is not
  applied.
- `create_node` with an `id` argument is rejected.
- `update_node` attempting to set `status` is rejected and points at `update_node_status`.
- `update_node_status` with `done` changes the node file and creates no proposal.
- `update_node_status` with a bad value writes nothing and lists the six statuses.
- Unknown `node_id` on `get_context`, `check_scope`, and `update_node_status` returns the
  id and does not suggest a neighbour.
- `get_context` output contains the assigned scope globs as literal strings, contains the
  protected sibling's path or name, and contains the instruction to call `check_scope`.
  This pins the required framing. It does not pin prose that task 06.7 is expected to tune;
  keep the assertion on the obligations, not on a full snapshot of the paragraph, unless a
  snapshot is the only practical way to stop the obligations being dropped.

## Manual — task 06.7

For each of Codex, Cursor, and Claude Code:

- Configure `ambit mcp` as a stdio server with `ambit start` not running. Confirm the tools
  list.
- `seed_model` or `create_node` against a scratch model. Confirm a proposal appears in the
  stage 04 UI and the canonical nodes did not change.
- `get_context` on a node whose sibling is `protected`. Read the brief. Note whether the
  agent then edits the protected path anyway.
- Record the result in `docs/decisions/`.

## Not in this stage

- A test that the brief reduces violations in the wild. That is what 06.7 observes, and it
  is not a pass/fail gate.
- Rate limiting `check_scope`.
