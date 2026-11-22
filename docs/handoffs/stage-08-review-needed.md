# Stage 08 review queue (distribution) + MVP sign-off checklist

Written for you to review alongside your design pass. Companion: `stage-07-review-needed.md`.
Branch: `stage/07-node-presentation`. **Nothing is pushed, tagged, published, uploaded or merged.**

## Where the MVP stands

| Stage | State |
| --- | --- |
| 00-05 | Done (merged earlier) |
| 06 | Built. **06.7 is open**: iterate the brief against Codex/Cursor/Claude Code (needs you to run them) |
| 07 | Built (Paper theme, flat-first groups, roll-up, group/ungroup proposals). Needs your design pass and sign-off |
| 08 | Built (this doc). Needs the manual release steps below only when you want to publish |
| Gap | Assignment UI (03.6) was never built; added in the follow-up listed below |

After you sign off the design pass, the remaining human steps are: 06.7, merge the branch, and (optionally)
publish. You can use ambit in your own apps **today** without publishing: `make install`.

## Do this first

1. `make install` (Go + Node needed). Check `ambit version`, then in one of your app repos:
   `ambit init` and `ambit start`, open http://127.0.0.1:8080.
   Full walkthrough: `docs/runbooks/using-ambit-in-your-app.md`.
2. Look at the embedded UI (no Vite) and your design pass against it. I verified it loads, but not by eye.
3. Answer the decisions below.

## Decisions I made for you (change freely)

| Decision | Default chosen | Why / where |
| --- | --- | --- |
| Embed location | `internal/webui` (`go:embed all:dist`, dist contents gitignored, `.gitkeep` committed) | `go:embed` cannot reach `frontend/build`; fresh clones still `go build`/`go test` |
| Plain `go build` without a bundle | Succeeds; UI routes return 503 "run make build". `make build` fails without a bundle | matches the stage test plan |
| SPA fallback | GET/HEAD only; API prefixes (`/levels /nodes /relationships /assignment /layout /integrity /proposals /events`) never return HTML | `httpapi.APIPrefixes` in `internal/httpapi/ui.go`; a test fails if a route prefix is missing |
| Version | `ambit version`, set by `-ldflags -X main.version` | needed by the shim and release |
| Platforms | darwin, linux, windows x amd64, arm64 | cross-built, only linux/amd64 was run |
| Release host | GitHub Releases `KoJaco/ambit`, tag `v<version>` | **you** upload |
| Checksums | **Pinned inside the npm package** (`npm/checksums.json`), sha256, verified before chmod/rename/exec | a checksum file next to the binary on the same host would not protect against a compromised host. ADR 0011 |
| npm package name | `@kojaco/ambit` | **resolved:** unscoped `ambit` is taken (date parser); publishing as `@kojaco/ambit` (see decision 0011) |
| License | none set | no `LICENSE` file exists; pick one before publishing |
| CI | `.github/workflows/ci.yml` (added after stage 08; see `docs/runbooks/ci.md`) | release is still the local `make release VERSION=x.y.z` |
| Fonts | Self-hosted Inter (variable, no opsz axis, no italics) + JetBrains Mono | the embedded UI makes zero third-party requests; +311 KiB woff2, browser fetches latin only |
| Package manager | npm (not pnpm) | `frontend/package-lock.json` is tracked; I removed the stray `pnpm-lock.yaml` |
| AGENTS.md | Replaced only the stale Commands section and the first Architecture-map bullet | the rest is byte-identical; CLAUDE.md untouched |

## Verified (by me, in this environment)

- `go build/vet`, `go test ./... -race -count=1`, `cd npm && node --test` (18/18), frontend `tsc` + vitest all pass.
- `make build` for real: 12 MB binary, `ambit version`, `ui: embedded`.
- Embedded server with **no Vite** on a copy of your Example model: `/`, `/node/<id>`, `/relationship/x` -> HTML (no-cache);
  `/assets/*.js` -> JS, immutable; `/favicon.ico` ok; `/nodes/zzz` and `/levels/zzz` -> JSON 404; `GET /nodes` -> JSON 404
  (not HTML); `POST /node/x` -> 405.
- Browser (Playwright): direct load of `/node/example-boundary` (= hard refresh) renders 23 nodes / 28 edges, only host
  contacted is the local server, Inter loaded locally, **0 console errors**.
- `scripts/release.sh 0.0.0-smoke`: 6 binaries + `checksums.txt` + `npm/checksums.json`; shim against a local http server:
  good checksum runs `ambit version`, second run is a cache hit (one request in the log), a tampered binary exits 1 with
  "not trustworthy" and leaves **nothing cached**. Placeholders and output were cleaned up afterwards.

## Not verified

- darwin and windows binaries were cross-built only, never run. The shim was only tested on Linux.
- `shellcheck` is not installed; `scripts/release.sh` was only checked with `bash -n` and a real run.
- `gh release create` and `npm publish` were never run (by design). The real download URL layout
  (`.../releases/download/v<version>/<artifact>`) is untested against GitHub itself.
- The MCP config snippets in the runbook use the standard `mcpServers` shape; I did not run them in Claude Code or Cursor.
- Shim edge cases not handled: concurrent first runs (both download, last rename wins), Windows rename-over-running-binary,
  proxy variables beyond Node's `fetch` defaults, resume/retry, stale `.download-*` cleanup, cache pruning.

## Findings you should know about

1. **RESOLVED: `ambit hook install` used to hardcode the absolute binary path.** The hook now runs `ambit check` via PATH,
   falling back to `npx --yes @kojaco/ambit check`; no absolute path is embedded.
2. **`ambit check` exits 0 on out-of-scope files** (it prints a report; non-zero only on errors), so the pre-commit hook does
   not block out-of-scope commits. This matches the docs ("not `--strict` by default") but is worth knowing.
3. **Assignment UI was missing** (03.6 only had the backend; no component ever called `setAssignment`, and nothing exposed the
   current assignment). Added: `GET /assignment`, an `assigned` flag on node summaries, an inspector "Agent assignment" section
   and an "Agent" chip. The file watcher now also reacts to `.arch/local.json`, so assignments made via CLI/MCP show up live.
4. **`.arch/` in this repo is your Example media model**, not an ambit self-model, so your uncommitted `.gitignore` line
   ignoring it looks deliberate. I never staged `.gitignore`. `.playwright-mcp/` (my screenshots) is untracked; add it to
   `.gitignore` or delete it.
5. `make build` runs `npm ci` (deletes and reinstalls `frontend/node_modules`).

## For your design pass (seen in the embedded UI on the Example model)

- Label pills overlap each other where several edges converge (right side of the Example Boundary level: "writes <a>",
  "writes <b>"), and some pills cover card text where nodes are close.
- Dense bundles of parallel edges into one node; consider collapsing parallel relationships into the existing
  "N relationships" pill at group boundaries.
- Small blue handle dots appear on group containers.
- The right rail (Review/Inspector) covers the canvas; fit-view pads only the left sidebar.
- Backward edges can still cross nodes; group resize handles are mouse-only.
- Opening an aggregated pill logs a 404 per member (probes); the clean fix is a per-member `drillable` flag from the server.

## Manual steps only you can do (when you want to publish)

1. npm name is resolved (`@kojaco/ambit`; you need publish rights on the `@kojaco` scope). Add a `LICENSE`.
2. `make release VERSION=0.1.0` (refuses without all 6 checksums, runs the shim tests).
3. Create the GitHub release `v0.1.0` on `KoJaco/ambit` and upload `bin/release/*`.
4. `cd npm && npm pack --dry-run`, then `npm publish --access public`. (Commit or revert `npm/checksums.json` and
   `npm/package.json` afterwards; they are tracked placeholders.)
5. Smoke test on a clean machine: `npx @kojaco/ambit version`, then `npx @kojaco/ambit init && npx @kojaco/ambit start`.

## Plugging into your own apps (checklist)

- [ ] `make install`, `ambit version`
- [ ] In the app repo: `ambit init`, add `.arch/local.json`, `.arch/.cache/`, `.arch/.proposals/` to its `.gitignore`
- [ ] `ambit start`, build the model visually; group related nodes, set "Agent may edit" scopes, mark protected nodes
- [ ] Add the MCP config (`.mcp.json` / `.cursor/mcp.json`, `ambit mcp`), restart the harness
- [ ] Assign a node to the agent (inspector), have the agent call `get_context` / `check_scope` / `update_node_status`
- [ ] `ambit hook install` (optional)
- [ ] 06.7: record what you had to change in the brief after a real session

## Assignment follow-up (verified in the browser)

Embedded binary on a copy of your model: inspector showed "No active assignment" (Clear disabled); pressing Assign stored
`{"node_id":"upload-service","assigned_at":...}`, put an "Agent" chip on that node, and the inspector said "Assigned to this node";
the node's status stayed `draft`. A `DELETE /assignment` from outside the UI removed the chip and updated the inspector within
~1s. Not eyeballed: chip placement on a collapsed summary card or a dark-mode screenshot; the dangling-assignment display
(unit-tested only). Decision for you: the chip says "Agent"; rename if you prefer "Assigned".
