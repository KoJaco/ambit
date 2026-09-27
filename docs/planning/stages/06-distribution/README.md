# Stage 06 — Distribution

## Status

Not started.

## What this stage proves

A person can run ambit without installing Go or keeping a Node process up. The SPA and the
API are one binary. The npx shim will not run a binary it has not checksummed.

## Build-plan steps

Step 9. Ordering: [`v1-build-plan.md`](../../v1-build-plan.md).

## Depends on

[Stage 03](../03-architect-canvas/README.md) for a SPA that can be built, and
[stage 05](../05-agent-interface/README.md) if the binary is meant to include `ambit mcp`.
The embed itself only needs the frontend build. Shipping a downloadable binary that is
missing MCP would be a broken release, so this stage waits until the subcommands exist.

## In scope

- `react-router build` producing a static client bundle.
- `go:embed` of that bundle into the `ambit` binary.
- `ambit start` serving the bundle, with fallback to the entry document for client routes.
- An `npx ambit` shim: platform detection, download, cache, checksum verification.
- The checksum release gate.

Distribution shape: [ADR-0003](../../../adr/0003-runtime-and-distribution.md).

## Out of scope

- Homebrew, `go install`, or a download page as the primary path. Those can be added later.
  `npx` is the path this stage has to get right.
- A Node server at runtime. The shim is Node only long enough to fetch the binary.
- Requiring Node, Go, or a package manager on the machine that only runs the binary.
- Independent versioning of the SPA and the API. They ship in one artifact.
- CI for the product. Whether the release-gate tests run in this repo's own CI is still
  open in [decision note 0004](../../../decisions/0004-testing-strategy.md) and is not
  settled here.

## Release gate

Task 06.5. A test feeds the shim a binary and a checksum that does not match, and the shim
refuses to execute it. A matching checksum is allowed to proceed. The shim does not ship
with this test failing or skipped.

## Watch-items

- **Concurrent writes** remain unsolved. Distribution does not add a lock.
- **Embedding means a frontend change rebuilds the Go binary.** The Makefile or build
  script should make that the normal path so a stale bundle is hard to ship by accident.

## Links

- Tasks: [`tasks.md`](tasks.md)
- Tests: [`test-plan.md`](test-plan.md)
- Slice log: [`slice-log.md`](slice-log.md)
- ADR: [0003](../../../adr/0003-runtime-and-distribution.md)
- Spec: [`spec-v1.md`](../../spec-v1.md) Section 7
