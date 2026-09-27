# Stage 01 — Canonical model

## Status

Not started. Next stage. Branch from `main` as `stage/01-canonical-model`. See
[`workflow.md`](../../workflow.md).

## What this stage proves

A valid `.arch` can be created from nothing and mutated without corrupting the hierarchy,
the relationship graph, or the node-file pair. Git will not commit machine-local or
unreviewed paths after `ambit init`.

## Build-plan steps

Steps 1 and 2. Ordering: [`v1-build-plan.md`](../../v1-build-plan.md).

## Depends on

[Stage 00](../00-foundation/README.md).

## In scope

- On-disk format in [`arch-model-format.md`](../../../contracts/arch-model-format.md):
  node `.json` and sibling `.md`, `index.json`, `config.json`, `local.json`.
- `ambit init`, including idempotent `.gitignore` entries.
- `ambit-core` read/write, the in-memory index, validated mutations, and file-watch rebuild.
- The release-gate integration test.

## Out of scope

- Layout cache storage. [Stage 03](../03-architect-canvas/README.md).
- Proposals. [Stage 04](../04-review-gate/README.md).
- `ambit check` and the runtime `git check-ignore` warning.
  [Stage 02](../02-enforcement/README.md) implements the warning; stage 03 calls it from
  `ambit start`. This stage writes the ignore entries and proves git honours them.
- Reserved fields `observed_implementation` and `agent_authority`. Unknown fields are
  preserved on round-trip and otherwise ignored. No code reads or writes the reserved names.
- An LLM client, a node-type registry, SQLite.

## Release gate

Task 01.9. The integration test:

1. Runs the full `ambit init` flow in a temp directory.
2. Runs `git init`.
3. Runs `git add -A`.
4. Asserts `local.json`, `.arch/.cache/`, and `.arch/.proposals/` do not appear in
   `git status --porcelain`.

A string assertion on `.gitignore` does not satisfy the gate. `init` before `git init`, a
pre-existing ignore file that merges oddly, or an entry edited out later would all pass a
content check and still fail the thing the gate exists to prevent: unreviewed proposals,
derived layout, or machine-local assignment entering a commit.

## Watch-items

- **Two files per node.** Create, delete, and rename must keep `.json` and `.md` together.
  An orphaned `.md` is a real state. Revisit the split only if orphan handling becomes a
  recurring bug. See [`open-questions.md`](../../open-questions.md).
- **Reserved fields.** Preserve unknown JSON fields. Do not start populating
  `observed_implementation` or `agent_authority`.

## Links

- Tasks: [`tasks.md`](tasks.md)
- Tests: [`test-plan.md`](test-plan.md)
- Slice log: [`slice-log.md`](slice-log.md)
- Contract: [`arch-model-format.md`](../../../contracts/arch-model-format.md)
- ADR: [0001](../../../adr/0001-canonical-model-storage.md)
- Spec: [`spec-v1.md`](../../spec-v1.md) Sections 2 and 3
