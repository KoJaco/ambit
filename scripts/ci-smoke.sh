#!/usr/bin/env bash
# Smoke test for a built binary: serves the embedded UI and checks content types.
# Usage: scripts/ci-smoke.sh [path/to/ambit]   (default: bin/ambit; run `make build` first)
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin="${1:-$root/bin/ambit}"
case "$bin" in /*) ;; *) bin="$PWD/$bin" ;; esac
[ -x "$bin" ] || { echo "error: $bin not found; run 'make build' first" >&2; exit 1; }

work="$(mktemp -d)"
pid=""
cleanup() {
  [ -n "$pid" ] && kill "$pid" 2>/dev/null || true
  [ -n "$pid" ] && wait "$pid" 2>/dev/null || true
  rm -rf "$work"
}
trap cleanup EXIT

cd "$work"
"$bin" init . >/dev/null
"$bin" start --addr 127.0.0.1:0 >"$work/server.log" 2>&1 &
pid=$!

addr=""
for _ in $(seq 1 50); do
  addr="$(sed -n 's/^listening on //p' "$work/server.log" | head -n1)"
  [ -n "$addr" ] && break
  kill -0 "$pid" 2>/dev/null || { echo "error: server exited early" >&2; cat "$work/server.log" >&2; exit 1; }
  sleep 0.2
done
[ -n "$addr" ] || { echo "error: no 'listening on' line" >&2; cat "$work/server.log" >&2; exit 1; }
echo "server at $addr"

fail=0
check() { # path expected-status expected-content-type-prefix
  local path="$1" status="$2" ctype="$3" out got_status got_type
  out="$(curl -sS -o /dev/null -w '%{http_code} %{content_type}' "http://$addr$path")"
  got_status="${out%% *}"
  got_type="${out#* }"
  case "$got_type" in
    "$ctype"*) [ "$got_status" = "$status" ] && { echo "ok   $path -> $got_status $got_type"; return; } ;;
  esac
  echo "FAIL $path -> $got_status $got_type (want $status $ctype*)" >&2
  fail=1
}

check /           200 text/html
check /node/x     200 text/html
check /nodes/zzz  404 application/json

[ "$fail" = 0 ] && echo "smoke ok"
exit "$fail"
