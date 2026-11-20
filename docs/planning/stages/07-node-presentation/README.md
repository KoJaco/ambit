# Stage 07 — Node presentation

## Status

Built on `stage/07-node-presentation`, pending your browser review (see
[`stage-07-review-needed.md`](../../../handoffs/stage-07-review-needed.md)). Inserts before
distribution ([stage 08](../08-distribution/README.md)).

## What this stage proves

Nodes are readable at a glance on the canvas: optional **colour** (type- or user-driven),
**icons** and **shapes** from fixed registries (`standard` = today’s card), and a clear signal
when a node **has children** and can be drilled into (see
[decision 0007](../../../decisions/0007-drill-affordance-and-hierarchy-compression.md)).

**Flat-first canvas** ([decision 0010](../../../decisions/0010-flat-first-groups-and-summary-nodes.md),
amends 0007) also landed here: a node with children shows as an expanded group or a
collapsed summary card on its parent's level, relationships roll up to the visible anchors
(aggregated edges), and group, ungroup and reparent go through reviewed proposals. Collapse
state is view state, not model state. The Paper theme (warm palette, soft cards and edges)
shipped with it.

**Behaviour change:** parent levels now show roll-up edges they used to drop, and a
relationship fully inside a collapsed node is hidden and counted on its card instead of
appearing as a crossing.

## Depends on

[Stage 06](../06-agent-interface/README.md) on `main`. Canvas and inspector from stage 03.

## In scope

- A **closed icon registry** (IDs → SVG or icon component). MCP and HTTP document the allowed
  set; agents pick from the list via a new optional field (or tool argument), not free-form paths.
- A **closed shape registry** (IDs → NodeCard geometry). Sparse set to start; **`standard`**
  is the current rounded card when unset. Registry docs may list **suggested** shape↔type/icon
  (and optionally edge-label) pairings as hints only — not validation. Sketch:
  [`shape-registry-sketch.md`](shape-registry-sketch.md).
- **Optional node colour** on the canonical model, with sensible defaults derived from
  `type` where the architect has not overridden.
- **Inspector** pickers for colour and shape (architect-first). MCP may set colour only if we
  explicitly allow it in the contract; default posture is user-first, agent optional later.
- Icon, shape, and colour are independent; `type` may suggest default colour or suggested shape,
  not icon.

## Out of scope

- A full node **type registry** enum (still free-form strings per
  [decision 0002](../../../decisions/0002-node-type-taxonomy.md)). Stage 07 only adds
  presentation metadata, not type validation.
- Custom per-node uploaded icons or custom SVG shapes.
- Theme system redesign.

## Release gate

See [`test-plan.md`](test-plan.md). Go: registry round-trip, anchor roll-up, group/ungroup
proposals. Vitest: pure mapping and level-view logic. The visual pass is manual (not
Playwright) and is listed in the handoff review file.

## Links

- Tasks: [`tasks.md`](tasks.md)
- Decision: [0006](../../../decisions/0006-node-icon-and-color.md)
- Product iteration notes: [`../../product-iteration-notes.md`](../../product-iteration-notes.md)
