# Stage 07 tasks

Working list. Status is mirrored in [`checklist.md`](../../checklist.md).

## 08.1 Frontend build into the Go build

- [ ]

**Touches:** `frontend/` build script, `Makefile` or the equivalent at the repo root.

**Done when:** one command runs `react-router build` and then `go build`, and the Go build
fails if the client bundle directory is missing. The bundle is the static client output
(`ssr: false`). Dev workflow from stage 03 (Vite against `ambit start`) still works for
local UI work.

## 08.2 Embed and serve

- [ ]

**Touches:** `internal/httpapi`, `cmd/ambit`.

**Done when:** the client bundle is `go:embed`-ed and `ambit start` serves it on the same
localhost listener as the API. API paths are unchanged. No Node process is required to
serve a page.

## 08.3 Client-route fallback

- [ ]

**Touches:** `internal/httpapi`.

**Done when:** a request for `/node/<id>` with no matching API route returns the SPA entry
document, so a browser refresh on a drill-down URL renders the app. A missing API resource
(unknown node id on an API path) is still a 404 JSON body and is not rewritten into HTML.

## 08.4 npx shim

- [ ]

**Touches:** a small Node package at a path this task chooses and records in the slice log.
Keep it out of `frontend/`, which is the SPA.

**Done when:** `npx ambit` detects the current platform, downloads the matching binary if
the cache does not have it, and execs that binary so `init`, `start`, `mcp`, `check`, and
`hook install` are the same subcommands as the Go binary. The shim does not stay resident
as a server. A cache hit does not download again.

## 08.5 Release gate

- [ ]

**Touches:** the shim's tests.

**Done when:** a mismatched checksum refuses to exec and does not leave the bad binary as
the cached one. A matching checksum execs. The check runs before the first execution of a
newly downloaded file. Shipping documentation for the shim states that a release without
published checksums is not a release.
