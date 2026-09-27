# Slice log — Canonical model

Completed implementation passes land here. One section per pass. Status stays in
[`tasks.md`](tasks.md) and the [checklist](../../checklist.md).

## 2026-09-27 — `.arch` read, write, and init

- **Landed:** `internal/core` reads and writes the node pair, `index.json`, `config.json`,
  and `local.json`, preserving unknown JSON keys. The in-memory index keeps hierarchy
  (`parent_id`) separate from relationships. Mutations validate cycles, references, and
  slug derivation. `ambit init` scaffolds `.arch/` and appends ignore entries idempotently.
  File watch rebuilds through the same `Open` path. `local.json` assignment keys `node_id`
  and `assigned_at` are named in the format contract.
- **Deferred:** Layout cache, proposals, `ambit check`, and the runtime `git check-ignore`
  warning. Those stay in later stages.
- **Verified:** `go test ./...` passes, including the `cmd/ambit` gate: real `ambit init`,
  real `git init`, real `git add -A`, with `local.json`, `.arch/.cache/`, and
  `.arch/.proposals/` absent from `git status --porcelain`. Covered init-before-git,
  git-before-init, and a pre-existing `.gitignore`.
