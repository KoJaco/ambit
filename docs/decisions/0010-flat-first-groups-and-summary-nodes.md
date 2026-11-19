# Decision Note 0010 — Flat-first canvas: groups, summary nodes, and edge roll-up

## Status

Accepted for stage 07 (2026-11-20). Amends [decision 0007](0007-drill-affordance-and-hierarchy-compression.md):
replaces its drill-affordance design (the `child_count` half already shipped in stage 07) and
reverses its "no virtual collapse" non-goal (reasons below). Builds on the `group` renderer
from [0009](0009-canvas-renderers.md), which becomes a real container.

## Context

Today every node with children is a separate level. You only see a node's children by
drilling into `/node/:id`, one level at a time. Dogfooding the ambit self-model showed two
problems:

- **Drill fatigue.** Small subsystems (2–6 children) get a level of their own, so the
  architect clicks in and out constantly to see what is really a single picture.
- **Hidden dependencies.** `Level()` only draws relationships whose endpoints are both direct
  children of the level. A relationship between two *grandchildren* in different subtrees does
  not show on the parent level at all, so a summarised level hides real coupling.

What we want instead: **build flat first.** Related nodes sit together on one canvas inside a
visual group. When a group, or the whole level, gets too large, it collapses into a single
**summary node** that you can drill into.

## Decision

### 1. One hierarchy, two ways to show it

`parent_id` stays the only hierarchy. Nothing about storage changes. A node with children is
drawn on its parent's level in one of two **view modes**:

| Mode | Rendering | Interaction |
| --- | --- | --- |
| **group** (expanded) | A labelled frame with its children laid out inside it, on the same canvas | Collapse toggle in the header; "Open" drills to `/node/:id` |
| **summary** (collapsed) | A single card showing a child count and a short type mix (e.g. `6 · 3 service, 2 store`) | Double-click drills; expand toggle |

A leaf node renders exactly as it does today.

The view mode is **view state, not model state**. It lives in the gitignored layout cache
(`.arch/.cache/`) next to positions, keyed per level. Enforcement, MCP and `ambit check`
never read it.

### 2. Default mode: flat until it gets crowded

When the architect has not set a mode for a node:

- Expand a node as a group if it has ≤ `group_max` children (default 8) **and** the level's
  visible node count stays ≤ `level_budget` (default 24).
- Otherwise show it as a summary.
- Expand groups inside groups at most 2 deep in the MVP.

If a level goes over budget because of the architect's own toggles, show a non-blocking hint
naming the largest groups to collapse. Never collapse anything automatically after the
architect has made a choice: ambit stays architect-first.

Both thresholds go in `config.json` with these defaults.

### 3. Edge roll-up through anchors

Let **V** be the set of visible nodes on the level (the level's children plus the members of
expanded groups). For any node *n*, `anchor(n)` is the nearest ancestor-or-self of *n* that
is in V, or none if *n* sits outside this level's subtree.

For each active relationship *(from, to)*:

- Both anchors exist and are different → draw an edge between the two anchors. If either
  endpoint was rolled up, add it to an **aggregated edge** for that anchor pair (label
  `3 relationships`, with tooltips for the kinds). Clicking an aggregated edge lists the
  underlying relationships, and any drillable ones keep their drill links.
- Both anchors are the same node → the relationship is internal to a collapsed node, so it is
  hidden and counted on the summary card.
- Exactly one anchor exists → it is a **crossing**, as today.

When nothing is expanded this does the same as today, plus the new roll-up edges that
`Level()` currently drops.

### 4. API and core

- `NodeView.ChildCount` already exists (stage 07). This note adds `InternalRelCount`,
  `TypeMix` and `GroupID`.
- `Level(parent, expanded []NodeID)` returns nested group membership, the anchor-resolved
  edges with an `aggregated` flag and member relationship ids, and crossings computed
  against V. Roll-up is computed **server-side** so the client never derives the model
  ([frontend.md](../architecture/frontend.md) invariant). `RelationshipLevel` shares the
  same anchor helper.
- HTTP: `GET /levels/{nodeId}?expand=a,b` (and `GET /levels?expand=...` for the root). The
  client sends the expanded set it read from the layout cache. Unknown ids, ids that are not
  children of a visible node, and ids deeper than 2 are ignored.
- The layout cache key includes a hash of the sorted expanded set, so collapsing restores the
  previous positions rather than re-laying everything out.

### 5. Layout and rendering

- elkjs compound layout (`elk.hierarchyHandling: INCLUDE_CHILDREN`), with group nodes as
  ELK parents.
- React Flow subflows: a group node type with children set to `parentId` and
  `extent: "parent"`.
- Dragging a node *within* its group writes the cache. Dragging a node *out of* its group is
  a reparent, so it goes through the gate as a **proposal**, never a silent write.

### 6. Group / ungroup (simplifies 07.8)

- **Group selection:** create a container plus `update_node parent_id` for each selected
  node, staged as one reviewed proposal. The new container starts expanded.
- **Ungroup:** move the children up to the container's parent and delete the container (or
  keep it as a leaf, if the architect chooses), also staged as one proposal.
- Relationships attach to node ids and not to levels, so reparenting never breaks an edge.
  Anchor roll-up simply re-derives where the edge is drawn.

### 7. Agent guidance

The MCP surface does not change. Authoring tool docs and the `get_context` framing add one
line: *"Prefer shallow models. Use a container node to group related nodes. Nest more than
two levels only for genuinely separate subsystems."*

## Why the 0007 "no virtual collapse" non-goal no longer applies

0007 rejected virtual collapse because hidden-but-present children could make enforcement and
MCP dishonest. Here collapse never changes `parent_id`, `implementation` or `scope`. It only
changes what one architect's canvas shows, and that state is gitignored. The canonical model
the agents and `ambit check` see is identical in both modes. Real structural changes (group,
ungroup, dragging out of a group) still go through proposals.

## Relationship to 0009

0009 defines `renderer: group` as a purely visual frame around a single node. Under this
note a node with children becomes a real container when expanded, and the existing field
keeps its meaning (the architect's choice of how a node looks). A `group` renderer on a leaf
still draws the static frame.

## Consequences

- Core: an anchor-resolution pass in `Level()` (O(relationships × depth), so cheap at v1
  model sizes), plus new tests for roll-up, same-anchor hiding, and crossings against V.
- Frontend: a group container node, a summary `NodeCard` variant, aggregated-edge rendering,
  compound ELK layout, and cache-key changes.
- Behaviour change: parent levels now show roll-up edges they used to drop. This is
  intended, and the release notes should mention it.
- Open: whether the view mode should *ever* become shareable (team defaults in canonical
  `config.json`). Not in v1.

## Follow-ups

- [ ] Slice into stage 07 tasks: (a) core anchor roll-up + tests, (b) group/summary
      rendering + toggle + compound layout + cache key, (c) group/ungroup proposals.
- [ ] Tune `group_max` / `level_budget` against the ambit self-model.
- [ ] Update `local-http-api.md` and `frontend.md`.

## References

- [0007 — drill affordance and hierarchy compression](0007-drill-affordance-and-hierarchy-compression.md)
- [0009 — canvas renderers](0009-canvas-renderers.md)
- [frontend.md](../architecture/frontend.md)
- [open-questions.md](../planning/open-questions.md) — drillable hierarchy, lift/embed
