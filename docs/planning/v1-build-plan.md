# ambit v1 — Build Plan

## Status

Canonical ordering for the v1 MVP. Task lists live in the [stage directories](stages/);
status lives in [`checklist.md`](checklist.md).

## Date

2026-09-27

## Purpose

The ordered path from an empty repository to a working v1, with the rationale for the
ordering and the gates that stop a stage being declared done prematurely.

The target vocabulary, contracts, and decisions this plan builds against are in
[`spec-v1.md`](spec-v1.md), the [ADRs](../adr/), and the [contracts](../contracts/).

## Ordering principle

The canonical model and the enforcement check are the part worth getting right slowly.
Everything after them assumes the schema and the core package hold up, and a mistake in
either propagates into the HTTP API, the MCP surface, and the frontend simultaneously.

Enforcement is deliberately early and out of dependency order — `ambit check` could
technically wait until there is a UI to configure nodes from. It comes before the canvas
because it is small, and because it is the product's actual differentiator. Building it
early means the thing ambit is *for* is provable before any of the presentation layer
exists, and if the glob-to-node mapping turns out to be unworkable, that is far better
learned before the frontend exists than after.

The review gate comes before MCP. The reverse order would mean a window in which MCP
authoring tools have nowhere to stage to, and the tempting fix at that moment is to let
them write directly — which is the decision
[ADR-0002](../adr/0002-agent-interface-and-review-gate.md) explicitly rejects.

`ambit-core` grows across stages. Later stages add to it. They do not re-plan earlier work.
How that sequence moves through git is in [`workflow.md`](workflow.md): one branch per
stage, merged to `main` when the stage is done.

## Stages

### Stage 00 — Foundation — done

Build step 0. Repository topology. Recorded in
[stage 00](stages/00-foundation/README.md).

### Stage 01 — Canonical model — steps 1 and 2 — RELEASE GATE

[Stage README](stages/01-canonical-model/README.md).

`.arch` read and write, `ambit init`, the in-memory index, and validated mutations.
Format: [`arch-model-format.md`](../contracts/arch-model-format.md).

**Gate:** the integration test runs the full `ambit init` flow in a temp directory, actually
runs `git init`, actually runs `git add -A`, and asserts that `local.json`,
`.arch/.cache/`, and `.arch/.proposals/` do not appear in `git status --porcelain`.

The runtime `git check-ignore` warning is implemented in
[stage 02](stages/02-enforcement/README.md) and called again from `ambit start` in
[stage 03](stages/03-architect-canvas/README.md). Stage 01 writes the ignore entries and
proves git honours them. The later stages prove the running commands notice when someone
has undone that.

### Stage 02 — Enforcement — step 3 — RELEASE GATE

[Stage README](stages/02-enforcement/README.md).

Semantics: [`enforcement-model.md`](../architecture/enforcement-model.md).

**Gate:** unit coverage on glob matching, `protected`, and the unmapped-file rules. This is
where a bug either lets an agent out of its boundary or blocks legitimate work, and both
erode trust in the mechanism the product rests on.

### Stage 03 — Architect canvas — steps 4, 5, and 6

[Stage README](stages/03-architect-canvas/README.md).

Local HTTP API for one drill-down level, direct graph edits, layout cache, and SSE for
model and integrity changes. Then the frontend hard reset and elkjs. Inventory:
[`frontend-refactor.md`](stages/03-architect-canvas/frontend-refactor.md).

Proposal endpoints and the review UI wait for stage 04. This is the one stage that is not
incrementally shippable — there is a period where the frontend does not build.

### Stage 04 — Review gate — step 7 — RELEASE GATE

[Stage README](stages/04-review-gate/README.md).

Contract: [`proposals.md`](../contracts/proposals.md).

**This replaces the original spec's step 7 (LLM proxy and single-turn UI), which is cut
entirely.**

A test helper stages proposals so apply and the review UI are provable before MCP exists.

**Gate:** apply atomicity and staleness hash comparison under test.

### Stage 05 — Agent interface — step 8 — RELEASE GATE

[Stage README](stages/05-agent-interface/README.md).

Contract: [`mcp-tools.md`](../contracts/mcp-tools.md).

**Budget real iteration time on `get_context`'s framing.** It is not a template to write
once. Test against Codex, Cursor, and Claude Code directly, and record what was learned when
the wording changes.

**Gate:** a test asserting `check_scope` and `ambit check` return the same verdict for the
same inputs. The contract claims they cannot disagree; that should be enforced by a test
rather than by intent.

### Stage 06 — Distribution — step 9 — RELEASE GATE

[Stage README](stages/06-distribution/README.md).

`go:embed` of the static SPA, and the `npx ambit` shim.

**Gate:** checksum verification on download. Do not ship the shim without it.

## Release gates, collected

1. **The `.gitignore` integration test** (stage 01) — real `init`, real `git init`, real
   `git add -A`, asserting absence from `git status --porcelain`.
2. **Enforcement unit coverage** (stage 02) — glob matching, `protected`, unmapped-file rules.
3. **Proposal apply atomicity and staleness** (stage 04).
4. **`check_scope` / `ambit check` equivalence** (stage 05).
5. **npx shim checksum verification** (stage 06).

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
- Workflow: [`workflow.md`](workflow.md)
- Checklist: [`checklist.md`](checklist.md)
- Stages: [`stages/`](stages/)
- Superseded ordering:
  [`docs/archive/spec-v0-original.md`](../archive/spec-v0-original.md) Section 13
- Frontend inventory:
  [`stages/03-architect-canvas/frontend-refactor.md`](stages/03-architect-canvas/frontend-refactor.md)
- Unresolved: [`open-questions.md`](open-questions.md)
