# ADR-0004: Frontend Platform and Domain Reset

## Status

Accepted

## Date

2026-09-27

## Context

The original spec called for a Next.js app with static export, `go:embed`-ed into the Go
binary. The repository does not contain one. [`frontend/`](../../frontend) holds a React
Router v8 SPA, cloned from a `node-canvas-template` repository, and it is not an
architecture tool — it is a partially-built audio and LLM pipeline builder. Its domain model
is `NodeKind = STT | Chunker | EnrichText | Validator | ...`, with a typed port
compatibility matrix (`PortType`, `isPortCompatible`, `PortMatrix`) and per-kind
configuration types for speech-to-text providers, chunking triggers, and database writes.

The relevant question is therefore not "Next.js or React Router" in the abstract, but what
to do with roughly 3,300 lines of existing code whose plumbing is useful and whose domain
model is wrong in a way that cannot be renamed into correctness.

The mismatch is structural, not cosmetic. The pipeline builder connects nodes through typed
ports where compatibility is validated by a matrix, and nodes sit on one flat canvas. ambit
nests nodes, drills into them, and has relationships that carry a label rather than a type
that must typecheck. `PortType` has no analogue in ambit. Neither does `isPortCompatible`.

The repository also carries evidence of an abandoned prior refactor: of the 36 files under
`frontend/app/components/canvas/`, 34 are zero bytes, one contains only a comment block, and
one is a component that duplicates an existing top-level stub.

## Decision

### React Router SPA, not Next.js

Keep [`frontend/`](../../frontend) as a React Router v8 application in SPA mode.
[`frontend/react-router.config.ts`](../../frontend/react-router.config.ts) already sets
`ssr: false`, so `react-router build` already emits a static client bundle at
`build/client`, ready for `go:embed`.

No loaders, actions, or server-side rendering may do real work. All logic lives in Go.

### Hard reset of the domain layer, keep the chrome

Delete the pipeline domain model outright rather than migrating it: the pipeline half of
`app/components/types.ts`, plus `node-config.ts`, `node-registry.ts`, `ports.ts`,
`palette-defaults.ts`, `constants.ts`, `utils.ts`, and `app/welcome/`. Delete the abandoned
`app/components/canvas/` skeleton.

Keep and adapt the application shell and canvas chrome: `root.tsx`, `theme.tsx`, `app.css`,
`ui-context.tsx`, `mode-toggle.tsx`, and `ControlBar.tsx` — which turns out to be cleanly
domain-neutral, importing only the `ControlBarTool` union. The `Sidebar` and `node-inspector`
shells keep their layout and styling but have their contents rebuilt around `scope`,
`protected`, `status`, and `implementation`; both currently import `NodeRegistry` and are
coupled to the pipeline palette throughout.

The concrete file-by-file inventory is in
[`docs/planning/frontend-refactor.md`](../planning/frontend-refactor.md).

### Standardise on `@xyflow/react` v12

Both `reactflow` v11 and `@xyflow/react` v12 are currently installed, and
[`app/components/FlowCanvas.tsx`](../../frontend/app/components/FlowCanvas.tsx) imports the
v11 package. Drop `reactflow` and `@reactflow/node-resizer` — v12 ships `NodeResizer`
built in.

### Drill-down is a real route

`/node/:nodeId` renders that node's children; the root model is at `/`.

### Frontend testing is Vitest on pure logic only

Proposal diffing, scope glob matching in the inspector, layout cache keying. No component
tests and no end-to-end tests in v1.

## Alternatives Considered

- **Migrate to Next.js as the original spec specified** — rejected. The only property the
  spec actually needed from Next.js was a static client-side export, which the existing
  React Router SPA already produces. Migrating would mean rewriting routing, the build
  pipeline, and the Tailwind v4 integration to arrive at the same artifact, and would
  discard working chrome to do it. Next.js also makes it *easier* to accidentally introduce
  server-side work, which this architecture forbids, because its defaults assume a server
  exists.

- **Incrementally adapt `FlowCanvas.tsx` in place, renaming kinds and stripping ports
  gradually** — rejected. There is no intermediate state where a port compatibility matrix
  is partially meaningful for a hierarchy model; the concepts do not overlap. An incremental
  path would mean carrying dead abstractions through several passes and deleting them
  anyway, with the added risk that some survive by inertia and become load-bearing. The
  zero-byte `canvas/` directory is direct evidence that an incremental refactor of this
  codebase has already been started and abandoned once.

- **Empty `app/` entirely and rebuild from scratch** — rejected as throwing away work that
  is genuinely reusable. The theme handling, Tailwind v4 setup, control bar, screen-size
  guard, and the general inspector and sidebar layout are all domain-neutral and would be
  rebuilt near-identically. The domain model is the part that   is wrong, and the reset is
  scoped to exactly that. The reusable fraction is smaller than a first glance suggests —
  `ControlBar.tsx` survives nearly intact, but `Sidebar.tsx` and `node-inspector.tsx` keep
  only their layout — which narrows the gap between this option and a full rebuild without
  closing it.

- **Staying on `reactflow` v11** — rejected. v12 is the maintained line under the
  `@xyflow/react` name, it is already in the dependency tree, and it absorbs
  `@reactflow/node-resizer` as a built-in. Keeping two React Flow versions installed while
  importing the older one is a latent source of confusion.

- **Drill-down as component state with a single route** — rejected. It breaks browser back
  and forward, makes a level impossible to link to or bookmark, and discards a natural key
  for the layout cache that the router would otherwise hand over for free.

- **Query parameter (`/?at=node-id`)** — works, but a path segment reads better, nests
  naturally, and is the more conventional expression of navigating into a resource.

- **Playwright end-to-end tests over the drill-down and proposal review flows** — deferred.
  Valuable once those flows settle, but the UI shape is the least-settled part of v1 and
  E2E tests written against a moving target are maintenance cost without much signal.

- **No frontend tests at all** — rejected. Scope glob matching and proposal diffing are pure
  functions with real correctness stakes, and they are cheap to pin.

## Consequences

- **Pros**
  - No framework migration; the existing static-export posture is already correct.
  - The reusable chrome survives, so the reset costs far less than a rebuild.
  - Deleting the port matrix and pipeline configs removes concepts that would otherwise
    mislead every future reader of this codebase about what ambit is.
  - A single React Flow version removes an ambiguity in the dependency tree.
  - Routed drill-down gives back/forward, deep links, and a layout cache key for free.

- **Cons**
  - `FlowCanvas.tsx` is 895 lines built around the pipeline model and the v11 API. It is the
    largest single piece of work in the refactor and most of it changes.
  - The hard reset is a period where the frontend does not build or run, rather than a
    sequence of individually-shippable steps.
  - Vitest on pure logic only means the drill-down and proposal review flows have no
    automated coverage in v1, and regressions there will be found by hand.
  - React Router in SPA mode means being permanently disciplined about not using loaders and
    actions, which is the framework's idiomatic path. This needs to stay written down or it
    will be reintroduced by habit.

- **Follow-ups / TODOs**
  - The concrete delete/keep inventory is in
    [`docs/planning/frontend-refactor.md`](../planning/frontend-refactor.md).
  - Revisit Playwright coverage once the proposal review flow has stabilised.
  - `Dockerfile` and `.dockerignore` become dead under `go:embed` distribution; confirm and
    remove during the refactor.

## References

- Canonical spec: [`docs/planning/spec-v1.md`](../planning/spec-v1.md) Sections 5 and 7
- Architecture: [`docs/architecture/frontend.md`](../architecture/frontend.md)
- Plan: [`docs/planning/frontend-refactor.md`](../planning/frontend-refactor.md)
- Related: [decision note 0001](../decisions/0001-repo-topology.md),
  [decision note 0004](../decisions/0004-testing-strategy.md)
