# Slice log — Node presentation

## 2026-09-29 — Stage 07 implementation

- Core: optional `icon`, `shape`, `color` on nodes; registry validation on write; `child_count` on level summaries.
- HTTP PATCH and MCP `create_node` / `update_node` (`icon`, `shape` only).
- Canvas `NodeCard`, inspector presentation subviews, Vitest mapping tests.

## 2026-11-20 — Flat-first canvas and Paper theme

- Decision [0010](../../../decisions/0010-flat-first-groups-and-summary-nodes.md) amends 0007.
- Paper theme folded into the existing `--node-*` tones; all edge types restyled.
- Core: `Level(parent, expanded)` with anchor roll-up, aggregated edges, `InternalRelCount`,
  `TypeMix`, `?expand=`, `group_max` / `level_budget`.
- Frontend: container and summary rendering, compound ELK layout, level-view defaults and
  budget hint, expanded-set cache keys, view modes in localStorage, aggregated edge member list.
- Proposals: group, ungroup and reparent endpoints; drag-out and drag-in stage reparent.
- Review fixes: drag-out target on nested levels, duplicate group ids, stale hashed layouts,
  `StageGroup` panic on a dangling parent.
- Release note: parent levels now show roll-up edges they used to drop.
