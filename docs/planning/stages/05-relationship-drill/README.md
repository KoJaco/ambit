# Stage 05 — Relationship drill

## Status

Complete on branch `stage/05-relationship-drill`.

## What this stage proves

An architect can open a relationship that has an interior and see that interaction as its own
canvas level. Multiple distinct relationships can exist between the same two nodes. Connection
point placement on the canvas is presentation only and does not change the canonical model.

## Build-plan steps

Step 8. Ordering: [`v1-build-plan.md`](../../v1-build-plan.md).

## Depends on

[Stage 04](../04-review-gate/README.md) for proposals and the review UI, and
[stage 03](../03-architect-canvas/README.md) for level queries, routes, layout cache, and
the canvas.

## In scope

- Stable relationship ids and interiors, per [ADR-0005](../../../adr/0005-relationship-drill.md).
- Proposal staging and atomic accept for relationship interiors.
- HTTP level for a relationship id and a client route distinct from `/node/:nodeId`.
- Canvas drill on a label that has an interior; labels without an interior stay annotations.
- Connection-point UX: drag edges to the other side of a node; `+` zones to add handles
  (layout cache only).
- A visual clarity pass on review operation detail (existing drill-in UI).

Contracts amended in this stage:

- [`arch-model-format.md`](../../../contracts/arch-model-format.md)
- [`local-http-api.md`](../../../contracts/local-http-api.md)
- [`proposals.md`](../../../contracts/proposals.md)
- [`frontend.md`](../../../architecture/frontend.md)

## Out of scope

- MCP tools ([stage 06](../06-agent-interface/README.md) must stage the new relationship
  shape).
- Merging hierarchy and relationships into one adjacency list.
- Canonical storage of handle positions.
- A node type registry.

## Release gate

Task 05.6. One Go test: two relationships between the same pair both exist; a relationship
with members returns that level and those members are absent from both endpoints' parent
levels; a relationship with no members has no level and is not drillable.

## Watch-items

- **Migration of existing models** without ids on relationships. The contract must say how
  ids are assigned on load or first write.
- **Review UI** for interior membership must stay per-operation, not all-or-nothing.

## Links

- Tasks: [`tasks.md`](tasks.md)
- Tests: [`test-plan.md`](test-plan.md)
- Slice log: [`slice-log.md`](slice-log.md)
- ADR: [0005](../../../adr/0005-relationship-drill.md)
- Spec: [`spec-v1.md`](../../spec-v1.md) Section 13
