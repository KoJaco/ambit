# @kojaco/ambit (npx shim)

Thin launcher for the `ambit` CLI. It downloads the release binary for your
platform, verifies it against checksums pinned inside this package, caches it,
and runs it with your arguments. It never stays resident.

## Usage

    npx @kojaco/ambit init
    npx @kojaco/ambit start
    npx @kojaco/ambit mcp
    npx @kojaco/ambit check
    npx @kojaco/ambit method list
    npx @kojaco/ambit stage
    npx @kojaco/ambit check-model --strict
    npx @kojaco/ambit hook install

Other runners work the same way: `pnpm dlx @kojaco/ambit <cmd>`,
`yarn dlx @kojaco/ambit <cmd>`, `bunx @kojaco/ambit <cmd>`. The shim is a plain
zero-dependency CommonJS package, so npm, pnpm, yarn and bun runners all work.
After a global install (`npm i -g @kojaco/ambit`) the command is just `ambit`.

The unscoped name `ambit` on npm belongs to another package (a date parser), so
never run `npx ambit`.

All arguments pass through untouched; the exit code is the binary's. Shim
messages go to stderr only, so `ambit mcp` keeps stdout clean for JSON-RPC.

## Environment

- `AMBIT_DOWNLOAD_BASE`: release base URL (default
  `https://github.com/KoJaco/ambit/releases/download`). Artifacts are fetched from
  `<base>/v<version>/ambit_<version>_<os>_<arch>[.exe]`. Safe for mirrors because
  checksums are pinned in the package.
- `AMBIT_CACHE_DIR`: cache root (default `$XDG_CACHE_HOME` or `~/.cache` on Linux,
  `~/Library/Caches` on macOS, `%LOCALAPPDATA%` on Windows, plus `/ambit`).
  Layout: `<root>/<version>/<os>-<arch>/ambit[.exe]`.

## Supported platforms

darwin, linux, windows on amd64 (x64) and arm64. Anything else exits non-zero.

## Verification

`checksums.json` (version, sha256 per artifact) ships inside the package. The
downloaded file is hashed and compared to the pinned value before it is made
executable, moved into the cache, or run. On mismatch the temp file is deleted,
nothing is cached, and the shim exits non-zero. The shim also refuses when the
artifact entry is missing, when the checksums version differs from the package
version, or when the file is unreadable. A cached binary is re-hashed on each run
and re-downloaded if it no longer matches.

**A release without published checksums is not a release.**

## Status notes

- The repo copy of `checksums.json` is a placeholder (`0.0.0-dev`, no artifacts);
  the release script rewrites it and `package.json`'s version.
- No license is set (`UNLICENSED` placeholder).

## Tests

    cd npm && node --test
