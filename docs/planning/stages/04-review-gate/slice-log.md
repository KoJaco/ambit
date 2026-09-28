# Slice log — Review gate

Completed implementation passes land here. One section per pass. Status stays in
[`tasks.md`](tasks.md) and the [checklist](../../checklist.md).

## 2026-09-28 — Proposal staging and atomic accept

- **Landed:** `Index.Stage` writes `.arch/.proposals/p-<YYYYMMDD>-<HHMM>-<4 hex>/`
  with manifest version 1 and materialised `.json` and `.md` for creates and updates.
  It does not write `.arch/nodes/`. `create_node` ids use the stage 01 slug rules and
  store `base_hash` null. Other operations store `sha256:` plus the hex digest of the
  length-prefixed on-disk JSON and markdown bytes. A version mismatch is unreadable.
  A missing `.md` is not applied. Staleness is computed on read. Accept copies the
  materialised pair and updates `index.json` as one unit, then runs the same validation
  as a direct edit; a failure restores the files and leaves the operation `pending`.
  `delete_node` removes the pair and the membership together. `set_relationship` calls
  the same mutation as a UI edit and does not overwrite node files. Accept-all skips
  stale operations. A stale hash requires `confirm_stale`. A missing node cannot be
  applied. The directory is removed when every operation is accepted or rejected.
  Accepting a create, reparent, or delete uses the existing layout invalidation.
- **Deferred:** proposal HTTP, `proposals-changed`, and the review UI.
- **Verified:** `go test ./internal/core/` passes, including the release-gate cases:
  injected failure after the json write, the markdown write, and the index write;
  stale accept writes nothing; a confirmed stale update that would cycle stays
  `pending`; accept-all reports the skipped operation; reject keeps a pending sibling;
  the last resolution removes the directory; a wrong `manifest_version` and a missing
  `.md` are not applied; `delete_node` of a parent fails at stage time with the child
  count and names.

## 2026-09-28 — Proposal HTTP and proposals-changed

- **Landed:** List, diff, accept, reject, accept-all, reject-all, and delete are on
  `/proposals`. A stale accept without `confirm_stale` is 409, names the node, and writes
  nothing. Unreadable proposals are listed and can be deleted. The watcher treats
  `.arch/.proposals/` separately from the model, and `/events` emits `proposals-changed`
  with `{}` when a proposal is staged, resolved, or deleted. The client still ignores
  unknown event names and refreshes proposal state on this one.
- **Deferred:** the review panel and the Vitest row mapping.
- **Verified:** `go test ./internal/httpapi/` passes, including the stale 409, accept-all
  skip list, unreadable delete, and two `proposals-changed` events around stage and accept.
  A node edit still does not emit `proposals-changed`.

## 2026-09-28 — Review UI, stage complete

- **Landed:** The review panel hangs on the home overlay, so `/` and `/node/:nodeId` both
  show it. Each proposal shows its summary, or its source and id, and one compact row per
  operation from the server diff. Stale rows are visually distinct. A hash mismatch offers
  "Apply over newer edit", which sends `confirm_stale`. A missing node has no accept
  control. Accept-all renders the skipped operations from the response. About thirty
  operations stay a summary plus a short row list. `proposalRows` copies `stale` from the
  payload and does not compare hashes. Vitest covers create, update, stale update, and
  `set_relationship`. Tasks 04.1–04.10 are checked. The release gate in 04.8 was already
  green, so this pass closes the stage.
- **Deferred:** MCP tools and proposal garbage collection. Relationship drill is
  [stage 05](../05-relationship-drill/README.md); MCP is stage 06.
- **Verified:** `npm test` and `npm run typecheck` pass. The manual pass was run once
  against Vite on `127.0.0.1:5173` with `ambit start` on a scratch model: the panel
  appeared through `proposals-changed` without a reload, a thirty-operation seed read as a
  summary and compact rows, an inspector save flipped the matching row to stale, accept-all
  showed `Skipped scratch (base_hash mismatch)`, confirming that row applied the proposed
  spec, and an accept plus a reject on a third still-pending operation survived a reload.
  The scratch model was deleted.
