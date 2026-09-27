# Planning Docs

This directory contains active implementation planning that is intended to guide real work.

## Conventions

- One canonical plan per major migration or initiative
- Dated slice logs for each completed implementation pass
- Plans should define target vocabulary, migration boundaries, and explicit deletes as well as additions

## Current canonical plan

- [`spec-v1.md`](spec-v1.md) — the canonical v1 spec. Defines the target vocabulary
  (`.arch`, node, drill-down level, proposal, scope, protected, assignment) and supersedes
  [`../archive/spec-v0-original.md`](../archive/spec-v0-original.md) in full. Its deltas
  from the original are enumerated at the top rather than left to be inferred.
- [`v1-build-plan.md`](v1-build-plan.md) — the ordered build path, step 0 through step 9,
  with the release gates that stop a step being declared done early.

Supporting:

- [`frontend-refactor.md`](frontend-refactor.md) — the file-by-file delete, keep, adapt, and
  rebuild inventory for [`frontend/`](../../frontend), including the explicit deletes.
- [`open-questions.md`](open-questions.md) — deliberately unresolved questions and known
  thin spots, with the trigger that should prompt each revisit.

## Current status

**Step 0 done. The project root is the git repository, on `main`, with no remote.**

`frontend/` is a normal subdirectory. Its template repository and `node-canvas-template`
remote are gone; the template-stripping edits were kept and committed as the tree. The Go
module is `github.com/KoJaco/ambit`, with stub packages in `cmd/ambit`, `internal/core`,
`internal/httpapi`, and `internal/mcp`. `make test` and `make build` succeed. No product
behaviour is implemented.

Next session plans the MVP build order and branches explicitly, starting from step 1
(`.arch` schema and `ambit init`). `Dockerfile` and `.dockerignore` are still in
`frontend/` until the refactor confirms they are unused. See
[decision note 0001](../decisions/0001-repo-topology.md).

## Latest slice log

None yet. The first slice log covers build step 1, which is complete only once the
`.gitignore` release-gate integration test passes — a real `ambit init`, a real `git init`,
a real `git add -A`, asserting that `local.json`, `.arch/.cache/`, and `.arch/.proposals/`
do not appear in `git status --porcelain`.

## Historical handoffs

None yet.
