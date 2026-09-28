# Product iteration notes

## Status

Live scratchpad while dogfooding ambit on the ambit repo model. Not release gates.

## Date

2026-09-28

## Stage reorder (planned)

After stage 06 merges to `main`:

| Stage | Branch (planned) | Content |
| --- | --- | --- |
| **07** | `stage/07-node-presentation` | Icon registry, node colour (see [07 README](stages/07-node-presentation/README.md)) |
| **08** | `stage/08-distribution` | Former stage 07: `go:embed`, npx shim, checksum gate |

Update [`workflow.md`](workflow.md) and [`checklist.md`](checklist.md) when stage 07 work starts; directory `stages/08-distribution/` replaces `07-distribution`. **2026-09-28:** planning docs updated (stage 07 presentation, stage 08 distribution).

## UI / model (from exploration)

- **Icon registry** — agents choose from a documented set; independent of node type.
- **Shape registry** — sparse closed set; **`standard`** = current NodeCard; optional `shape` on
  nodes; documented **suggested** pairings with types/icons (and maybe edge labels) are hints
  only, not validation ([decision 0006](../decisions/0006-node-icon-and-color.md),
  [shape sketch](stages/07-node-presentation/shape-registry-sketch.md)).
- **Colour** — optional; type-based defaults; colour picker in inspector (user-first);
  MCP colour TBD ([decision 0006](../decisions/0006-node-icon-and-color.md)).
- **Proposal review list** — separate actions cleanly; minimal list layout (architect-owned pass).
- **Proposal review detail** — clearer operation diff and stale/confirm UX (architect-owned pass).
- **Drill affordance** — show when a node has child nodes (hierarchy drill), not only relationship
  `drillable` crossings. Needs per-summary signal from API + canvas treatment ([decision 0007](../decisions/0007-drill-affordance-and-hierarchy-compression.md)).
- **Lift / embed (hierarchy compression)** — guided promote/demote of a subsystem: collapse siblings
  under a container or lift children up to reduce clutter and drill fatigue; batch via proposals;
  inverse operations paired ([decision 0007](../decisions/0007-drill-affordance-and-hierarchy-compression.md)).

## Meta-model

- Top layer under **Platform** matches [system-overview](../architecture/system-overview.md).
- Next passes drill **ambit-core** and **Architect Canvas** before leaf services.
- **2026-09-28:** staged proposals (accept in review panel): `p-20260928-1059-93c7` (6 under ambit-core), `p-20260928-1059-46ba` (4 under architect-canvas), `p-20260928-1059-1e1d` (4 under local-http-api), `p-20260928-1059-8575` (2 under agent-interface), `p-20260928-1059-78d1` (2 under enforcement-cli).
- **2026-09-28 (architect canvas explicit):** spec updates `p-20260928-1103-3de3` … `d297` (root + four modules); component tree `p-20260928-1103-6c50` (16 components).
- **2026-09-28 (canvas relationships):** `p-20260928-1106-48a6` — note node **Canvas data flow** + 33 cross-cutting edges (composition, `http-and-sse-client` fan-in, links to read/write/SSE/layout API modules).

## References

- [checklist.md](checklist.md)
- [open-questions.md](open-questions.md)
