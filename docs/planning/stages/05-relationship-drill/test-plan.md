# Stage 05 test plan

Go tests carry the release gate. Canvas drill, connection points, and review detail clarity
are manual. Decision: [0004](../../../decisions/0004-testing-strategy.md).

## Release gate — task 05.6

One fixture model in a temp directory:

- Create two relationships with the same `from` and `to` but different ids and labels.
  Both exist after reload.
- Add member nodes to one relationship. `Level` for that relationship id returns those
  members. `Level` for the parent of `from` and `Level` for the parent of `to` do not list
  those members as children.
- A relationship with no members: the relationship level query returns not drillable (404 or
  equivalent contract shape). The client must not register a route that implies an empty
  canvas.

## Proposals — task 05.3

- Stage a proposal that creates relationship members and accepts it. Members appear only on
  the relationship level. A partial accept failure leaves the operation `pending` and does
  not write a subset of members.

## Manual — tasks 05.5, 05.7, 05.8

**05.5:** On a model with at least one relationship interior, drill from an edge label into
the interior and back via breadcrumb. Confirm members are not visible on the service level.

**05.7:** Drag an accepted edge to the opposite side of a node. Add a connection point via
the `+` zone. Reload the page; positions persist. Confirm `index.json` relationship entries
are unchanged.

**05.8:** Open review detail for create, update, `set_relationship`, and a stale update.
Confirm operation type, endpoints, spec before/after, and stale styling are obvious.
