# Decision Note: Repository Topology

## Date

2026-09-27

## Summary

ambit becomes a single monorepo rooted at the project directory, holding `/docs`,
`/frontend`, `/cmd`, and `/internal`. The existing `frontend/.git`, which is a clone of the
`KoJaco/node-canvas-template` repository with the template remote still attached, is
absorbed into it and its history is not carried forward.

## Decision

- The project root is the one and only git repository.
- `frontend/.git` is removed and the frontend becomes a normal subdirectory.
- The `node-canvas-template` remote is dropped.
- History starts fresh at the root rather than being grafted from the template clone.

Layout:

```
/
├── docs/          # this documentation set
├── frontend/      # React Router SPA, go:embed-ed into the binary
├── cmd/           # ambit CLI entry points
└── internal/      # ambit-core and the front doors
```

## Rationale

- **The Go binary embeds the frontend build.** `go:embed` needs the static bundle present
  at build time. A split repo would mean either a submodule pointer to bump on every
  frontend change, or a build artifact committed into the Go repo. Both are friction on the
  most common change.
- **The spec, contracts, and code change together.** The format contract and the Go code
  implementing it should move in one commit and be reviewable as one diff. Splitting them
  across repos makes "did the implementation match the contract" a cross-repo question.
- **The template remote is actively wrong.** `frontend/` currently pushes to
  `node-canvas-template`. Leaving that attached risks pushing ambit work to a template
  repository.
- **The history has no value.** It is three commits: `Initial commit from create-react-router`,
  `will commit properly from now`, and `init project from laptop`. Preserving it would
  preserve a template's provenance, not ambit's.
- **Single-user project.** The coordination costs that sometimes justify splitting repos
  do not apply.

A wrinkle worth noting: ambit's own repository will eventually contain a `.arch/` directory
if ambit is used to model itself. That is fine and arguably a good dogfooding signal, but it
means the repo root hosts both the tool and a model built by the tool. Keep them distinct in
the reader's mind — `/docs` is the durable record, `.arch/` would be a working artifact.

## Impact

- The working tree in `frontend/` currently has uncommitted template-stripping changes
  (deleted `app/routes/action.set-theme.ts` and `app/services/sessions.server.tsx`, modified
  `root.tsx`, `routes.ts`, `package.json`, `tsconfig.json`, `react-router.config.ts`).
  These must be reconciled before or during the absorption — they are wanted changes, not
  drift to discard.
- `.gitignore` moves to the root and needs to cover `frontend/node_modules/`,
  `frontend/build/`, `frontend/.react-router/`, plus the Go and ambit entries.
- The frontend's existing `.dockerignore` and `Dockerfile` become dead under `go:embed`
  distribution and are removed as part of the refactor.
- Any existing clone or local checkout referencing `frontend/` as a repository stops being
  valid.

## Follow-ups

- [x] Reconcile the uncommitted `frontend/` changes before absorbing.
- [x] Remove `frontend/.git` and the template remote.
- [x] Write a root `.gitignore` covering Node, Go, and ambit paths.
- [x] `git init` at the root and make the first commit.
- [x] Confirm `Dockerfile` and `.dockerignore` are genuinely unused before deleting.

## References

- Related ADR: [`docs/adr/0004-frontend-platform.md`](../adr/0004-frontend-platform.md)
- Refactor plan: [`docs/planning/stages/03-architect-canvas/frontend-refactor.md`](../planning/stages/03-architect-canvas/frontend-refactor.md)
