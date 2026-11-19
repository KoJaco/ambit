# Decision note 0009 — Canvas presentation renderers

## Date

2026-09-29

## Summary

Optional node field `renderer` (`card`, `schema`, `placeholder`, `group`) and optional
`schema` rows. The architect sets them from the HTTP inspector and sidebar create form.
Relationship `kind` chooses the edge component. Hierarchy and relationships are unchanged.

## Decision

- A labeled group is a frame around **that node**, not a container of its children.
- Schema rows are visual. Edges do not attach to columns.
- MCP authoring does not set `renderer` or `schema` in this slice (same posture as `color`).
- Unknown renderer ids already on disk round-trip; new unknown ids are rejected on write.
- Absent `renderer` and unknown ids render as the standard card.

Note (decision 0010): `renderer: group` on a node that has children is no longer a static frame.
It is rendered as a summary card or an open container per
[decision 0010](0010-flat-first-groups-and-summary-nodes.md). The static frame remains for
`renderer: group` on a leaf.

## Out of scope

Expand-in-place, decision 0007 lift/embed, per-column edges, and AI-suggested groupings.

## References

- [arch-model-format.md](../contracts/arch-model-format.md)
- [0007](0007-drill-affordance-and-hierarchy-compression.md)
