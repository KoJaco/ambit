# Decision Note 0011 — Distribution: embedded UI, pinned checksums, local release

## Status

Accepted for stage 08 (2026-11-20), pending review. Defaults chosen unattended; change on review.

## Context

Stage 08 ships one Go binary that serves the SPA and the API, plus an `npx ambit` shim that
fetches it. `go:embed` cannot reach `frontend/build`, a missing embed directory breaks
`go build`, and `cmd/ambit` tests build the binary, so a fresh clone must still compile and
test without a frontend build. The shim runs downloaded code, so it must not trust the host
that serves the binary.

## Decision

1. **Embed.** The `internal/webui` package embeds `dist/` (`//go:embed all:dist`), which holds
   a committed `.gitkeep`. `make webui` copies `frontend/build/client/*` into it (contents
   gitignored). `make build` depends on that and fails if `dist/index.html` is missing. Plain
   `go build`, `make dev-build` and `go test` always compile; tests that need the bundle skip
   with a message naming `make build`.
2. **Fallback.** A GET/HEAD under an API prefix that reaches the fallback is a JSON 404; an
   existing embedded file is served; any other path gets `index.html` (client routes such as
   `/node/:id`). If the bundle is absent, `/` returns a plain message naming `make build`.
   An explicit prefix list is used because some API paths (`GET /nodes`) have no GET route.
3. **Pinned checksums.** `npm/checksums.json` (sha256 per artifact) ships inside the npm
   package and is generated from the exact released binaries. The shim verifies before
   chmod, cache commit or exec. We rejected a checksum file next to the binary: a compromised
   release host could replace both. With pinning, an attacker needs the npm package too, and
   the download base can be mirrored (`AMBIT_DOWNLOAD_BASE`) without weakening verification.
   `bin/release/checksums.txt` is emitted for humans only.
4. **Artifacts.** `ambit_<version>_<os>_<arch>[.exe]` for darwin, linux, windows on amd64 and
   arm64, built `CGO_ENABLED=0 -trimpath`, hosted on GitHub Releases `KoJaco/ambit` at tag
   `v<version>`. Shim version == binary version (`ambit version`) == tag.
5. **No CI release workflow.** Product CI is out of scope (decision 0004). `scripts/release.sh`
   (`make release VERSION=x.y.z`) builds and pins; a human uploads and publishes
   ([runbook](../runbooks/release.md)). The script refuses to finish if any checksum is missing.
6. **npm name `ambit` is unchecked.** Fallback is a scoped name; see the runbook.

## Consequences

- A frontend change requires `make build` to reach the binary; a stale bundle is possible if
  `make webui` is skipped, so releases always run it.
- Each release rewrites two tracked placeholders (`npm/checksums.json`, `npm/package.json`).
- Releasing is manual and depends on the maintainer's discipline for upload-before-publish.
- Windows and macOS binaries are cross-built and unsigned; they have not been run on those
  platforms.

## References

- [Stage 08](../planning/stages/08-distribution/README.md), [release runbook](../runbooks/release.md)
- [`npm/README.md`](../../npm/README.md), [decision 0004](0004-testing-strategy.md)
