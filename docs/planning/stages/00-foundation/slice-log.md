# Slice log — Foundation

Completed implementation passes land here. Status stays in [`tasks.md`](tasks.md) and the
[checklist](../../checklist.md).

## 2026-09-27 — Monorepo scaffold

- **Landed:** Commit `1c57da7` (`Scaffold the ambit monorepo at the project root.`). The
  frontend template repository is absorbed; `frontend/` is a subdirectory. Root `.gitignore`
  covers Node, Go, and `.arch/local.json`, `.arch/.cache/`, `.arch/.proposals/`. Go module
  `github.com/KoJaco/ambit` with stubs in `cmd/ambit`, `internal/core`, `internal/httpapi`,
  and `internal/mcp`. `Makefile` targets `test` and `build`. `main` tracks
  `origin` at `github.com/KoJaco/ambit`.
- **Deferred:** `frontend/Dockerfile` and `frontend/.dockerignore` remain until stage 03
  confirms they are unused. No product behaviour.
- **Verified:** `make test` and `make build` succeed on the stub tree. Decision note
  [0001](../../../decisions/0001-repo-topology.md) follow-ups for the absorption are checked.
