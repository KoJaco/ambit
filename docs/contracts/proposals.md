# Contract: Proposal Staging Format

## Purpose

Defines the staging area where agent-authored structural mutations wait for the architect's
review. This is the mechanism that keeps the product principle — *the human is in the seat
driving the agents* — true once an agent in a separate terminal can author the model.

It is a contract between three parties: the MCP authoring tools that write proposals,
`ambit-core` which applies or discards them, and the review UI which renders them as a diff.

## Scope

**In scope:** the proposal directory layout, the manifest format, operation shapes, base
content hashing and staleness, and the accept/reject semantics.

**Out of scope:** the MCP tool signatures that produce proposals (see
[`mcp-tools.md`](mcp-tools.md)), the HTTP endpoints that review them (see
[`local-http-api.md`](local-http-api.md)), and the canonical model format (see
[`arch-model-format.md`](arch-model-format.md)).

## Versioning

- **Current version:** v1
- **Compatibility:** Proposals are ephemeral and gitignored, so no long-lived compatibility
  guarantee is needed. A version mismatch may discard pending proposals.
- **Change policy:** `manifest_version` in each manifest. On mismatch, the proposal is
  reported as unreadable and offered for deletion rather than being applied.

## Definitions

- **Proposal** — a set of related structural mutations produced by one MCP authoring call,
  staged for review as a unit.
- **Operation** — one mutation within a proposal, scoped to a single node or relationship.
  The unit of accept/reject.
- **Base hash** — the content hash of the node as it stood when the operation was computed.
  The mechanism for detecting staleness.
- **Stale** — an operation whose base hash no longer matches the node on disk, because the
  node changed after the proposal was staged.

## Contract Shape (Conceptual)

### Directory layout

```
.arch/.proposals/
└── <proposal-id>/
    ├── manifest.json
    └── nodes/
        ├── <node-id>.json    # the would-be node file
        ├── <node-id>.md      # the would-be prose
        └── ...
```

**Gitignored in its entirety.** A committed proposal would mean unreviewed agent-authored
content entering git under the guise of the model — precisely the failure the review gate
exists to prevent. This is one of the paths the `.gitignore` release gate verifies.

Proposals hold **materialised would-be files** alongside the manifest, rather than only a
patch. The UI can then diff two real files rather than applying a patch in its head, and
applying an accepted operation is a file move rather than a transformation that could fail
halfway.

`<proposal-id>` is time-ordered and readable: `p-<YYYYMMDD>-<HHMM>-<short-random>`, e.g.
`p-20260927-1043-a91f`. Time-ordered so the review list sorts naturally and an abandoned
proposal's age is obvious at a glance.

### `manifest.json`

#### Required fields

- `manifest_version` — integer. `1` for this contract.
- `proposal_id` — matching the directory name.
- `created_at` — ISO 8601 timestamp.
- `source` — which MCP tool produced this (`seed_model`, `create_node`, `update_node`,
  `delete_node`, `set_relationship`, `delete_relationship`, `group_nodes`, `ungroup_node`, `reparent_node`; the last
  three come from the canvas via `POST /proposals/group|ungroup|reparent`).
- `operations` — ordered list of operations.

#### Optional fields

- `summary` — a short human-readable description of the proposal's intent, shown at the top
  of the review panel. Worth populating for `seed_model`, where thirty operations arrive at
  once and the architect needs to know what they are looking at before reading any of them.

#### Operation shape

Every operation carries:

- `op` — one of `create_node`, `update_node`, `delete_node`, `set_relationship`,
  `delete_relationship`, `set_constraints`.
- `node_id` — the node this operation concerns. For `create_node`, the caller-supplied `id`
  when one was given (a free slug), otherwise the ID `ambit-core` derived from the proposed
  name. For `set_relationship` and `delete_relationship`, the `from` node — relationships
  are reviewed as part of their source node, so the architect sees an edge in the context of
  the thing that owns it rather than as a free-floating item.
- `base_hash` — the content hash of the node as it stood when this operation was computed.
  `null` for `create_node`, which has no prior state.
- `status` — `pending`, `accepted`, or `rejected`. Mutated in place as the architect
  reviews, so a partially-reviewed proposal survives a page reload or a restart.

`update_node` additionally carries `fields`, the list of field names it changes, so the UI
can summarise an operation without diffing to find out what moved.

`set_relationship` additionally carries the relationship itself: `from`, `to`, `label`,
`kind`, and optionally `contract` (the edge contract: `format`, `ref`, `text`). An absent
`contract` leaves an existing edge's contract unchanged; an empty one (`{}`) clears it.
Limits are those of the model format and are checked at staging. `rel_covered: true` records
that `base_hash` also covers the relationship's own JSON (see Staleness).

`set_constraints` replaces the project `constraints` list in `config.json` (full list, not a
patch). It carries `constraints` (the validated list; empty clears) and no node: `node_id` is
empty. Its `base_hash` is the sha256 of the `config.json` bytes, so any concurrent edit to
the file makes it stale. At most one `set_constraints` may appear in a proposal. Accepting
applies the list with the same validation as the direct `SetConstraints` call.

`create_node` and `update_node` carry the node's `depends_on` and `contract` inside the
staged node file (decision 0012). `update_node` lists `depends_on` and `contract` in
`fields` when they change. Dependencies are validated at staging against the model as
earlier operations of the same proposal leave it, so a cycle formed across two operations is
rejected with `ErrDependencyCycle`; they are validated again at accept time against the
canonical model. A staged `delete_node` is rejected with `ErrHasDependents` (naming the
dependents) while a node outside the deleted set still lists it in `depends_on`.

Diff snapshots. `NodeSnapshot` gains `depends_on` (sorted ids) and `contract`; `RelSnapshot`
gains `contract`. A `set_constraints` diff row carries `current_constraints` and
`proposed_constraints` (an absent key means an empty list).

`delete_relationship` carries `relationship_id`, plus `from`, `to`, `label`, `kind` of the
edge as it stood when staged. Its `base_hash` covers the `from` node's hash, the hashes of
any interior nodes, and the edge's own JSON. Applying it runs `DeleteRelationship`, which also removes
any nodes inside the edge (relationship interior). A proposal can only delete an edge that
exists in the canonical model at staging time.

`delete_node` carries nothing further.

#### Caller-supplied node ids

A `create_node` operation may carry an explicit id. It must match the slug pattern
(`^[a-z0-9]+(-[a-z0-9]+)*$`) and is rejected (`ErrExists`) when a node already has it, when
an earlier operation in the same proposal creates it, or when a pending `create_node` in any
other staged proposal reserves it. Within a proposal, later operations may name a staged node
in `parent_id` or as a relationship endpoint. Referencing a node staged in a *different*
proposal is not supported: such a parent or endpoint must exist in the canonical model at
staging time.

#### Staged-create dependencies

An operation that depends on a `create_node` in the same proposal (a create or update whose
`parent_id` names it, or a `set_relationship` with it as an endpoint) cannot be accepted
until that create is accepted. A single-operation accept returns a stale error; accept-all
skips it and reports `container create_node was not accepted` or `edge endpoint create_node
was not accepted` (for example when the create was rejected).

### Accept and reject semantics

**Granularity is per node.** The architect accepts or rejects each operation individually,
with accept-all and reject-all shortcuts. Per-node rather than atomic because `seed_model`
against a real transcript produces the whole initial model at once, and an all-or-nothing
gate on thirty nodes trains the user to click accept without reading — which is worse than
no gate, because it looks like review.

**Accept-all is all-or-nothing for failures.** If any operation fails validation while
accept-all applies a proposal, every operation that call already applied is rolled back
(node files, `index.json`, and the manifest are restored), the response reports no applied
operations and the failing one under `failed`, and the remaining operations are not
attempted. Stale operations are different: they are still skipped and reported under
`skipped` while the rest apply, as described under Staleness. Cached layouts invalidated by
rolled-back operations are not restored.

**Accepting an operation is atomic.** The node's `.json` and `.md` move from the proposal
into `.arch/nodes/`, and `index.json` is updated, as one unit. A node never lands with its
prose missing, and `index.json` never disagrees with what is on disk.

**Rejecting marks the operation `rejected`; it does not delete the proposal.** The architect
may accept some operations and reject others in the same sitting, and may leave and return.

**A proposal is removed once every operation is resolved** — all accepted, all rejected, or
a mix. Nothing in v1 expires an abandoned proposal with operations still `pending`.

### Staleness

Between an agent staging a proposal and the architect reviewing it, the architect may have
edited the same node in the UI. Architect edits write directly, so the node on disk has
moved out from under the proposal.

Each operation's `base_hash` records what the agent saw. At review time, `ambit-core`
recomputes the node's current hash. A mismatch marks the operation **stale**.

A relationship operation's `base_hash` also covers the relationship's own JSON (decision
0012, item 7), so two agents editing one edge's contract cannot silently overwrite each
other. It is the sha256 of the `from` node hash plus the edge JSON when the edge existed on
disk at staging and no earlier operation in the same proposal touched it (`rel_covered`);
otherwise it is the `from` node hash alone, so a proposal that creates or repeatedly edits an
edge does not go stale against itself. Proposals staged before this field existed have
`rel_covered` absent and keep the old from-node-only hash.

**Stale operations are flagged and require explicit confirmation, not auto-rejected.**
Auto-rejecting would force the agent to redo work whenever the architect touched anything
nearby, including changes that do not actually conflict. Flagging leaves the judgment where
the whole posture says it belongs. The UI must make the stale state visually distinct and
must not let an accept-all shortcut sweep a stale operation through silently.

## Invariants (Must Always Hold)

1. **Nothing in `.arch/.proposals/` is ever committed.** Verified at runtime by
   `git check-ignore`.
2. **No MCP authoring tool writes to `.arch/nodes/`.** Structural mutation reaches the
   canonical model only through an accepted operation.
3. **Accepting is atomic per node.** `.json`, `.md`, and the `index.json` update land
   together or not at all.
4. **A stale operation cannot be applied without explicit confirmation**, including via
   accept-all.
   Accept-all never leaves the model half-applied after a validation failure (see
   Accept and reject semantics).
5. **Operation `status` is persisted in the manifest**, so partial review survives a
   restart.
6. **Applying an accepted operation goes through `ambit-core`'s normal validation**, the
   same path a UI edit takes. Staging is a delay, not a bypass — a proposal that would
   create a hierarchy cycle is rejected at apply time even though it was staged.
7. **A proposal directory is self-contained.** It never references files outside itself
   except by node ID.

## Error Handling

- **Unreadable or version-mismatched manifest** — the proposal is listed as unreadable with
  its ID and age, and offered for deletion. It is never partially applied.
- **Proposal references a node deleted since staging** — the operation is marked stale;
  `update_node` and `delete_node` against a missing node cannot be applied and are reported
  as such.
- **Apply-time validation failure** — the operation is reported with the reason and left
  `pending`. The rest of the proposal is unaffected.
- **Missing `.md` for a proposed node** — proposal integrity error. Not applied with empty
  prose, since silently creating an empty spec would produce a node that `get_context`
  cannot brief an agent from.
- **Two pending proposals touching the same node** — both are shown, both carry their own
  base hash. Accepting one makes the other stale, which the flagging mechanism handles
  without any special case.

## Examples

### Minimal valid manifest

```json
{
    "manifest_version": 1,
    "proposal_id": "p-20260927-1043-a91f",
    "created_at": "2026-09-27T10:43:12Z",
    "source": "create_node",
    "operations": [
        {
            "op": "create_node",
            "node_id": "refunds-service",
            "base_hash": null,
            "status": "pending"
        }
    ]
}
```

### A `seed_model` proposal, partially reviewed

```json
{
    "manifest_version": 1,
    "proposal_id": "p-20260927-0915-3c2d",
    "created_at": "2026-09-27T09:15:44Z",
    "source": "seed_model",
    "summary": "Top-level model from the Acme onboarding call: platform, orders, payments, notifications.",
    "operations": [
        {
            "op": "create_node",
            "node_id": "platform",
            "base_hash": null,
            "status": "accepted"
        },
        {
            "op": "create_node",
            "node_id": "orders-service",
            "base_hash": null,
            "status": "accepted"
        },
        {
            "op": "update_node",
            "node_id": "payments-service",
            "base_hash": "sha256:4f1a9c...",
            "fields": ["implementation", "spec"],
            "status": "pending"
        },
        {
            "op": "set_relationship",
            "node_id": "orders-service",
            "base_hash": "sha256:8b7e21...",
            "from": "orders-service",
            "to": "payments-service",
            "label": "requests authorisation",
            "kind": "sync",
            "status": "pending"
        }
    ]
}
```

### Invalid example, with expected handling

```json
{
    "manifest_version": 1,
    "proposal_id": "p-20260927-1102-77b0",
    "created_at": "2026-09-27T11:02:00Z",
    "source": "update_node",
    "operations": [
        {
            "op": "update_node",
            "node_id": "payments-service",
            "base_hash": "sha256:0000deadbeef",
            "fields": ["scope"],
            "status": "pending"
        }
    ]
}
```

The `base_hash` does not match `payments-service` on disk — the architect edited the node
after this was staged. Expected handling: the operation renders as **stale**, visually
distinct, showing both what the agent saw and what the node now is. Accepting it requires
explicit confirmation. An accept-all shortcut skips it and says so.

### Operational notes

- Proposals are small: a handful of files and a manifest. `seed_model` is the only realistic
  source of a large one.
- The UI learns about new proposals through the `/events` SSE stream, so a proposal staged
  by an agent appears without a refresh. See
  [`local-http-api.md`](local-http-api.md).
- Nothing garbage-collects abandoned proposals in v1. Whether that needs a
  `ambit proposals prune` command is an open question — see
  [`docs/planning/open-questions.md`](../planning/open-questions.md).

### References

- ADR: [`docs/adr/0002-agent-interface-and-review-gate.md`](../adr/0002-agent-interface-and-review-gate.md)
- Canonical spec: [`docs/planning/spec-v1.md`](../planning/spec-v1.md) Section 9
- Related contracts: [`mcp-tools.md`](mcp-tools.md),
  [`local-http-api.md`](local-http-api.md),
  [`arch-model-format.md`](arch-model-format.md)
