# Contract: MCP Tool Surface

## Purpose

Defines the tools ambit exposes over MCP, and the semantics a coding harness can rely on.
This is the boundary between ambit and every agent that touches the model — Codex, Cursor,
Claude Code, or anything else speaking MCP.

Two distinct consumers use this surface for different things, and the split matters:

- **The architect's harness**, used to author the model from a transcript or to flesh out a
  node. Calls the staged authoring tools.
- **An assigned coding agent**, building the thing a node describes. Calls `get_context`,
  `check_scope`, and `update_node_status`.

## Scope

**In scope:** all eight tools, their arguments, return shapes, staging behaviour, and the
framing requirements for `get_context`.

**Out of scope:** the MCP protocol itself, the proposal file format (see
[`proposals.md`](proposals.md)), and the model file format (see
[`arch-model-format.md`](arch-model-format.md)).

## Versioning

- **Current version:** v1
- **Compatibility:** Additive. New tools and new optional arguments may be added; existing
  tool names, required arguments, and staging behaviour may not change meaning.
- **Change policy:** Moving a tool between staged and direct is a breaking change to the
  product's safety posture and requires an ADR, not a version bump.

## Definitions

- **Staged** — the tool's effect is written to `.arch/.proposals/`, not to the canonical
  model. It takes effect only when the architect accepts it in the UI.
- **Direct** — the tool writes to the canonical model immediately.
- **Assignment** — the node currently assigned to an agent, recorded in the gitignored
  `local.json`. Determines whose `scope` applies during `ambit check`.
- **Brief** — the formatted, imperative task description `get_context` returns. Not a data
  dump.

## Contract Shape (Conceptual)

Transport is **stdio**, invoked as `ambit mcp`. This matches how Codex, Cursor, and Claude
Code expect to launch a local MCP server: a direct command, not a running HTTP server.

The process is **independent of `ambit start`**. MCP works whether or not the UI server is
running. See [ADR-0003](../adr/0003-runtime-and-distribution.md).

All eight tools are thin wrappers over `ambit-core`, sharing the identical validation and
write path as the HTTP API. There is never a second implementation of what a valid mutation
is.

---

### Authoring tools (staged)

All five write to `.arch/.proposals/<proposal-id>/` and return the proposal ID. **None of
them modify the canonical model.** Every one of them must say so in its returned message, so
an agent does not report to its user that a change has been made when it has only been
proposed.

#### `seed_model(transcript, parent_id?)`

Propose a model from a client transcript. The headline authoring tool: the architect pastes
or points their harness at a transcript, and the harness produces a set of nodes and
relationships.

- `transcript` — the transcript text. Required.
- `parent_id` — optional. When present, the proposed nodes are children of that node,
  allowing a transcript to seed one branch rather than a whole model. When absent, they are
  root-level.

Returns the proposal ID and a summary of what was proposed — node count, names.

This tool exists **only** for transcript-seeded generation. Freeform
natural-language-to-architecture is explicitly out of scope for v1; see
[`docs/planning/spec-v1.md`](../planning/spec-v1.md) Section 12.

#### `create_node(name, type, parent_id?, implementation?, scope?, protected?, spec?)`

Propose one new node. `spec` is the markdown prose that becomes the node's sibling `.md`.
The node's `id` is derived from `name` by `ambit-core` at apply time, not supplied by the
caller.

#### `update_node(node_id, ...fields)`

Propose changes to an existing node. Any subset of mutable fields: `name`, `type`,
`parent_id`, `implementation`, `scope`, `protected`, `spec`.

`id` is immutable and cannot be updated. `status` is not settable here — use
`update_node_status`.

#### `delete_node(node_id)`

Propose deleting a node. Deleting a node with children is rejected at proposal time rather
than at apply time, so the agent gets the error while it still has context, instead of the
architect discovering it at review.

#### `set_relationship(from, to, label?, kind?)`

Propose creating or modifying a cross-cutting relationship. Directed. `kind` is one of
`sync`, `async`, `data`.

---

### Read and report tools (direct)

#### `get_context(node_id)`

Returns the assigned agent's task brief. **This is the most important tool in the surface**,
and the one whose output quality determines how often the enforcement backstop has to fire.

It returns a formatted, imperative brief — not a serialised node. A raw data dump would make
the agent infer its constraints, and inference is exactly what should not be happening here.

**Required framing.** The brief must:

- **Enumerate the allowed paths explicitly**, from the node's `scope` (defaulting to
  `implementation` when empty). List the globs; do not describe them in prose.
- **Name protected paths and sibling nodes as off-limits explicitly.** Do not merely omit
  them. An agent that has not been told a path is forbidden will treat it as unmentioned
  rather than prohibited, and unmentioned reads as permissible.
- **Include sibling nodes' interfaces without their implementation detail.** The agent needs
  enough to call into a neighbouring system correctly, and nothing that invites it to reach
  inside one.
- **Be imperative about the boundary and the escape hatch.** The agent may modify only files
  matching the given globs; if the task appears to require more, it must stop and report
  back rather than push through. The escape hatch has to be stated, or a capable agent will
  reason its way past the constraint to complete the task.
- **Instruct the agent to call `check_scope`** periodically during the task and before
  finishing.

**This framing needs real iteration against actual harnesses, not a one-shot template.**
Build step 9 budgets time for it. Treat the wording as a tuned artifact and record what was
learned when it changes.

#### `check_scope(node_id, files)`

Runs the same `CheckScope` logic `ambit check` applies to a git diff, against a caller-
supplied file list. Callable mid-task, before an agent has gone too far.

- `node_id` — the node whose scope applies.
- `files` — list of paths the agent has touched or intends to touch.

Returns, per file: allowed, or a violation naming the node and the rule hit (`protected`, or
outside `scope`).

This costs almost nothing to build — it is the same underlying function reached over MCP
instead of from a diff — and it converts end-of-task failure into mid-task correction for any
agent that uses it.

#### `update_node_status(node_id, status)`

Sets a node's `status` to one of `draft`, `specified`, `assigned`, `in_progress`, `done`,
`blocked`.

**Writes directly, bypassing staging.** This is a deliberate exception. Routing it through
review would mean an agent finishing work leaves a proposal sitting unreviewed, so `status`
stops reflecting reality until the architect returns. The risk staging guards against is an
agent restructuring the model; a single enumerated field that cannot restructure anything
does not carry that risk, and it is trivially revertable.

## Invariants (Must Always Hold)

1. **No authoring tool modifies the canonical model.** All five write only to
   `.arch/.proposals/`.
2. **Every staged tool's response states that the change is proposed, not applied.**
3. **Only `update_node_status` writes canonically.** Any future tool that writes directly
   requires an ADR.
4. **All tools go through `ambit-core`.** No MCP-specific validation or write path exists.
5. **`get_context` names forbidden paths explicitly.** Omission is not prohibition.
6. **`check_scope` and `ambit check` share one implementation.** They cannot disagree about
   whether a file is in scope; if they can, the contract is broken.
7. **MCP never requires `ambit start`.** The two processes are independent.
8. **`id` is never settable.** Not on create, not on update.
9. **ambit makes no outbound model-provider calls.** No tool in this surface talks to an
   LLM. Generation is the harness's job entirely.

## Error Handling

- **Unknown `node_id`** — error naming the ID, listing nothing. Do not fuzzy-match to a
  similar ID; a wrong node silently accepted is worse than a failed call.
- **Invalid `status`** — error listing the six valid values.
- **`delete_node` on a node with children** — rejected at proposal time with the child count
  and names, so the agent can reparent or delete them first.
- **`create_node` or `update_node` producing a hierarchy cycle** — rejected at proposal time.
- **Relationship endpoint naming a missing node** — rejected at proposal time.
- **Attempt to set `id`** — rejected, with the reason: IDs are immutable, rename via `name`.
- **Model integrity warnings** (orphans, dangling references) — surfaced on the call that
  encountered them, not swallowed. An agent working against a broken model should be told.
- **Ignore entries ineffective** — warned loudly, consistent with `ambit start` and
  `ambit check`.

## Examples

### Minimal valid call and response

`check_scope`:

```json
{
    "node_id": "payments-service",
    "files": ["src/payments/authorise.ts", "src/orders/create.ts"]
}
```

```json
{
    "results": [
        { "path": "src/payments/authorise.ts", "allowed": true },
        {
            "path": "src/orders/create.ts",
            "allowed": false,
            "rule": "outside_scope",
            "node": "orders-service",
            "detail": "belongs to orders-service; not in the scope declared for payments-service"
        }
    ]
}
```

### Staged call, showing the required response framing

`create_node`:

```json
{
    "name": "Refunds Service",
    "type": "service",
    "parent_id": "platform",
    "implementation": ["src/refunds/**"],
    "spec": "# Refunds Service\n\nOwns refund issuance..."
}
```

```json
{
    "proposal_id": "p-20260927-1043-a91f",
    "applied": false,
    "message": "Proposed 1 node (Refunds Service) as proposal p-20260927-1043-a91f. This has NOT been applied to the model. The architect must accept it in the ambit UI before it takes effect."
}
```

### Invalid example, with expected handling

```json
{
    "node_id": "payments-servic",
    "status": "done"
}
```

Expected handling: error naming the unknown ID. No fuzzy match to `payments-service`, and
no write. Marking the wrong node done is a silent corruption of the architect's view of what
is finished.

### Operational notes

- `get_context` output is the highest-leverage prompt in the product. Changes to its framing
  should be recorded in [`docs/decisions/`](../decisions/) with what was observed, not made
  silently.
- `check_scope` is cheap and intended to be called repeatedly. Nothing rate-limits it.
- Proposals accumulate; nothing in v1 expires them. See
  [ADR-0002](../adr/0002-agent-interface-and-review-gate.md) follow-ups.

### References

- ADR: [`docs/adr/0002-agent-interface-and-review-gate.md`](../adr/0002-agent-interface-and-review-gate.md)
- Canonical spec: [`docs/planning/spec-v1.md`](../planning/spec-v1.md) Section 8
- Related contracts: [`proposals.md`](proposals.md),
  [`arch-model-format.md`](arch-model-format.md)
- Enforcement:
  [`docs/architecture/enforcement-model.md`](../architecture/enforcement-model.md)
