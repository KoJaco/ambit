# Slice log — Architect canvas

Completed implementation passes land here. One section per pass. Status stays in
[`tasks.md`](tasks.md) and the [checklist](../../checklist.md).

## 2026-09-28 — Local HTTP API

- **Landed:** `ambit start` listens on a loopback address (`127.0.0.1:8080` by default)
  and rejects any other host, including `0.0.0.0`. It loads `.arch` through `Open` and
  does not call `CheckScope`. `GET /levels` and `GET /levels/{nodeId}` return one
  drill-down. A relationship that leaves the level is a `crossings` entry, not another
  node. `GET /nodes/{id}` returns the structured fields and the markdown. `GET /integrity`
  and the level responses carry orphans and dangling references without failing the read.
  Create, update, delete, and set-relationship call the stage 01 functions; a cycle is 409
  with the core error text. `PUT` and `DELETE /assignment` write only `local.json`.
  Layout positions live under `.arch/.cache/layout/`, keyed by the node id or `_root` for
  `/`. Create, delete, and reparent delete that level's cache; a field edit does not.
  `GET /events` is a one-way stream of `model-changed` and `integrity-changed`.
  `proposals-changed` is not emitted. Before serving, `ambit start` calls `IgnoreWarning`.
  A warning does not refuse to serve.
- **Deferred:** proposal routes, `go:embed`, and the frontend reset. The existing frontend
  still builds.
- **Verified:** `go test ./...` passes, including the level-isolation, cycle, layout,
  assignment, loopback, and SSE cases. The existing frontend build passes.
