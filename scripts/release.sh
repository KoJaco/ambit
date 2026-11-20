#!/usr/bin/env bash
# Build release artifacts and pin their checksums into the npm shim.
# Usage: scripts/release.sh <version>      (e.g. 0.1.0, no leading "v")
# Never pushes, tags, publishes, or uploads. Prints the manual steps at the end.
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <version>   (e.g. 0.1.0)" >&2
  exit 2
fi
version="$1"
if ! [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "error: version must look like 0.1.0 or 0.1.0-rc.1 (no leading v): $version" >&2
  exit 2
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

if [ "${ALLOW_DIRTY:-0}" != "1" ]; then
  if ! git diff --quiet HEAD --; then
    echo "error: tracked files have uncommitted changes; commit them or set ALLOW_DIRTY=1" >&2
    exit 1
  fi
fi

out="bin/release"
targets=(darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64)

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

echo "==> building web UI"
make webui
if [ ! -f internal/webui/dist/index.html ]; then
  echo "error: internal/webui/dist/index.html is missing after make webui" >&2
  exit 1
fi

rm -rf "$out"
mkdir -p "$out"

artifacts=()
for t in "${targets[@]}"; do
  goos="${t%/*}"
  goarch="${t#*/}"
  name="ambit_${version}_${goos}_${goarch}"
  if [ "$goos" = "windows" ]; then name="${name}.exe"; fi
  echo "==> building $name"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "-s -w -X main.version=${version}" -o "$out/$name" ./cmd/ambit
  artifacts+=("$name")
done

echo "==> computing checksums"
: > "$out/checksums.txt"
for a in "${artifacts[@]}"; do
  printf '%s  %s\n' "$(sha256_of "$out/$a")" "$a" >> "$out/checksums.txt"
done

# Write npm/checksums.json from checksums.txt (no jq).
node -e '
const fs = require("fs");
const [version, txt, dest] = process.argv.slice(1);
const artifacts = {};
for (const line of fs.readFileSync(txt, "utf8").split("\n")) {
  if (!line.trim()) continue;
  const m = /^([0-9a-fA-F]+)  (.+)$/.exec(line);
  if (m) artifacts[m[2]] = m[1].toLowerCase();
}
fs.writeFileSync(dest, JSON.stringify({ version, algorithm: "sha256", artifacts }, null, 2) + "\n");
' "$version" "$out/checksums.txt" npm/checksums.json

# Set the npm package version, preserving other fields.
node -e '
const fs = require("fs");
const [version, file] = process.argv.slice(1);
const p = JSON.parse(fs.readFileSync(file, "utf8"));
p.version = version;
fs.writeFileSync(file, JSON.stringify(p, null, 2) + "\n");
' "$version" npm/package.json

# Gate: a release without published checksums is not a release.
echo "==> verifying every artifact has a pinned checksum"
node -e '
const fs = require("fs");
const [version, json, dir, ...names] = process.argv.slice(1);
const c = JSON.parse(fs.readFileSync(json, "utf8"));
const bad = [];
if (c.version !== version) bad.push("checksums.json version " + c.version + " != " + version);
for (const n of names) {
  if (!fs.existsSync(dir + "/" + n)) bad.push("missing artifact " + n);
  const h = c.artifacts && c.artifacts[n];
  if (typeof h !== "string" || !/^[0-9a-f]{64}$/.test(h)) bad.push("missing or malformed checksum for " + n);
}
if (Object.keys(c.artifacts || {}).length !== names.length) bad.push("checksums.json must list exactly " + names.length + " artifacts");
if (bad.length) {
  console.error("error: refusing to release. A release without published checksums is not a release.");
  for (const b of bad) console.error("  - " + b);
  process.exit(1);
}
' "$version" npm/checksums.json "$out" "${artifacts[@]}"

echo "==> running shim tests"
(cd npm && node --test)

cat <<EOT

Release ${version} built in ${out}/. NOT published. Remaining steps are manual:

  1. Create the GitHub release and upload the artifacts:
       gh release create v${version} ${out}/* --repo KoJaco/ambit --title v${version}
  2. Publish the shim (check the tarball first):
       cd npm && npm pack --dry-run
       cd npm && npm publish --access public

If you are NOT publishing, revert the tracked placeholders:
  git checkout -- npm/checksums.json npm/package.json
EOT
