# Frontend Refactor — Inventory

## Status

Frontend half of [stage 03](README.md). Not yet executed. Proposal review UI listed under
"New" is built in [stage 04](../04-review-gate/README.md).

## Date

2026-09-27

## Purpose

The file-by-file inventory for resetting [`frontend/`](../../../../frontend) from a pipeline
builder to the ambit canvas. The rationale for a hard reset over an incremental migration is
in [ADR-0004](../../../adr/0004-frontend-platform.md); the target behaviour is in
[`docs/architecture/frontend.md`](../../../architecture/frontend.md).

## Starting point

`frontend/` is a React Router v8 SPA cloned from `KoJaco/node-canvas-template`, holding
roughly 3,300 lines across `app/`. It is a partially-built **audio and LLM pipeline
builder**, not an architecture tool.

The mismatch is structural rather than cosmetic. Its domain model connects nodes through
typed ports validated by a compatibility matrix, on one flat canvas. ambit nests nodes,
drills into them, and has relationships carrying a label rather than a type that must
typecheck. `PortType` and `isPortCompatible` have no analogue in ambit.

The platform underneath is correct and stays: React Router v8 with `ssr: false`, Vite,
Tailwind v4, Node 24.

## Delete

### Pipeline domain model

- `app/components/node-config.ts` (103 lines) — `STTConfig`, `ChunkerConfig`,
  `EnrichTextConfig`, `DBWriteConfig` and friends. Entirely pipeline configuration.
- `app/components/node-registry.ts` (351 lines) — the `NodeKind` registry with ports,
  categories, and factory defaults per pipeline node kind.
- `app/components/ports.ts` (47 lines) — `PortMatrix` and `isPortCompatible`. **No ambit
  analogue exists.** Relationships in ambit are directed and labelled; there is nothing to
  typecheck.
- `app/components/palette-defaults.ts` (18 lines) — pipeline palette seeding.
- `app/components/constants.ts` (5 lines) — `chunk_strategy`, `redaction_policy`. Pipeline
  defaults.
- `app/components/utils.ts` (50 lines) — imports `Flow` and `PortMatrix`; every helper is
  port- or flow-shaped.

### Pipeline half of `app/components/types.ts` (135 lines)

Delete `NodeKind`, `SpecialKind`, `PortType`, `CanvasNode`, `CanvasConnection`, `Edge`,
`Flow`, `ChunkStrategy`.

**Keep** `ControlBarTool`, `SidebarSection`, and `SidebarItem` — these are UI vocabulary, not
domain, and `ControlBar.tsx`'s only domain import is `ControlBarTool`.

### Abandoned refactor skeleton

`app/components/canvas/` — 36 files, of which 34 are zero bytes, one
(`registries/groups.ts`) contains only a comment block, and one
(`_components/screen-size-alert.tsx`, 28 lines) is a more complete version of the top-level
stub.

Salvage `_components/screen-size-alert.tsx` before deleting the directory: the top-level
`ScreenSizeAlert.tsx` is a 5-line placeholder that renders the literal string
`ScreenSizeAlert`, so the `canvas/` copy is the real implementation and the top-level one is
the stub. This is the opposite of what the file layout suggests.

This directory is direct evidence that an incremental refactor of this codebase has been
started and abandoned once already — a considerable part of the argument for a hard reset.

### Template leftovers

- `app/welcome/` — `welcome.tsx` plus the React Router logo SVGs.
- `README.md` — the `create-react-router` template readme, still describing server-side
  rendering and Docker deployment.
- `Dockerfile` and `.dockerignore` — dead under `go:embed` distribution. Confirm genuinely
  unused before deleting.

## Keep as-is

- `app/root.tsx` (95 lines) — clean. Imports only `react-router`, `app.css`, and the theme
  provider. No domain coupling.
- `app/components/theme.tsx` (91 lines) and `app/components/ui/mode-toggle.tsx` — theme
  handling, domain-neutral.
- `app/app.css` (237 lines) — Tailwind v4 setup and design tokens.
- `app/components/ui-context.tsx` (45 lines) — UI state, domain-neutral.
- `app/components/Sidebar.module.css` (41 lines) — layout styling, reusable under a rebuilt
  sidebar.

## Keep and adapt

### `app/components/ControlBar.tsx` (203 lines)

**Cleanly reusable.** Its only domain import is the `ControlBarTool` union
(`grab | pointer | undo | redo | zoom-in | zoom-out | recenter`), which is canvas vocabulary
and applies unchanged. Expect near-zero change beyond an import path.

### `app/components/NodeCard.tsx` (66 lines)

Presentational. Rebuild the content around ambit's node fields — `name`, `type`, `status`,
and a `protected` indicator — and keep the card shell and styling.

## Rebuild, keeping layout only

Both of these import `NodeRegistry` and are coupled to the pipeline palette throughout.
Their *structure* is worth keeping; their contents are not.

### `app/components/Sidebar.tsx` (259 lines)

Currently a palette of pipeline node kinds grouped by category. ambit's sidebar is not a
palette of fixed kinds — node `type` is a free-form string with no enum, so there is nothing
to enumerate. It becomes navigation: the hierarchy tree, breadcrumbs into the current
drill-down level, and node creation. Keep the layout, the collapse behaviour, and
`Sidebar.module.css`.

### `app/components/node-inspector.tsx` (445 lines)

Currently renders per-kind pipeline config forms driven by `NodeRegistry`, and imports
`useReactFlow` from `reactflow` v11. Rebuild the fields around `name`, `type` (free text, not
a select), `status`, `implementation` globs, `scope` globs, the `protected` toggle, and the
markdown spec.

Two fields need deliberate treatment rather than plain rendering:

- **Empty `scope` must visibly communicate that it defaults to `implementation`**, not read
  as "no access".
- **`protected` must read as absolute** — it applies regardless of assignment, and the word
  suggests something conditional to anyone who has not read the enforcement model.

## Rewrite

### `app/components/FlowCanvas.tsx` (895 lines)

The largest single piece of work. Built around the pipeline model and the `reactflow` v11
API, with port handles, connection validation against `PortMatrix`, a hardcoded
`START_NODE_ID = "audio-in"`, and a per-kind icon switch spanning `STT`, `Chunker`,
`EnrichText`, and the rest.

Rebuild as the ambit canvas: one drill-down level at a time, nodes fed from a scoped API
query rather than filtered client-side, labelled directed relationships instead of typed
port connections, elkjs positions from the layout cache, and drill-in navigation to
`/node/:nodeId`.

### `app/routes.ts` and `app/routes/home.tsx`

Currently a single index route. Becomes `/` for the root model and `/node/:nodeId` for a
drill-down level. `home.tsx` also imports `ReactFlowProvider` from `reactflow` v11 and needs
the v12 import.

## New

- **API client** — typed wrapper over the local HTTP API.
- **SSE client** — `/events` subscription driving model, proposal, and integrity refreshes.
- **Layout integration** — elkjs per level, with cache read and write through the API.
- **Proposal review surface** — the largest new component. Per-node accept and reject, stale
  operations visually distinct, accept-all skipping stale operations. Built in stage 04;
  the stage 03 reset should leave a place for it and should not implement it.
- **Vitest setup** — pure logic only. Stage 03 covers layout cache keying and the
  inspector's scope preview. Proposal diffing is stage 04. No component or E2E tests in v1;
  see [decision note 0004](../../../decisions/0004-testing-strategy.md).

## Dependencies

### Remove

- `reactflow` v11 and `@reactflow/node-resizer` — **both v11 and `@xyflow/react` v12 are
  currently installed while the code imports v11.** v12 ships `NodeResizer` built in.
- `serve`, `isbot`, `@react-router/node` — the Go binary serves the bundle; there is no Node
  server.

### Add

- `elkjs`
- `vitest`

### Keep

`@xyflow/react` v12, `react` 19, `react-router` v8, `tailwindcss` v4, `lucide-react`,
`clsx`, `vite`, `typescript`.

## Sequencing note

This is **not incrementally shippable**. There is a period where the frontend does not
build, which is the accepted cost of the hard reset — there is no intermediate state in
which a port compatibility matrix is partially meaningful for a hierarchy model.

Sequenced after this stage's HTTP tasks so the rebuild targets a real API rather than a
mock that would have to be integrated against twice.

## Follow-ups

- [ ] Salvage `canvas/_components/screen-size-alert.tsx` before deleting `canvas/`.
- [ ] Confirm `Dockerfile` and `.dockerignore` are unused, then delete.
- [ ] Replace the template `README.md`.
- [ ] Verify nothing else imports the deleted modules after the reset.

## References

- ADR: [`docs/adr/0004-frontend-platform.md`](../../../adr/0004-frontend-platform.md)
- Target behaviour: [`docs/architecture/frontend.md`](../../../architecture/frontend.md)
- Contract: [`docs/contracts/local-http-api.md`](../../../contracts/local-http-api.md)
- Stage: [`README.md`](README.md)
- Build order: [`v1-build-plan.md`](../../v1-build-plan.md)
- Repo topology: [decision note 0001](../../../decisions/0001-repo-topology.md)
