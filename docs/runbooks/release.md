# Runbook: cutting an ambit release

Nothing here is automated in CI (product CI is out of scope; see
[stage 08](../planning/stages/08-distribution/README.md)). A human runs every step.
Design: [decision 0011](../decisions/0011-distribution-pinned-checksums.md).

## Prerequisites

- Go (version in `go.mod`), Node >= 18 and npm, `make`, `git`.
- `gh` authenticated for `KoJaco/ambit` (for the GitHub release).
- An npm account that can publish the package name (see "npm name" below).
- A clean tree on the commit you intend to release. Tag and release from that commit.

## Versioning

One version everywhere: the git tag is `v<version>`, the binary reports `<version>`
(`ambit version`), and `npm/package.json` has `<version>`. `<version>` has no leading `v`
(`0.1.0`, `0.1.0-rc.1`). The script sets all of them from one argument. The shim refuses to
run if `npm/checksums.json` and `npm/package.json` disagree on the version.

## Steps

1. Run the release build (does not publish anything):

       make release VERSION=0.1.0

   This runs `scripts/release.sh 0.1.0`, which:
   - refuses if tracked files are modified (override with `ALLOW_DIRTY=1`, for dry runs only);
   - runs `make webui` (fresh `npm ci` + build, copied into `internal/webui/dist`);
   - cross-builds six binaries into `bin/release/` with `CGO_ENABLED=0 -trimpath` and
     `-X main.version=0.1.0`:
     `ambit_0.1.0_{darwin,linux,windows}_{amd64,arm64}` (`.exe` on windows);
   - writes `bin/release/checksums.txt` (`sha256sum` format) and `npm/checksums.json`;
   - sets the version in `npm/package.json`;
   - fails if any artifact or checksum is missing or not 64 hex characters;
   - runs `cd npm && node --test`.

2. Create the GitHub release and upload the binaries and `checksums.txt`:

       gh release create v0.1.0 bin/release/* --repo KoJaco/ambit --title v0.1.0

   The shim downloads from `https://github.com/KoJaco/ambit/releases/download/v<version>/<artifact>`.

3. Check the package contents, then publish:

       cd npm && npm pack --dry-run
       cd npm && npm publish --access public

   Publish only after step 2: the published shim points at assets that must exist.

4. Commit `npm/checksums.json` and `npm/package.json` only if you want the repo to record the
   released state. If you are not publishing, revert them (they are tracked placeholders,
   `0.0.0-dev` with no artifacts):

       git checkout -- npm/checksums.json npm/package.json

   Output under `bin/` is gitignored. `make clean` removes it.

## Checksum design

`npm/checksums.json` ships inside the npm package and is generated from the exact binaries
uploaded to the release. The shim hashes the downloaded file and compares it to the pinned
value before it is made executable, cached, or run. Pinning them in the package (rather than
a `checksums.txt` next to the binary) means an attacker who replaces the release assets would
also have to compromise the npm package. `bin/release/checksums.txt` is for humans and for
manual verification. A release without published checksums is not a release.

## Testing locally without publishing

    ALLOW_DIRTY=1 scripts/release.sh 0.0.0-smoke
    mkdir -p /tmp/ambit-serve/v0.0.0-smoke && cp bin/release/ambit_* /tmp/ambit-serve/v0.0.0-smoke/
    (cd /tmp/ambit-serve && python3 -m http.server 8099 --bind 127.0.0.1) &
    AMBIT_DOWNLOAD_BASE=http://127.0.0.1:8099 AMBIT_CACHE_DIR=$(mktemp -d) \
      node npm/bin/ambit.js version        # prints: ambit 0.0.0-smoke

The server must serve files under `/v<version>/`. Flip a byte in the served binary and use a
fresh cache dir to see the refusal ("not trustworthy", non-zero exit, nothing cached). When
finished, stop the server, `git checkout -- npm/checksums.json npm/package.json`, and
`make clean`.

## npm name

Availability of the unscoped name `ambit` on npm has not been checked. Check with
`npm view ambit`. If it is taken, use a scoped name (for example `@kojaco/ambit`; scoped
public packages need `npm publish --access public`, which the steps above already use).
Change it in:
- `npm/package.json` (`name`; the `bin` key can stay `ambit`, so the command is still `ambit`);
- `npm/README.md` and the usage lines (`npx @scope/ambit ...`);
- `docs/runbooks/using-ambit-in-your-app.md` and this file.

The GitHub release location is independent of the npm name.
