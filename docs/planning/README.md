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
- [`stages/`](stages/) — the seven capability stages.

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
| [02 Enforcement](stages/02-enforcement/README.md) | `ambit check` flags work outside the drawn boundary | 3 | Next |
| [03 Architect canvas](stages/03-architect-canvas/README.md) | The architect can see and edit one level at a time | 4–6 | Not started |
| [04 Review gate](stages/04-review-gate/README.md) | Agent-authored structure waits for accept or reject | 7 | Not started |
| [05 Agent interface](stages/05-agent-interface/README.md) | A harness can propose, brief, and report through MCP | 8 | Not started |
| [06 Distribution](stages/06-distribution/README.md) | One binary, installed without a toolchain | 9 | Not started |

## Current status

**Stage 01 done. Next is stage 02, enforcement.**

`ambit init` scaffolds a `.arch` model, and `internal/core` can load and mutate it.
The gitignore release gate passes. Cut `stage/02-enforcement` from `main` once this
branch has merged. See [`workflow.md`](workflow.md).

`Dockerfile` and `.dockerignore` are still in `frontend/` until stage 03 confirms they are
unused. See [decision note 0001](../decisions/0001-repo-topology.md).

## Latest slice log

[Stage 01](stages/01-canonical-model/slice-log.md). The gitignore release gate passed.
