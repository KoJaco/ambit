# Planning Docs

This directory contains active implementation planning that is intended to guide real work.

## Conventions

- One canonical plan per major migration or initiative. For the v1 build, that plan is
  [`v1-build-plan.md`](v1-build-plan.md) (ordering and gates) plus the stage directories
  under [`stages/`](stages/) (execution).
- [`spec-v1.md`](spec-v1.md) is the product spec. Stage docs link to it, the
  [contracts](../contracts/), and the [ADRs](../adr/). They do not restate them.
- [`checklist.md`](checklist.md) is the status rollup: one line per task, grouped by stage,
  plus the release gates and the v1 non-goals. Each stage's `tasks.md` is the working list,
  with packages to touch and a done-when line. When a task's status changes, update the
  stage file and the checklist in the same edit.
- Slice logs live in the stage directory (`slice-log.md`). They record what a finished pass
  actually shipped. They do not replace the checkboxes.
- [`open-questions.md`](open-questions.md) stays the list of things deliberately unresolved.
  Stage READMEs name the questions that can bite during that stage. Resolving one means
  moving it to the Resolved section there, not deleting it.
- [`workflow.md`](workflow.md) is how stages move through git: one branch per stage, merged
  to `main` when the stage is done.

Each stage directory holds:

- `README.md` — status, what the stage proves, which build-plan steps it nests,
  dependencies, scope, release gate, watch-items.
- `tasks.md` — ordered work items.
- `test-plan.md` — what is tested, and what stays manual.
- `slice-log.md` — completed passes.

## Current canonical plan

- [`spec-v1.md`](spec-v1.md) — the canonical v1 spec. Defines the target vocabulary
  (`.arch`, node, drill-down level, proposal, scope, protected, assignment) and supersedes
  [`../archive/spec-v0-original.md`](../archive/spec-v0-original.md) in full.
- [`v1-build-plan.md`](v1-build-plan.md) — why the work is ordered the way it is, and the
  release gates.
- [`checklist.md`](checklist.md) — every task and its status.
- [`workflow.md`](workflow.md) — branch, merge, and the stage 03 worktree window.
- [`stages/`](stages/) — the eight capability stages.

Supporting:

- [`open-questions.md`](open-questions.md) — deliberately unresolved questions and known
  thin spots, with the trigger that should prompt each revisit.
- The frontend reset inventory lives with the canvas stage:
  [`stages/03-architect-canvas/frontend-refactor.md`](stages/03-architect-canvas/frontend-refactor.md).

## Stages

| Stage | Proves | Build steps | Status |
| --- | --- | --- | --- |
| [00 Foundation](stages/00-foundation/README.md) | The repo can hold the tool | 0 | Done |
| [01 Canonical model](stages/01-canonical-model/README.md) | A valid `.arch` can be created and mutated | 1–2 | Done |
| [02 Enforcement](stages/02-enforcement/README.md) | `ambit check` flags work outside the drawn boundary | 3 | Done |
| [03 Architect canvas](stages/03-architect-canvas/README.md) | The architect can see and edit one level at a time | 4–6 | Done |
| [04 Review gate](stages/04-review-gate/README.md) | Agent-authored structure waits for accept or reject | 7 | Done |
| [05 Relationship drill](stages/05-relationship-drill/README.md) | Relationships open as their own canvas level | 8 | Done |
| [06 Agent interface](stages/06-agent-interface/README.md) | A harness can propose, brief, and report through MCP | 9 | In progress |
| [07 Node presentation](stages/07-node-presentation/README.md) | Icons and colour on the canvas | 10 | Planned |
| [08 Distribution](stages/08-distribution/README.md) | One binary, installed without a toolchain | 11 | Not started |

## Current status

**Stage 04 lands on `main` via PR. Cut `stage/05-relationship-drill` from `main` next.**

A proposal can be staged, listed, and accepted or rejected per operation. Accept-all skips
a stale operation, and confirming that operation applies it over the newer edit. The manual
pass was run once against the review panel. See [`workflow.md`](workflow.md).

Stage 03 confirmed nothing invokes `frontend/Dockerfile` or `frontend/.dockerignore` and
removed them. See [decision note 0001](../decisions/0001-repo-topology.md).

## Latest slice log

[Stage 04](stages/04-review-gate/slice-log.md). The apply-atomicity and staleness release gate passed.
