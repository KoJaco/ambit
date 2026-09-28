# Stage 05 tasks

Working list. Status is mirrored in [`checklist.md`](../../checklist.md).

## 05.1 Identity

- [x]

**Touches:** `internal/core`, `index.json` schema in
[`arch-model-format.md`](../../../contracts/arch-model-format.md).

**Done when:** each relationship has a stable id in `index.json`. Multiple directed
relationships may exist between the same `from` and `to`. Existing from/to edges without
ids load as one relationship each. `set_relationship` no longer upserts solely on the pair.

## 05.2 Interior level

- [x]

**Touches:** `internal/core` query layer.

**Done when:** a level query keyed by relationship id returns member nodes owned by that
relationship, not by either endpoint's `parent_id`. Members do not appear on the parent
levels of `from` or `to`. An empty interior is not a level and is not drillable.

## 05.3 Proposals

- [x]

**Touches:** `internal/core` proposals, [`proposals.md`](../../../contracts/proposals.md).

**Done when:** staging an interior (create members, attach to the relationship, set or
update the edge) uses the stage 04 stage function. Accept applies atomically with the same
staleness and rollback rules as node edits.

## 05.4 HTTP and route

- [x]

**Touches:** `internal/httpapi`, [`local-http-api.md`](../../../contracts/local-http-api.md),
`frontend/` routes.

**Done when:** a relationship level endpoint exists. The client has a deep-linkable route
distinct from `/node/:nodeId`. Layout cache keys use the relationship id. SSE
`model-changed` covers interior edits.

## 05.5 Canvas drill

- [x]

**Touches:** `frontend/` canvas and navigation,
[`frontend.md`](../../../architecture/frontend.md).

**Done when:** clicking a label with an interior navigates into that level. Breadcrumb
returns to the level where the edge was drawn. A label with no interior is not navigable.

## 05.6 Release gate

- [x]

**Touches:** `internal/core` test per [`test-plan.md`](test-plan.md).

**Done when:** the release-gate case passes. The stage is not done without it.

## 05.7 Connection points (visual only)

- [x]

**Touches:** `frontend/` React Flow layer, layout cache format.

**Done when:** accepted edges can be dragged to the other side of a node. While dragging
from a handle, the bottom-left and bottom-right of a node show a `+` that adds another
connection point on that node. Positions persist in `.arch/.cache/layout/` only. `from`,
`to`, `label`, and `kind` are unchanged.

## 05.8 Review detail clarity

- [x]

**Touches:** `frontend/app/components/proposal-review.tsx`,
`frontend/app/proposal-detail.ts`.

**Done when:** on a staged proposal, an architect can see the operation, endpoints, spec
change, and stale state without reading raw diff JSON. Judged manually on a staged proposal.
No new review architecture.
