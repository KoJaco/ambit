# ADR-0005: Relationship drill-down and stable relationship identity

## Status

Accepted

## Date

2026-09-28

## Context

The product separates hierarchy (`parent_id`) from cross-cutting relationships
([ADR-0003](0003-runtime-and-distribution.md)). Drill-down today follows only the tree: a
route opens a node's direct children and the relationships among those children. A
relationship is a directed edge with `from`, `to`, `label`, and `kind`. It has no stable id
and no interior. The mutation layer upserts on the `from`+`to` pair, so two distinct
interactions between the same services cannot coexist.

On real systems, an edge label such as "calls" or "persists to" often names a process the
architect should open: steps, handlers, or data flows that are not children of either
endpoint. Option 3 (point the edge at an ordinary node elsewhere in the tree) forces a fake
`parent_id` for that process. Option 2 (a first-class relationship with its own level)
matches how arbitrary systems are modeled without lying about containment.

## Decision

### Relationships are drill targets when they have an interior

- **Containment** stays `parent_id`. Opening a node shows what is inside that boundary.
- **Relationships** stay a separate adjacency structure. Opening a relationship shows how
  that interaction works: ordinary member nodes that belong to the relationship, laid out
  and cached like a node level, and not listed as children of either endpoint.
- **No interior** means the label is annotation only. There is no route and no empty canvas.

### Stable relationship identity

Each relationship has a stable id stored in `index.json`. Multiple directed relationships
may exist between the same `from` and `to` pair (different ids, labels, or kinds). Existing
models that store only `from`/`to`/`label`/`kind` load as one relationship each; migration
assigns ids at read or write time as defined in the arch-model contract.

`set_relationship` and proposal apply stop upserting solely on `from`+`to`. Create-or-update
is keyed by relationship id when present, or creates a new id when adding a second edge
between the same pair.

### Presentation: connection points

Which side of a node card an edge attaches to, and how many connection points a node has on
a level, is **presentation only**. Positions live in `.arch/.cache/layout/` with the rest
of elk and drag state. They are never written to canonical node or relationship records.
Changing a handle does not change `from`, `to`, `label`, or `kind`.

## Consequences

- **HTTP and routing** gain a level query and client route distinct from `/node/:nodeId`,
  keyed by relationship id. Layout cache keys follow the same pattern as node levels.
- **Proposals** gain operations (or extended `set_relationship` payloads) for relationship
  interiors and membership. Accept remains atomic through `ambit-core`.
- **MCP** (build step 9) must stage the new shape. It is not part of the relationship-drill
  stage itself.
- **ADR-0003** still holds: hierarchy and relationships are not merged into one adjacency
  list. The interior is a third container kind (members owned by the relationship), not a
  hierarchy edge.

## Alternatives considered

- **Pointer from edge to an existing node (option 3)** — rejected for arbitrary systems;
  see Context.
- **Model every process as a child of one endpoint** — rejected; misstates ownership and
  clutters service levels.
- **Canonical handle coordinates** — rejected; coordinates stay out of the model per
  [ADR-0001](0001-canonical-model-storage.md).

## References

- [arch-model-format.md](../contracts/arch-model-format.md) — amended in stage 05
- [local-http-api.md](../contracts/local-http-api.md) — amended in stage 05
- [frontend.md](../architecture/frontend.md) — amended in stage 05
- Stage: [05-relationship-drill](../planning/stages/05-relationship-drill/README.md)
