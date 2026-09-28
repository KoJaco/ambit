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
