# Decision note 0007 — Drill affordance and hierarchy compression

## Status

Proposed from product dogfooding (2026-09-28). Not scheduled to a stage gate yet.

## Context

The canvas shows one drill-down level at a time. Relationship crossings already expose
`drillable` links into relationship interiors. **Child drill** (double-click / navigate to
`/node/:id`) exists in the product but nodes do not visibly signal whether they have
children, so the architect cannot tell at a glance what is drillable vs a leaf.

Separately, deep models produce **drill fatigue**: many levels of real structure, but not
every level needs to be visited on every session. Architects want to **reduce clutter** on a
level and to **promote or demote** a small subsystem without hand-editing `parent_id` on many
nodes.

## Decisions (direction only)

### 1. Drill affordance on node cards

Nodes with one or more direct children (hierarchy) should show a **consistent, minimal**
indicator on the canvas (icon badge, chevron, or registry icon — likely aligned with
[stage 07](../planning/stages/07-node-presentation/README.md) presentation work).

Relationship drill already uses `drillable` on crossings; child drill should be equally
discoverable. The level API already returns `children` for the *current* level; the client
needs a per-node signal for *each child summary* (e.g. `has_children` or `child_count` from
core query) so cards on the parent level can render the affordance without fetching every
sub-level.

**Non-goal:** showing total descendant count or a preview tree on the card.

### 2. Hierarchy compression (“lift” / “embed”)

Introduce a **guided structure operation** (canvas and/or inspector), not a new storage
primitive, unless evidence says otherwise:

| User intent | Effect (conceptual) |
| --- | --- |
| **Embed / collapse down** | Selected siblings (or a selection + edges among them) become children of a **new or existing** container node on the same level; relationships among members are preserved within the new interior where possible. |
| **Lift / expand up** | Children of a container node **promote** to the container’s parent level; the container is removed or kept as a leaf, per explicit choice. |

The inverse pairing is intentional: same underlying mutations (`create_node`, `update_node`
`parent_id`, `set_relationship`) batched in one **reviewed proposal**, similar to
`seed_model` granularity.

**Open design points** (see [open-questions.md](../planning/open-questions.md)):

- Selection UX on React Flow (multi-select today is limited).
- How crossing relationships behave when endpoints move levels.
- Whether a container node gets auto `implementation`/`scope` union or stays presentation-only.
- Naming and spec for the new wrapper node on embed.

**Non-goals for v1 of the feature:**

- Automatic AI-suggested groupings.
- Hiding children while keeping them in the model without reparenting (virtual collapse);
  first version should change canonical hierarchy so enforcement and MCP stay honest.

## Consequences

- Likely **HTTP summary shape** + **NodeCard** change for drill affordance; stage 07 or a
  small follow-on slice after 07.
- Compression likely **stage 04-class** staging (multi-op proposal) with new UI entry points
  on the canvas; may land after stage 07 or as part of a “canvas ergonomics” pass.

## References

- [frontend.md](../architecture/frontend.md) — drill-down and crossings
- [product-iteration-notes.md](../planning/product-iteration-notes.md)
- Stage 07: [README](../planning/stages/07-node-presentation/README.md)
