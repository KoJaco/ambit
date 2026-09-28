# ADR-0002: Agent Interface and the Review Gate

## Status

Accepted

## Date

2026-09-27

## Context

The original spec gave ambit its own LLM client. The architect typed a single-turn
instruction into the UI, ambit proxied it to Anthropic using a key stored in
`.arch/local.json`, and the result appeared as a diff preview to accept or reject. MCP
existed alongside this as a narrow read-and-report sidecar: three tools, only one of which
(`update_node_status`) wrote anything, and that only a single field.

That design was rejected for a simple reason: ambit's users are already sitting in a coding
harness — Codex, Cursor, Claude Code — with a model subscription, a context window full of
the relevant repository, and a UI for reviewing generated output that is better than
anything ambit would build in v1. Shipping a second, worse LLM client inside ambit
duplicates all of that and adds a provider dependency, a secret to handle, a billing
relationship, and a compatibility burden, in exchange for nothing the user wanted.

Removing the LLM client has a consequence that is not optional to address. The product
principle — *the human is in the seat driving the agents, not the agents driving the human*
— was enforced in the original design by an accident of layout: the architect typed the
instruction and saw the result in the same window, so the accept/reject gate was
automatically co-located with the human. Once an agent in a separate terminal is authoring
the model, the human is simply not present at the moment of writing. The gate has to be
rebuilt explicitly or the product principle quietly becomes a slogan.

## Decision

### ambit ships no LLM client

No proxy endpoint, no provider SDK, no API key, no model configuration. ambit never makes an
outbound network call to a model provider. Generation happens in the architect's harness;
the harness calls ambit's MCP tools with the result.

### MCP becomes the primary authoring surface — eight tools

Authoring (staged): `seed_model`, `create_node`, `update_node`, `delete_node`,
`set_relationship`.

Read and report (direct): `get_context`, `check_scope`, `update_node_status`.

All eight are thin wrappers over `ambit-core`, sharing the identical validation and
file-write path as the HTTP API.

### Structural mutations are staged, not applied

Writes from the five authoring tools land in `.arch/.proposals/<proposal-id>/`, which is
gitignored. Each proposal holds the would-be node files plus a `manifest.json` listing the
operations and the base content hash each was computed against. Nothing enters
`.arch/nodes/` without the architect accepting it.

### Review is per-node, with staleness detection

The UI renders a proposal as a diff against current state. The architect accepts or rejects
per node, with accept-all and reject-all shortcuts. When a manifest's recorded content hash
no longer matches the node on disk — because the architect edited it after the proposal was
staged — the UI flags that operation as stale and requires explicit confirmation before
applying over it.

### `update_node_status` and architect UI edits write directly

Status transitions from an assigned agent bypass staging. Architect edits made in the UI
bypass staging.

### Enforcement stays layered

`get_context` framing lowers violation frequency. `check_scope` gives voluntary early
feedback. `ambit check` is the mandatory backstop. All three remain, and the third stays
required regardless of how well the first two work.

## Alternatives Considered

- **Keep the local LLM proxy** (the original design) — rejected as described above. It also
  put ambit in the position of owning a secret, which is the single highest-severity failure
  mode a local-first tool can have.

- **Harness for transcript seeding, local proxy for in-UI node refinement** — a hybrid that
  keeps the convenience of editing without leaving the canvas. Rejected because it retains
  every cost of the LLM client (key handling, provider dependency, billing) to serve only
  the narrower half of the use case, and it means two different generation paths with two
  different review behaviours.

- **Both paths supported, proxy optional** — same objection, plus an ongoing maintenance
  burden on a code path most users would not enable.

- **Agents write directly to `.arch/nodes/`, with `git diff` as the review gate** — the most
  tempting alternative, because it is genuinely how the *code* half of the workflow already
  works, and it needs no staging machinery at all. Rejected on two grounds. First, the model
  is the instrument of review, not the thing under review: if unreviewed agent output can
  enter the model, the model can no longer be trusted as the baseline that `ambit check`
  evaluates code against. Second, `seed_model` against a real transcript produces the entire
  initial model in one shot; landing thirty unreviewed node files in the working tree and
  calling `git checkout` the reject mechanism is not a review gate, it is a cleanup task.

- **Direct writes with a pending-changes banner and per-node git revert** — keeps staging
  out of ambit by leaning on git for undo. Rejected because reverting is not the same as
  never having applied: between write and revert the model is wrong, and any concurrent
  `ambit check` or `get_context` call reads the unreviewed state.

- **Stage everything, including `update_node_status`** — consistent, but it means an agent
  finishing a task leaves a proposal sitting unreviewed until the architect returns, so
  `status` no longer reflects reality. The risk being guarded against is an agent
  restructuring the model; a single enumerated field that cannot restructure anything does
  not carry that risk.

- **A `config.json` trust setting allowing direct structural writes from MCP** — rejected as
  a setting whose only purpose is to disable the product's central safety property. Anyone
  who wants it can skip ambit.

- **Atomic whole-proposal accept/reject** — simpler to implement and to reason about.
  Rejected because a thirty-node seed proposal under an all-or-nothing gate trains the user
  to click accept without reading, which is worse than no gate at all because it looks like
  review.

- **Per-operation granularity, finer than per-node** — individually accepting a single field
  change within a node. Rejected as more UI and more manifest complexity than the workflow
  warrants; a node is the unit the architect thinks in.

- **Auto-reject stale operations** — safe, but makes the agent redo work whenever the
  architect touched anything nearby, including changes that do not actually conflict.
  Flagging and confirming leaves the judgment with the human, which is the whole posture.

- **Last-write-wins with no staleness tracking** — silently discards the architect's edit in
  favour of an agent's older view of the node. Exactly backwards.

## Consequences

- **Pros**
  - ambit has no model provider dependency, no secret to leak, no billing relationship, and
    no provider-compatibility surface. It works with whatever model the architect already
    pays for, including ones that do not exist yet.
  - Generation happens where the context already is: the harness has the repository loaded,
    which ambit's proxy never would have.
  - The review gate is now explicit and testable rather than a property of window layout.
  - Per-node review with staleness flagging keeps the architect's judgment in the loop at
    the granularity they actually think in.
  - Removing the LLM proxy and single-turn UI removes a meaningful slice of v1 build work.

- **Cons**
  - The staging layer is net-new machinery the original spec did not budget: a proposal
    format, apply and reject transactions, content hashing, a file watcher, an SSE channel,
    and a review UI. This is the single largest piece of added scope, and it is the price of
    keeping the product principle honest.
  - The architect works across two windows — the harness and the ambit UI — rather than one.
    Authoring and reviewing are no longer co-located.
  - ambit's authoring experience is now only as good as the harness's MCP support, which
    varies between clients and is outside ambit's control.
  - `.arch/.proposals/` is another directory that must never be committed, widening the
    `.gitignore` gate's surface.
  - An abandoned proposal is litter. Nothing in v1 expires or garbage-collects them.

- **Follow-ups / TODOs**
  - `get_context` prompt framing needs real iteration against Codex, Cursor, and Claude Code
    directly — budget time for it in build step 9 rather than treating it as a template.
  - Decide whether proposals need expiry or a `ambit proposals prune` command once there is
    evidence about how many get abandoned in practice.

## References

- Canonical spec: [`docs/planning/spec-v1.md`](../planning/spec-v1.md) Sections 8 and 9
- Contracts: [`docs/contracts/mcp-tools.md`](../contracts/mcp-tools.md),
  [`docs/contracts/proposals.md`](../contracts/proposals.md)
- Superseded source: [`docs/archive/spec-v0-original.md`](../archive/spec-v0-original.md)
  Sections 8 and 9
- Related: [ADR-0003](0003-runtime-and-distribution.md) for the SSE and process-model decisions
