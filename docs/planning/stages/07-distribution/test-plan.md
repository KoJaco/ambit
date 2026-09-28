# Stage 07 test plan

The gate is a shim test. Serving the embedded bundle is a Go test plus one manual refresh.
Decision: [0004](../../../decisions/0004-testing-strategy.md).

## Release gate — task 07.5

- Given a file and a checksum that does not match its bytes, the verifier returns an error
  and the caller does not exec.
- Given a matching checksum, the verifier returns success.
- A failed verification removes or never commits the downloaded bytes as the cache entry
  for that version, so the next run does not treat them as a hit.

Use a temp cache directory. Do not hit the network in the unit test; point the downloader
at a local fixture or test the verifier and the cache commit as the pieces the downloader
calls.

## Go

- An API 404 for an unknown node stays JSON.
- A non-API path returns the embedded entry document.
- The binary's tests can run without a prior manual copy step, because the frontend build
  is part of the command that produces the embed input. If `go test` of the embed package
  needs the bundle present, the test skips with a message that names `make build` when the
  bundle is absent, and `make build` itself fails when the bundle is absent. A skip is not
  the release path.

## Manual

- `make build` (or the task 07.1 command), then `ambit start` with no Vite server.
- Open `/`, drill into a node, refresh the browser on `/node/<id>`, and confirm the canvas
  returns.
- Run the shim against a local fixture binary once, with a good checksum and with a bad
  one. The bad one must not start.

## Not in this stage

- A test matrix of every OS in CI. Platform detection should be unit-tested with fake
  `process.platform` / architecture values. Actually downloading release artifacts for each
  OS can wait until a release exists.
- Playwright against the embedded server.
