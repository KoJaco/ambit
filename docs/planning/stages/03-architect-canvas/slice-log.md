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

## 2026-09-28 — Pipeline domain deleted

- **Landed:** The pipeline modules and `frontend/app/components/canvas/` are gone.
  `screen-size-alert.tsx` was copied over the top-level stub first. `types.ts` keeps
  `ControlBarTool`, `SidebarSection`, and `SidebarItem`. Nothing left imports the deleted
  modules.
- **Deferred:** The canvas rebuild. The frontend does not build: `npm run typecheck` fails
  because the remaining canvas, sidebar, and inspector still name the deleted pipeline
  symbols. `npm run build` still emits a bundle, because Vite does not typecheck.
- **Verified:** `npm run typecheck` exits 2. `go test ./...` still passes.

## 2026-09-28 — Canvas builds again

- **Landed:** `/` and `/node/:nodeId` render one level from `GET /levels`. Relationships
  are directed and labelled. A crossing is a marker on the on-screen node, not a node for
  `other_id`. An unknown `type` uses the same card and does not warn. Vite proxies the API
  to `127.0.0.1:8080`.
- **Deferred:** the inspector fields, hierarchy sidebar, elkjs, and the typed SSE client.
- **Verified:** the frontend builds again at `262b45d` (`npm run typecheck` and
  `npm run build`).

## 2026-09-28 — Canvas complete

- **Landed:** The inspector edits the node fields and says an empty scope defaults to
  implementation, and that protected applies regardless of assignment. The sidebar
  navigates the hierarchy and creates nodes. A typed client covers the endpoints this
  stage serves. The SSE client refetches on `model-changed` and `integrity-changed` and
  ignores any other event name. elkjs lays out a level in the browser when the cache is
  empty and writes it back; a cached level is rendered as stored. Dragging writes that
  cache. A structural change lays the level out again; a field edit does not. Vitest
  covers the layout key and the scope preview. `app/welcome/`, `frontend/Dockerfile`,
  and `frontend/.dockerignore` are gone. The frontend README describes Vite against
  `ambit start`.
- **Deferred:** proposal routes, emitting `proposals-changed`, `go:embed`, and the merge
  to `main`. Stage 04 stays uncut until that merge is requested.
- **Verified:** `npm test` and `npm run typecheck` pass. The manual pass was run once
  against Vite on `127.0.0.1:5173` with `ambit start` on `127.0.0.1:8080`: parent and
  child, back, reload of `/node/orders`, a drag that survived reload, a new child that
  laid the level out again, the inspector copy, assignment written only to `local.json`,
  an editor save reflected through SSE, and a rejected `0.0.0.0` bind.
