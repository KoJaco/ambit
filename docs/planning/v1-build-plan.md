# ambit v1 — Build Plan

## Status

Canonical plan for the v1 MVP.

## Date

2026-09-27

## Purpose

The ordered path from an empty repository to a working v1, with the rationale for the
ordering and the gates that stop a step being declared done prematurely.

The target vocabulary, contracts, and decisions this plan builds against are in
[`spec-v1.md`](spec-v1.md), the [ADRs](../adr/), and the [contracts](../contracts/).

## Ordering principle

Steps 1 through 3 are the part worth getting right slowly. Everything after them assumes the
schema and the core package hold up, and a mistake in either propagates into the HTTP API,
the MCP surface, and the frontend simultaneously.

Step 3 is deliberately early and out of dependency order — `ambit check` could technically
wait until there is a UI to configure nodes from. It comes third because it is small, and
because it is the product's actual differentiator. Building it early means the thing ambit
is *for* is provable before any of the presentation layer exists, and if the glob-to-node
mapping turns out to be unworkable, that is far better learned in week one than week three.

## Step 0 — Repository topology

Not in the original spec's ordering, but it precedes everything.

- Reconcile the uncommitted template-stripping changes in `frontend/`.
- Remove `frontend/.git` and its `node-canvas-template` remote.
- `git init` at the project root; write a root `.gitignore` covering Node, Go, and ambit
  paths.
- Scaffold the Go module: `/cmd` for CLI entry points, `/internal` for `ambit-core` and the
  front doors.

Details and the full follow-up list: [decision note 0001](../decisions/0001-repo-topology.md).

## Step 1 — `.arch` schema and `ambit init` — RELEASE GATE

Implement the format in
[`arch-model-format.md`](../contracts/arch-model-format.md): node `.json` and sibling `.md`,
`index.json` with `schema_version`, `config.json`, `local.json`.

`ambit init` scaffolds `.arch/`, creates `.gitignore` if missing, and appends the ignore
entries **idempotently** — checking for an existing entry before appending, so re-running
`init` is safe.

### The gate

**This step is not complete until the integration test passes.** Not a unit test, and not a
content assertion on `.gitignore` — asserting a string was written proves nothing about
whether a file stays out of a commit.

The gating test:

1. Runs the full `ambit init` flow in a temp directory.
2. Actually runs `git init`.
3. Actually runs `git add -A`.
4. Asserts `local.json`, `.arch/.cache/`, and `.arch/.proposals/` do **not** appear in
   `git status --porcelain`.

There is no API key in ambit any more, so the original spec's "leaked key" framing is gone.
The gate survives on different grounds: `.proposals/` holds unreviewed agent-authored
content, and committing it would mean exactly the failure the review gate exists to prevent —
unapproved agent output entering git disguised as the model. `local.json` holds
machine-local assignment state, and `.cache/` holds derived layout.

A content assertion would pass in every case that actually breaks: `init` running before
`git init`, a pre-existing `.gitignore` merging oddly, or an entry being edited out later.

### Also in this step

Runtime verification via `git check-ignore`, called by both `ambit start` and `ambit check`,
warning loudly when the entries are not in effect.

## Step 2 — `ambit-core`

The package everything else calls. See
[`system-overview.md`](../architecture/system-overview.md).

- File read and write for the model.
- The in-memory index: `map[NodeID]*Node`, `parent_id` for hierarchy, and a **separate**
  adjacency index for cross-cutting relationships. Separate because relationships jump
  across the tree and would corrupt a structure assuming one.
- Mutations with validation: create, update, delete, set relationship, set scope and
  protected. Cycle prevention, reference integrity, slug derivation with collision
  suffixing.
- File watching and index rebuild.

**Gate:** unit coverage on hierarchy validation, cycle prevention, and ID derivation.

## Step 3 — `ambit check`

Semantics in [`enforcement-model.md`](../architecture/enforcement-model.md).

- `git diff --name-only`, map files to nodes via `implementation` globs.
- Read the active assignment from `local.json`.
- Apply the rules: `protected` is absolute, empty `scope` defaults to `implementation`,
  unmapped files are violations when an assignment is active and informational drift when
  not.
- Report naming both the node and the rule hit. Exit 0 — warns, does not block.
- `ambit hook install`, opt-in, refusing to clobber an existing `pre-commit` hook.

**Gate:** unit coverage on glob matching, `protected`, and the unmapped-file rules. This is
where a bug either lets an agent out of its boundary or blocks legitimate work, and both
erode trust in the mechanism the product rests on.

## Step 4 — Local HTTP API

Contract: [`local-http-api.md`](../contracts/local-http-api.md).

- Graph CRUD over `ambit-core`, writing directly — the architect is in the seat.
- The drill-down level endpoint, returning one level. **Never the whole graph.**
- Layout cache read and write.
- `fsnotify` to SSE on `/events`.
- `git check-ignore` verification before serving.

## Step 5 — Frontend hard reset

Inventory: [`frontend-refactor.md`](frontend-refactor.md).

Delete the pipeline domain model, rebuild the canvas around the ambit node model, wire to
the HTTP API, and route drill-down at `/node/:nodeId`. Consolidate onto `@xyflow/react` v12.

This is the one step that is not incrementally shippable — there is a period where the
frontend does not build. Sequenced after the API so there is something real to wire to,
rather than rebuilding against a mock and integrating twice.

## Step 6 — elkjs layout and caching

Per drill-down level, never the whole graph flattened. Positions persisted to
`.arch/.cache/layout/`, recomputed only on structural change, with manual drag writing into
the same cache. See [`frontend.md`](../architecture/frontend.md).

## Step 7 — Proposals

Contract: [`proposals.md`](../contracts/proposals.md).

**This replaces the original spec's step 7 (LLM proxy and single-turn UI), which is cut
entirely.**

- The `.arch/.proposals/<id>/` format: manifest, operations, base hashes, materialised
  would-be files.
- Staging, staleness computation, and atomic apply in `ambit-core`.
- The review UI: per-node accept and reject, stale operations visually distinct and
  requiring explicit confirmation, accept-all skipping stale operations rather than sweeping
  them.

Sequenced before MCP so the review gate exists before anything can author through it. The
reverse order would mean a window in which MCP authoring tools have nowhere to stage to, and
the tempting fix at that moment is to let them write directly — which is the decision
[ADR-0002](../adr/0002-agent-interface-and-review-gate.md) explicitly rejects.

**Gate:** apply atomicity and staleness hash comparison under test.

## Step 8 — MCP server

Contract: [`mcp-tools.md`](../contracts/mcp-tools.md).

All eight tools over stdio, as a process independent of `ambit start`. Five authoring tools
staging to proposals; `get_context`, `check_scope`, and `update_node_status` direct.

**Budget real iteration time on `get_context`'s framing.** It is not a template to write
once. Test against Codex, Cursor, and Claude Code directly, and record what was learned when
the wording changes. The brief must enumerate allowed globs, explicitly *name* protected
paths and siblings as off-limits rather than omitting them, state the stop-and-report escape
hatch, and instruct the agent to call `check_scope` periodically.

**Gate:** a test asserting `check_scope` and `ambit check` return the same verdict for the
same inputs. The contract claims they cannot disagree; that should be enforced by a test
rather than by intent.

## Step 9 — Distribution

- `react-router build` to a static bundle, `go:embed`-ed into the binary.
- The `npx ambit` shim: platform detection, download, caching, and **checksum verification**.
  Do not ship without the last one.

## Release gates, collected

1. **The `.gitignore` integration test** (step 1) — real `init`, real `git init`, real
   `git add -A`, asserting absence from `git status --porcelain`.
2. **Enforcement unit coverage** (step 3) — glob matching, `protected`, unmapped-file rules.
3. **`check_scope` / `ambit check` equivalence** (step 8).
4. **Proposal apply atomicity and staleness** (step 7).
5. **npx shim checksum verification** (step 9).

Testing strategy: [decision note 0004](../decisions/0004-testing-strategy.md).

## What is explicitly not in this plan

Deferred with intent, listed in full in [`spec-v1.md`](spec-v1.md) Section 12. The items
most likely to creep back in during the build:

- **An LLM client of any kind.** The harness owns generation entirely. There is no
  instruction box in the UI, single-turn or otherwise.
- **A node type registry or enum.** Free-form strings; see
  [decision note 0002](../decisions/0002-node-type-taxonomy.md).
- **Policy rules beyond `protected` and `scope`.** Keep the check dumb. This is the easiest
  place in the product to start competing on a feature list that a funded team will win.
- **Blocking by default.** Exit 0; `--strict` is a future flag, not a v1 one.
- **SQLite or any database.**
- **Component or end-to-end frontend tests.**

## References

- Canonical spec: [`spec-v1.md`](spec-v1.md)
- Superseded ordering:
  [`docs/archive/spec-v0-original.md`](../archive/spec-v0-original.md) Section 13
- Frontend inventory: [`frontend-refactor.md`](frontend-refactor.md)
- Unresolved: [`open-questions.md`](open-questions.md)
