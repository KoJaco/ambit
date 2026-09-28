# Stage 06 — Agent interface

## Status

Not started.

## What this stage proves

A coding harness can propose model changes, fetch an imperative brief for an assigned node,
ask whether a file list is in scope, and mark a node done — without `ambit start` running,
and without an authoring tool writing straight into `.arch/nodes/`.

## Build-plan steps

Step 9. Ordering: [`v1-build-plan.md`](../../v1-build-plan.md).

## Depends on

[Stage 04](../04-review-gate/README.md) for the staging function,
[stage 05](../05-relationship-drill/README.md) for relationship identity and interiors
(MCP must stage the new shape), and [stage 02](../02-enforcement/README.md) for
`CheckScope`. The MCP process loads `.arch` itself. It does not proxy through the HTTP
server.

## In scope

- `ambit mcp` over stdio, the eight tools in [`mcp-tools.md`](../../../contracts/mcp-tools.md).
- `get_context` framing, and time to try that wording against Codex, Cursor, and Claude Code.
- The equivalence gate between `check_scope` and `ambit check`.

## Out of scope

- A new mutation path. Authoring tools call stage 04's stage function. `check_scope` calls
  stage 02's `CheckScope`. `update_node_status` calls the stage 01 field write.
- Any tool that calls a model provider.
- Freeform natural-language-to-architecture. `seed_model` takes a transcript and the
  harness does the generation.
- Fuzzy match on an unknown node id.
- Making `update_node_status` staged. Direct write of that one field is the contract.
- A cross-process lock. Both processes may write. The thin spot stays as recorded in
  [ADR-0003](../../../adr/0003-runtime-and-distribution.md).

## Release gate

Task 06.6. One test builds a fixture, runs `ambit check` against a diff that contains a
known file list, runs `check_scope` for the same assignment and the same list, and asserts
the verdicts match node-for-node and rule-for-rule.

## Watch-items

- **Whether the brief holds.** The required framing is specified. The violation rate across
  harnesses is not. Record wording changes in [`docs/decisions/`](../../../decisions/) with
  what was observed. Do not drop `ambit check` because a brief seemed to work.
  See [`open-questions.md`](../../open-questions.md).

## Links

- Tasks: [`tasks.md`](tasks.md)
- Tests: [`test-plan.md`](test-plan.md)
- Slice log: [`slice-log.md`](slice-log.md)
- Contract: [`mcp-tools.md`](../../../contracts/mcp-tools.md)
- ADR: [0002](../../../adr/0002-agent-interface-and-review-gate.md)
- Spec: [`spec-v1.md`](../../spec-v1.md) Section 8
