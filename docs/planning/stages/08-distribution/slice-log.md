# Slice log — Distribution

Completed implementation passes land here. One section per pass. Status stays in
[`tasks.md`](tasks.md) and the [checklist](../../checklist.md).

## 2026-11-20 — Stage 08 implementation (built, pending review)

- Embed package: `internal/webui` (`//go:embed all:dist`, committed `.gitkeep`); `make build`
  copies `frontend/build/client` into it and fails if `index.html` is missing.
- Shim location (task 08.4): `npm/` (package `ambit`, zero dependencies, Node >= 18,
  `node --test`). Pinned checksums in `npm/checksums.json`; the release gate (08.5) is in its tests.
- `ambit version` via `-ldflags -X main.version`. `make install` copies to `~/.local/bin`.
- Release tooling: `scripts/release.sh` / `make release VERSION=x.y.z`; manual upload and publish.
- Decisions: [0011](../../../decisions/0011-distribution-pinned-checksums.md). Runbooks:
  [release](../../../runbooks/release.md), [using ambit in your app](../../../runbooks/using-ambit-in-your-app.md).
- Not done: publishing, running binaries on macOS/Windows.  (npm name since resolved: @kojaco/ambit.)
