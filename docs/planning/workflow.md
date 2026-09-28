# Build workflow

## Status

Live. How v1 stages move through git.

## Date

2026-09-27

## Purpose

Stages 01 through 07 are a chain. Each one extends `ambit-core`, and a later stage assumes
the previous gate holds. The branch model follows that chain: one branch at a time, merged
to `main` when the stage is finished.

## `main`

`main` is the last finished stage. Stage 00 is already there. Product work for a stage
lands on that stage's branch, and reaches `main` when the stage is done.

A stage with a release gate is done when the gate test passes and the checklist boxes for
that stage are checked. A stage without a gate is done when its tasks are checked and, for
stage 03, the manual pass in its test plan has been done once. Task 05.7 is the exception
recorded below.

## One branch per stage

Branch from current `main` when the previous stage has merged:

```bash
git switch main
git switch -c stage/01-canonical-model
```

Names match the stage directories:

| Branch | Stage |
| --- | --- |
| `stage/01-canonical-model` | [01](stages/01-canonical-model/README.md) |
| `stage/02-enforcement` | [02](stages/02-enforcement/README.md) |
| `stage/03-architect-canvas` | [03](stages/03-architect-canvas/README.md) |
| `stage/04-review-gate` | [04](stages/04-review-gate/README.md) |
| `stage/05-relationship-drill` | [05](stages/05-relationship-drill/README.md) |
| `stage/06-agent-interface` | [06](stages/06-agent-interface/README.md) |
| `stage/07-distribution` | [07](stages/07-distribution/README.md) |

The next branch is cut from `main` after the merge. Leave stage 02 unstarted until stage 01
is on `main`. The model and the check are the part worth getting right slowly, and a branch
stacked on an unmerged predecessor will be rebased every time that schema moves.

Work in this checkout. A second worktree is for the one case in stage 03, below.

## Commits inside the stage

Commit on the stage branch as tasks land. When a task's status changes, update that stage's
`tasks.md` and [`checklist.md`](checklist.md) in the same commit.

Write the stage `slice-log.md` when a pass has actually shipped. For stage 01 the first
entry waits until task 01.9 passes. The slice log records what landed; the checkboxes
record status.

## Merging

Merge the stage branch to `main` when the stage is done, then push `main`. Delete the stage
branch after the merge. The following stage starts from that `main`.

## Stage 03, in two slices

Tasks 03.1–03.9 are the HTTP API. The existing frontend still builds at the end of that
slice. Merge that branch to `main` if you want a green checkpoint, then cut a fresh
`stage/03-architect-canvas` from that `main` for the reset.

Tasks 03.10–03.12 delete the pipeline domain and rebuild the canvas. The frontend does not
build between those tasks. Keep that work on the stage branch until it builds again.

During that gap, a second checkout of `main` is useful so `ambit check` still runs against
the last green tree:

```bash
git worktree add ../ambit-main main
```

That worktree tracks finished `main`. It is removed when the reset builds and the stage
merges. One extra checkout of `main`, for that window only.

## Task 06.7

The equivalence gate (task 06.6) can merge with the rest of stage 06 while brief iteration
stays open. After that merge, wording changes go on a short branch cut from `main`, and
what was learned goes in [`docs/decisions/`](../decisions/). The stage 06 slice log points
at that note.

## What stays sequential

Start a stage when its dependency is on `main`. Stage 04 needs stage 03's HTTP server.
Stage 05 needs stage 04's staging function and the canvas. Stage 06 needs stage 05's
relationship shape, stage 04's staging function, and stage 02's `CheckScope`. Stage 07
waits until the subcommands exist. Parallel stage branches would edit `internal/core` twice
and drift.

## References

- Ordering: [`v1-build-plan.md`](v1-build-plan.md)
- Status: [`checklist.md`](checklist.md)
- Stages: [`stages/`](stages/)
