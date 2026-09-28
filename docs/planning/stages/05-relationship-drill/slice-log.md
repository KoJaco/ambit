# Slice log — Relationship drill

Completed implementation passes land here. One section per pass. Status stays in
[`tasks.md`](tasks.md) and the [checklist](../../checklist.md).

## 2026-09-28 — Relationship drill (stage 05)

- **Landed:** Stable relationship ids in `index.json`; create-without-id; update by id.
  Members use `relationship_id` on node files. `RelationshipLevel` query with endpoint
  context; cascade delete on relationship and endpoint removal. Proposals: `create_node`
  with `relationship_id`, staged `set_relationship` ids, per-operation accept. HTTP
  `GET /levels/relationships/{id}`, `DELETE /relationships/{id}`, layout key `_rel_{id}`,
  SSE `relationship_ids`. Client route `/relationship/:id`, drill on double-click edge or
  drillable crossing link, edge actions panel (member create, parallel edge, confirm
  delete), layout `ports` and `attachments` with flip-side control. Release gate in
  `relationship_drill_test.go`. Review detail shows op kind, relationship id, interior on
  member create.
- **Verified:** `go test ./...`, `npm test`, `npm run typecheck`.
