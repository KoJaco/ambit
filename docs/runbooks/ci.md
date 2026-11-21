# Runbook: CI

Workflow: `.github/workflows/ci.yml` (runs on pull requests and pushes to `main`; superseded
runs on the same ref are cancelled). Dependabot (`.github/dependabot.yml`) opens weekly updates
for GitHub Actions, Go modules, and npm in `frontend/` and `npm/`. CI never publishes or
releases anything; see [release.md](release.md).

| Job | Checks | Run locally |
| --- | --- | --- |
| `go` | build, vet, tests with the race detector | `go build ./... && go vet ./... && go test ./... -race -count=1` |
| `lint (informational)` | prints files `gofmt -l` would change; never fails | `gofmt -l .` |
| `frontend` | `npm ci`, typecheck, vitest | `cd frontend && npm ci && npm run typecheck && npx vitest run` |
| `shim` | the npx shim tests (`npm/`) | `cd npm && node --test` |
| `build-smoke` | `make build VERSION=ci`, then serves the binary and checks `/` and `/node/x` return HTML and `/nodes/zzz` returns JSON 404 | `make build VERSION=ci && scripts/ci-smoke.sh` |
| `release-dry-run` | push to `main` only: `ALLOW_DIRTY=1 scripts/release.sh 0.0.0-ci`, expects 6 binaries and a 6-line `checksums.txt` | same command; afterwards restore `npm/checksums.json` and `npm/package.json` with git, and clean `bin/release` |

`go` uses `go-version-file: go.mod`, so the toolchain follows `go.mod`.

## Why lint is informational

The repo has pre-existing `gofmt` findings and no linter config. Gating on them would block
unrelated work, and fixing them is a separate, drive-by diff. The job reports the list so it
can be cleaned up in a dedicated change, after which it can become a required check.

## Package managers

The repo uses npm with a single `package-lock.json` (in `frontend/`). Do not add other
lockfiles. The published shim is zero-dependency CommonJS, so `npx`, `pnpm dlx`, `yarn dlx`
and `bunx` all run it.
