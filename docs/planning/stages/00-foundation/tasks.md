# Stage 00 tasks

Working list. Status is mirrored in [`checklist.md`](../../checklist.md).

## 00.1 One repository

**Touches:** repository root, `frontend/`.

**Done when:** `frontend/.git` is gone, the template remote is gone, and `git rev-parse
--show-toplevel` from `frontend/` is the project root.

- [x] Done. Commit `1c57da7`.

## 00.2 Root ignore file

**Touches:** `.gitignore`.

**Done when:** the root ignore file covers `frontend/node_modules/`, `frontend/build/`,
`frontend/.react-router/`, Go build output, and `.arch/local.json`, `.arch/.cache/`,
`.arch/.proposals/`.

- [x] Done.

## 00.3 Go module stubs

**Touches:** `go.mod`, `Makefile`, `cmd/ambit`, `internal/core`, `internal/httpapi`,
`internal/mcp`.

**Done when:** the module path is `github.com/KoJaco/ambit`, the four packages exist as
stubs, and `make test` and `make build` succeed. No product behaviour is required.

- [x] Done.
