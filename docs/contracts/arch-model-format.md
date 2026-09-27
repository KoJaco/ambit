# Contract: `.arch` Model Format

## Purpose

Defines the on-disk format of the ambit architecture model. This is the boundary between
`ambit-core` and everything that reads the model: git, the architect's editor, the HTTP API,
the MCP server, and any future tool. It is the most durable contract in the product — the
model outlives any particular version of ambit.

Consumers: `ambit-core` (read/write), the architect (hand-editing and `git diff` review),
and by extension the HTTP API, MCP server, and `ambit check`.

## Scope

**In scope:** directory layout, node file fields, `index.json` structure, `config.json`,
`local.json`, ID rules, versioning, and the invariants that must hold across all of them.

**Out of scope:** the derived layout cache (presentation only, see
[`docs/architecture/frontend.md`](../architecture/frontend.md)), the proposal staging format
(see [`proposals.md`](proposals.md)), and the wire formats of the HTTP and MCP surfaces.

## Versioning

- **Current version:** v1
- **Compatibility:** Additive only within v1. New optional fields may appear; existing
  fields may not change meaning or type.
- **Change policy:** `schema_version` lives in exactly one place, `index.json`. Node files
  inherit it and do not carry their own. A breaking change bumps it and requires a migration
  path; an additive change does not bump it.

## Definitions

- **Node** — a unit of architecture: a service, boundary, data store, or anything else the
  architect chooses to model. Nodes nest via `parent_id`.
- **Drill-down level** — a node's direct children plus the relationships among them. The
  unit of both rendering and layout.
- **Cross-cutting relationship** — a directed edge between two nodes that is *not* a
  hierarchy edge. These jump across the tree (`orders → payments`) and are stored separately
  from hierarchy for that reason.
- **Implementation globs** — the file patterns a node's code occupies. Used by
  `ambit check` to map a diff back to nodes.
- **Scope** — the file patterns an agent assigned to a node may modify. Defaults to the
  node's `implementation` when empty.
- **Protected** — a node no agent may modify under any circumstances.
- **Canonical** — committed to git, a source of truth. As opposed to **derived**, which is
  gitignored and rebuildable.

## Contract Shape (Conceptual)

### Directory layout

```
.arch/
├── nodes/
│   ├── <node-id>.json       # canonical — structured fields
│   ├── <node-id>.md         # canonical — prose spec
│   └── ...
├── index.json               # canonical — version, hierarchy, relationships
├── config.json              # canonical — non-secret project config
├── local.json               # GITIGNORED — machine-local state
├── .cache/                  # GITIGNORED — derived, disposable
│   └── layout/<node-id>.json
└── .proposals/              # GITIGNORED — staged, unreviewed mutations
    └── <proposal-id>/
```

Everything under `nodes/`, plus `index.json` and `config.json`, is committed. Everything
else is not. See "Invariants" below — this is enforced, not merely documented.

### Node file — `nodes/<node-id>.json`

#### Required fields

- `id` — the node's stable identifier, matching its filename. Immutable after creation.
  Lowercase slug: `[a-z0-9]+(-[a-z0-9]+)*`. Derived from `name` at creation, with a numeric
  suffix on collision (`orders-service-2`).
- `name` — human-readable display name. Mutable; renaming never changes `id`.
- `type` — free-form string describing what kind of thing this is (`service`, `vpc`,
  `data store`, whatever the architect writes). No enum, no registry, no validation. The UI
  matches known strings to icons on a best-effort basis and falls back to a generic shape.
  An unrecognised type is never an error.
- `status` — one of `draft`, `specified`, `assigned`, `in_progress`, `done`, `blocked`.

#### Optional fields

- `parent_id` — the ID of this node's parent. Absent or `null` for a root-level node.
- `implementation` — list of glob patterns for the files this node's code occupies. Absent
  or empty means the node has no code yet, which is normal for a node in `draft`.
- `scope` — list of glob patterns an assigned agent may modify. **When absent or empty, it
  defaults to `implementation`.** Set it explicitly only when an agent needs access beyond,
  or narrower than, the node's own files.
- `protected` — boolean, default `false`. When `true`, no agent may modify files matching
  this node's `implementation`, regardless of assignment.

#### Reserved, not implemented in v1

Documented so the format need not change shape later. No v1 code reads or writes these, and
none should be added without a corresponding ADR.

- `observed_implementation` — where the node's code was actually found, as distinct from
  where it was declared to be.
- `agent_authority` — a richer permission model than the `protected` and `scope` pair.

### Node prose — `nodes/<node-id>.md`

Plain markdown. The node's written specification: what it does, its interfaces, its
constraints. This is the substance of what `get_context` hands to an assigned agent.

No frontmatter, no required headings, no schema. It is prose, and the format deliberately
does not constrain it.

### `index.json`

#### Required fields

- `schema_version` — integer. `1` for this contract.
- `nodes` — list of node IDs present in the model. The authoritative membership list; a
  file in `nodes/` not listed here is an orphan (see "Error Handling").
- `relationships` — list of cross-cutting relationships.

#### Relationship shape

- `from` — source node ID. Required.
- `to` — target node ID. Required.
- `label` — free-form string describing the relationship (`"places order"`,
  `"reads from"`). Optional but strongly encouraged; an unlabelled edge tells a reader
  almost nothing.
- `kind` — one of `sync`, `async`, `data`. Optional. Used only to style the rendered edge.

Relationships are **directed**. `from → to` is not the same as `to → from`, and both may
exist independently.

Hierarchy is *not* stored here as edges — it lives in each node's `parent_id`. `index.json`
holds the membership list and the cross-cutting graph only.

### `config.json`

Committed, non-secret, project-level configuration. Deliberately minimal in v1. It exists so
there is an established place for settings to land rather than having to be introduced
later.

There is **no LLM or model provider configuration**, and no API key field anywhere in the
format. ambit does not call a model provider. See
[ADR-0002](../adr/0002-agent-interface-and-review-gate.md).

### `local.json`

**Gitignored.** Machine-local state that is meaningless on another machine and must never
be committed.

- `assignment` — the currently assigned node, if any: the node ID and when it was assigned.
  `ambit check` reads this to know whose `scope` applies to the current diff. Absent means
  no assignment is active, which changes how unmapped files are treated — see
  [`docs/architecture/enforcement-model.md`](../architecture/enforcement-model.md).
- UI preferences and similar machine-local state.

## Invariants (Must Always Hold)

1. **No coordinates in canonical files.** Layout positions never appear in `nodes/*.json` or
   `index.json`. Visual layout is a presentation concern; it lives in
   `.arch/.cache/layout/`. This omission is deliberate and must not be "corrected" — a node
   moving on screen is not an architectural change and must never appear in a diff.
2. **`id` matches filename and is immutable.** `nodes/orders-service.json` has
   `"id": "orders-service"`. Renaming a node changes `name` only.
3. **Every `.json` has a matching `.md`, and vice versa.** The pair is created, deleted, and
   moved together.
4. **`parent_id` references an existing node and the hierarchy is acyclic.** A node is not
   its own ancestor.
5. **Relationship endpoints reference existing nodes.**
6. **`local.json`, `.cache/`, and `.proposals/` are ignored by git.** Verified at runtime
   with `git check-ignore` by both `ambit start` and `ambit check`, not merely written into
   `.gitignore` once at init.
7. **Empty or absent `scope` means `implementation`.** Consumers must apply this default
   rather than treating empty scope as "nothing permitted".
8. **`protected` overrides assignment.** A protected node's files are off-limits even to an
   agent explicitly assigned to that node.
9. **All writes go through `ambit-core`.** No consumer implements its own mutation or
   validation path. This is what keeps the UI and MCP from diverging on what a valid
   mutation is.
10. **An unknown `type` never fails anything.** Not a validation error, not a render crash.

## Error Handling

- **Unknown `type`** — rendered with a generic fallback shape. No warning, no error. This is
  expected behaviour, not degradation.
- **Unknown field in a node file** — preserved on round-trip and otherwise ignored. This is
  what lets the reserved fields exist safely and lets a newer ambit's output survive an
  older ambit reading it.
- **Orphaned file** (a `.json` in `nodes/` absent from `index.json`, or a `.md` with no
  `.json`) — reported as a model integrity warning. Never silently deleted and never
  silently adopted; both are destructive guesses about the architect's intent.
- **Dangling reference** (`parent_id` or a relationship endpoint naming a missing node) —
  reported as a model integrity warning. The node still loads; the dangling edge is dropped
  from the rendered graph.
- **Hierarchy cycle** — rejected at mutation time by `ambit-core`, so it cannot be created
  through the HTTP API or MCP. If found on load (hand-edited), reported as an error and the
  offending edge is not traversed.
- **Malformed JSON** — hard error naming the file. The model does not partially load.
- **Ignore entries missing or ineffective** — `ambit start` and `ambit check` warn loudly.
  This is the release-gate condition described in
  [`docs/planning/spec-v1.md`](../planning/spec-v1.md) Section 2.

## Examples

### Minimal valid node

`.arch/nodes/orders-service.json`:

```json
{
    "id": "orders-service",
    "name": "Orders Service",
    "type": "service",
    "status": "draft"
}
```

### Fully populated node

`.arch/nodes/payments-service.json`:

```json
{
    "id": "payments-service",
    "name": "Payments Service",
    "type": "service",
    "parent_id": "platform",
    "status": "assigned",
    "implementation": ["src/payments/**", "migrations/payments/**"],
    "scope": ["src/payments/**", "migrations/payments/**", "src/shared/money.ts"],
    "protected": false
}
```

`scope` is set explicitly here because the assigned agent legitimately needs to touch a
shared module that is not part of this node's own implementation.

`.arch/nodes/payments-service.md`:

```markdown
# Payments Service

Owns payment authorisation and capture. Does not own refunds — see `refunds-service`.

## Interfaces

- `POST /payments/authorise` — takes an order ID and amount, returns an authorisation token.
- Emits `payment.captured` on the platform event bus.

## Constraints

- Must never log a full card number or CVV.
- All monetary values use the shared `Money` type; do not introduce a second representation.
```

### Minimal valid `index.json`

```json
{
    "schema_version": 1,
    "nodes": ["platform", "orders-service", "payments-service"],
    "relationships": [
        {
            "from": "orders-service",
            "to": "payments-service",
            "label": "requests authorisation",
            "kind": "sync"
        }
    ]
}
```

### Invalid example, with expected handling

```json
{
    "id": "orders-service",
    "name": "Orders Service",
    "type": "service",
    "parent_id": "platfrom",
    "status": "draft"
}
```

`parent_id` is a typo for `platform` and names no existing node. Expected handling: the node
loads and renders at root level, and a model integrity warning names both the node and the
dangling reference. The file is not modified and the reference is not guessed at.

### Operational notes

- The whole model is read at startup — N file reads for N nodes — and held in memory
  thereafter. At dozens to low hundreds of nodes this is immaterial and happens once.
- Writes are per-file and small. Two independent processes (`ambit start` and `ambit mcp`)
  may both write; v1 has no cross-process lock, relying on writes being scoped to distinct
  files and infrequent. Recorded as a known thin spot in
  [ADR-0003](../adr/0003-runtime-and-distribution.md).
- File watching drives index rebuilds, so hand-edits in an editor are picked up without a
  restart.

### References

- ADR: [`docs/adr/0001-canonical-model-storage.md`](../adr/0001-canonical-model-storage.md)
- Canonical spec: [`docs/planning/spec-v1.md`](../planning/spec-v1.md) Sections 2 and 3
- Related contracts: [`proposals.md`](proposals.md), [`mcp-tools.md`](mcp-tools.md)
- Enforcement semantics:
  [`docs/architecture/enforcement-model.md`](../architecture/enforcement-model.md)
