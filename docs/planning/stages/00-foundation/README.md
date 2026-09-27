# Stage 00 — Foundation

## Status

Done. 2026-09-27.

## What this stage proves

The repository can hold the tool: one history for docs, the SPA, and the Go module, before
any product behaviour exists.

## Build-plan steps

Step 0. Ordering: [`v1-build-plan.md`](../../v1-build-plan.md).

## Depends on

Nothing.

## In scope

- A single git repository at the project root.
- `frontend/` as a normal subdirectory, without the template remote.
- A root `.gitignore` covering Node, Go, and ambit working paths.
- The Go module and empty packages the later stages fill in.

## Out of scope

- Any `.arch` behaviour, CLI subcommand, HTTP server, or MCP server.
- Deleting `frontend/Dockerfile` and `frontend/.dockerignore`. That check belongs to
  [stage 03](../03-architect-canvas/README.md), with the rest of the template cleanup.

## Release gate

None.

## Watch-items

None left in this stage. The Dockerfile follow-up from
[decision note 0001](../../../decisions/0001-repo-topology.md) is stage 03 task 03.18.

## Links

- Tasks: [`tasks.md`](tasks.md)
- Tests: [`test-plan.md`](test-plan.md)
- Slice log: [`slice-log.md`](slice-log.md)
- Decision: [0001](../../../decisions/0001-repo-topology.md)
