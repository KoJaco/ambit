# Stage 05 tasks

Working list. Status is mirrored in [`checklist.md`](../../checklist.md).

The tools are thin. The brief is not. Do not schedule 05.3 and 05.7 as an afternoon.

## 05.1 MCP process

- [ ]

**Touches:** `cmd/ambit`, `internal/mcp`.

**Done when:** `ambit mcp` speaks MCP over stdio and exits when the client closes. It loads
`.arch` through `ambit-core` and watches the files the same way stage 01 does. It does not
dial `ambit start`. Killing the HTTP server leaves this process running. An ineffective
gitignore produces the same loud warning as `ambit check`.

## 05.2 Authoring tools

- [ ]

**Touches:** `internal/mcp`, calling the stage 04 stage function.

**Done when:** `seed_model`, `create_node`, `update_node`, `delete_node`, and
`set_relationship` each write a proposal and return its id with `applied: false` and a
message that states the change has not been applied. None of them write under
`.arch/nodes/`. `seed_model` requires a transcript and accepts an optional `parent_id`.
`create_node` does not accept an `id`. `update_node` cannot change `id` or `status`.
Unknown node ids error and do not fuzzy-match. Rejections required at proposal time
(`delete_node` with children, cycles, missing relationship endpoints) surface as tool
errors from the stage function, not as a second implementation.

## 05.3 `get_context`

- [ ]

**Touches:** `internal/mcp`, `internal/core` for the brief assembly.

**Done when:** the tool returns a brief, not a JSON dump of the node, and the brief:

- lists the allowed globs from `scope`, using `implementation` when `scope` is empty
- names protected paths and sibling nodes as off-limits in words, rather than omitting them
- includes sibling interfaces and omits sibling implementation detail
- tells the agent to modify only the allowed globs, and to stop and report if the task
  needs more
- tells the agent to call `check_scope` during the task and before finishing

A missing node id errors and names the id.

## 05.4 `check_scope`

- [ ]

**Touches:** `internal/mcp`.

**Done when:** the tool passes `node_id` and `files` to the stage 02 `CheckScope` function
and returns each file as allowed or as a violation naming the node and the rule. It does
not read a git diff.

## 05.5 `update_node_status`

- [ ]

**Touches:** `internal/mcp`, `internal/core`.

**Done when:** the tool writes `status` on the canonical node immediately, using the stage
01 update path. Legal values are `draft`, `specified`, `assigned`, `in_progress`, `done`,
`blocked`. Anything else errors and lists those six. The tool does not create a proposal
and does not change any other field.

## 05.6 Release gate

- [ ]

**Touches:** a Go test that can invoke both the check command and `CheckScope`.

**Done when:** the equivalence case in [`test-plan.md`](test-plan.md) passes. The stage is
not done without it.

## 05.7 Brief iteration

- [ ]

**Touches:** the brief wording in core, and a new note under [`docs/decisions/`](../../../decisions/)
when the wording changes.

**Done when:** the brief has been run against Codex, Cursor, and Claude Code on a fixture
model with a protected sibling and a scope that is not identical to `implementation`. The
decision note records what the agent did, what wording changed, and what was left alone.
This task can stay open across more than one pass. The release gate does not wait on it,
and the stage README stays honest that the brief is still being tuned if this box is open.
The other tasks can be checked while this one continues.
