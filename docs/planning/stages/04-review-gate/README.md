# Stage 04 — Review gate

## Status

Done. 2026-09-28. See [`workflow.md`](../../workflow.md) for the merge back to `main`.

## What this stage proves

Structural changes can sit uncommitted to the model until the architect accepts or rejects
them per node. A stale proposal cannot ride through on accept-all. This is provable before
any MCP tool exists.

## Build-plan steps

Step 7. Ordering: [`v1-build-plan.md`](../../v1-build-plan.md). This replaces the original
spec's LLM proxy step, which is cut.

## Depends on

[Stage 03](../03-architect-canvas/README.md), for the HTTP server, SSE, and a UI to hang
the review surface on. Staging and apply themselves depend only on stage 01's mutations;
the test helper does not need the canvas.

## In scope

- The proposal directory format in [`proposals.md`](../../../contracts/proposals.md).
- Staging, staleness, accept, reject, and atomic apply in `ambit-core`.
- A test helper that writes a proposal. Stage 05's tools call the same staging function.
- Proposal HTTP endpoints and the `proposals-changed` SSE event.
- The review UI.
- Vitest for proposal diffing.
- The apply-atomicity and staleness release gate.

## Out of scope

- The eight MCP tools. [Stage 05](../05-agent-interface/README.md). Do not add a temporary
  CLI that agents would be tempted to keep. Tests call the core function.
- Garbage collection of abandoned proposals. Nothing expires a proposal that still has
  `pending` operations.
- An LLM client, an instruction box, or regenerate.
- Applying a proposal by writing node files from the HTTP layer. Apply goes through the
  same mutations as a UI edit.

## Release gate

Task 04.8. Tests show that accepting an operation writes `.json`, `.md`, and the
`index.json` membership update together or not at all, and that a stale operation is not
applied without an explicit confirmation, including when accept-all runs.

## Watch-items

- **Per-node review at `seed_model` scale.** Thirty operations may train the architect to
  accept-all without reading. Build the summary and the per-node list the contract asks
  for, and do not collapse the gate to all-or-nothing. Whether the granularity holds is
  judged on the first real transcript, not in this stage.
  See [`open-questions.md`](../../open-questions.md).
- **Abandoned proposals.** Leave them. Age is visible because ids are time-ordered.

## Links

- Tasks: [`tasks.md`](tasks.md)
- Tests: [`test-plan.md`](test-plan.md)
- Slice log: [`slice-log.md`](slice-log.md)
- Contract: [`proposals.md`](../../../contracts/proposals.md)
- HTTP: [`local-http-api.md`](../../../contracts/local-http-api.md)
- ADR: [0002](../../../adr/0002-agent-interface-and-review-gate.md)
