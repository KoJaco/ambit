# Stage 03 — Architect canvas

## Status

Done. 2026-09-28. See [`workflow.md`](../../workflow.md) for the merge back to `main`.

## What this stage proves

The architect can open a model, move through it one drill-down level at a time, and edit a
node directly. Positions survive a reload. Nothing in this stage accepts agent-authored
structure; that is the next stage.

## Build-plan steps

Steps 4, 5, and 6. Ordering: [`v1-build-plan.md`](../../v1-build-plan.md).

## Depends on

[Stage 02](../02-enforcement/README.md), for the model, the mutations, and the shared ignore
check. The canvas does not call `ambit check`.

## In scope

- `ambit start`: localhost HTTP, drill-down reads, direct graph writes, layout cache,
  integrity warnings, SSE for model and integrity changes.
- Assignment written to `local.json` from the UI. The format already defines the field.
  The HTTP contract does not yet name the endpoint; adding that line to the contract is
  part of task 03.6, in the same change as the handler.
- The frontend hard reset. Inventory:
  [`frontend-refactor.md`](frontend-refactor.md).
- elkjs per drill-down level, cache read and write, drag-to-reposition.
- Vitest for layout-cache keying and the inspector's scope-glob preview.

Contract: [`local-http-api.md`](../../../contracts/local-http-api.md). Behaviour:
[`frontend.md`](../../../architecture/frontend.md).

## Out of scope

- Proposal endpoints, the `proposals-changed` SSE event, and the review UI.
  [Stage 04](../04-review-gate/README.md). The SSE client built here should ignore event
  kinds it does not handle, so stage 04 can add one.
- `go:embed` and SPA fallback on refresh. [Stage 07](../07-distribution/README.md). Until
  then, `ambit start` may serve the API alone and the SPA runs from the Vite dev server
  against it. Document the dev URL in the frontend README when it exists.
- MCP.
- Headless layout. elkjs stays in the browser. The Go side stores positions and does not
  compute them.
- Component tests and end-to-end tests.
- Loaders, actions, or any server work inside the SPA.

This stage is not incrementally shippable. After task 03.10 the frontend does not build
until 03.12 lands. That gap is accepted. Do not keep the port matrix alive to preserve a
green build.

## Release gate

None. The five release gates are elsewhere. This stage is done when its tasks are checked
and the manual pass in the test plan has been done once.

## Watch-items

- **Scope globs implemented twice.** Go owns enforcement. TypeScript owns the inspector
  preview. They are tested apart. A shared fixture is the candidate if they disagree; do
  not build the fixture in this stage unless a disagreement shows up.
  See [`open-questions.md`](../../open-questions.md).
- **Concurrent writes.** `ambit start` and, later, `ambit mcp` are separate processes with
  no cross-process lock. Do not add a lock file here. The thin spot is recorded in
  [ADR-0003](../../../adr/0003-runtime-and-distribution.md).
- **Layout cannot be computed headlessly.** Expected. Do not pull elk into Go.

## Links

- Tasks: [`tasks.md`](tasks.md)
- Tests: [`test-plan.md`](test-plan.md)
- Slice log: [`slice-log.md`](slice-log.md)
- Inventory: [`frontend-refactor.md`](frontend-refactor.md)
- Contract: [`local-http-api.md`](../../../contracts/local-http-api.md)
- Frontend: [`frontend.md`](../../../architecture/frontend.md)
- ADR: [0004](../../../adr/0004-frontend-platform.md)
