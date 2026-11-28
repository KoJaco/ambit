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
- **Decision 0012** added the optional node fields `contract` and `depends_on`, the
  relationship field `contract`, and the `config.json` key `constraints`. Additive, so
  `schema_version` stays 1; an older ambit keeps the new keys in its unknown-field bag.

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

## Modeling guidance (for architects and authoring harnesses)

Normative for humans and for MCP **authoring** tools (`seed_model`, `create_node`,
`set_relationship`, etc.). **Not** part of the default `get_context` brief for coding agents
implementing product code — see [decision 0008](../decisions/0008-arch-graph-modeling-guidance.md).

### Two mechanisms — do not merge them

| Mechanism | Use for | Stored as |
| --- | --- | --- |
| **`parent_id`** | Containment, module trees, drill-down boundaries | Hierarchy in `index.json` + node files |
| **Relationships** | Runtime or logical flow between peers at a level (calls, delegates, hosts, stages, HTTP) | `index.json` `relationships` |

A sibling pair with no relationship edge is valid when hierarchy already tells the story
(e.g. two API modules under one service). Add a relationship when you would explain **how
data or control moves** at that drill-down level.

### Reachability and orphans

- Every node listed in `index.json` should be **reachable from the product root** (typically
  `platform`) via `parent_id`, or be an intentional root listed in `index.json`.
- Orphans and dangling references are integrity warnings — fix them; do not rely on silent
  repair.

### Boundaries and external actors

- Model **in-repo** components under the platform boundary with `implementation` globs where
  they have code.
- Model **environment actors** (browser, coding harness) with `type: external` (or `note`)
  under the platform when inbound flow should appear on the canvas.
- Prefer **one directed edge per channel** (e.g. harness → agent interface for stdio MCP;
  browser → local HTTP API). Return paths and file-based convergence do not require symmetric
  “out” nodes unless you need a separate channel on the diagram.
- Represent **inbound and outbound relationships from outside a container** (users, other
  systems, upstream/downstream services) as `type: external` start/end nodes next to it,
  with one labelled edge per channel, rather than wiring unrelated internals together.
- A container the canvas shows as a **summary** card is a placeholder. Flesh it out into
  child nodes (each with its own implementation globs and spec) before building from it.

### Editing an existing model

- An edge is identified by `(from, to, label)` when no `relationship_id` is given:
  `set_relationship` / `seed_model` on an existing triple updates that edge instead of adding a
  duplicate. Pass `relationship_id` to change an edge's endpoints, label or kind.
- A node id changes only through `rename_node` (or `new_id` on `update_node`), which rewrites
  `parent_id`, `depends_on`, relationship endpoints, the assignment and cached layouts; it is
  rejected when the new id is taken or reserved. Other fields never change an id.
- A `parent_id` move and a `depends_on` change in one proposal are validated against the tree
  as it is after the whole proposal.

### Common patterns

- **Thin entry, fat module:** e.g. CLI/MCP entry node → `hosts` → tool or server module
  (`ambit-mcp-entry` → `mcp-tool-surface`).
- **Service delegate:** subsystem service → `delegates to` → `ambit-core` (and tool surface →
  core when drilling inside agent interface).
- **Cross-level edges:** allowed; the canvas shows a boundary-crossing indicator on the node
  present at the current level (see [frontend.md](../architecture/frontend.md)).
- **Busy levels:** add a child `type: note` (e.g. `*-data-flow`) summarising edges; keep
  `index.json` as the source of truth for relationships.

### What not to do

- Do not use relationships for parent/child containment — use `parent_id`.
- Do not require every node to have a relationship; require **intentional** edges for flows
  you care to review.
- Do not put layout coordinates in canonical files (see invariants below).

Reference graph in this repo: `.arch/nodes/canvas-data-flow.md`,
`.arch/nodes/agent-interface-data-flow.md`, and children under `platform`.

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
├── methods/                 # canonical, optional — user workflow methods (decision 0013)
│   ├── <method-id>.json
│   └── <method-id>.guidance.md   # optional text referenced as {"file": "..."}
├── local.json               # GITIGNORED — machine-local state
├── .cache/                  # GITIGNORED — derived, disposable
│   └── layout/<node-id>.json
└── .proposals/              # GITIGNORED — staged, unreviewed mutations
    └── <proposal-id>/
```

Everything under `nodes/` and `methods/`, plus `index.json` and `config.json`, is committed. Everything
else is not. See "Invariants" below — this is enforced, not merely documented.

### Node file — `nodes/<node-id>.json`

#### Required fields

- `id` — the node's stable identifier, matching its filename. Immutable except through the `rename_node` operation, which rewrites every reference.
  Lowercase slug: `[a-z0-9]+(-[a-z0-9]+)*`. Derived from `name` at creation, with a numeric
  suffix on collision (`orders-service-2`).
- `name` — human-readable display name. Mutable; renaming never changes `id`.
- `type` — free-form string describing what kind of thing this is (`service`, `vpc`,
  `data store`, whatever the architect writes). No enum, no registry, no validation. The UI
  matches known strings to icons on a best-effort basis and falls back to a generic shape.
  An unrecognised type is never an error.
- `status` — one of `draft`, `specified`, `assigned`, `in_progress`, `needs_review`, `done`,
  `blocked`. Only `done` counts as done in the build order.
- `risk` (`low`, `medium`, `high`), `review` (`none`, `required`) and `verify` (array of at
  most 8 commands, each at most 300 characters) — optional review metadata
  ([decision 0015](../decisions/0015-completion-policy-and-review-metadata.md)). `review`
  overrides the method's `completion.review_required` for the node; `verify` is printed in the
  brief and by `check_done`.
- Build-planning fields, all optional, trimmed and bounded on write (over-limit values already on
  disk still load):
  - `external` (bool) — the node is outside the build (a third-party service, a person). It is
    left out of the build order and listed under `externals`; it never blocks. Only this flag
    excludes a node; `type: external` alone does not.
  - `human_gates` (array of at most 16 `{kind, note?}`) — steps only a person can do. `kind` is
    one of `decision`, `credential`, `phone`, `deploy_approval`, `install` (lowercased); `note`
    at most 300 characters. A person clears a gate per user, outside the model (`local.json`
    `gates_cleared`, `PUT /gates`); a node with an uncleared gate is `needs-human` in the build
    order.
  - `deliverables` (at most 32) and `references` (at most 16) — repo-relative paths, each at
    most 300 characters, cleaned and de-duplicated. Absolute paths, `..` segments, drive letters
    and `~` are rejected. `check_deliverables` and `check_done` verify deliverables exist and are
    non-empty; `check_contract_sync` verifies references. A scope glob that names a deliverable
    is not an unmatched glob while the file does not exist yet.
  - `shared` (bool) — the node owns shared manifests (`go.mod`, `package.json`, lockfiles); it
    silences the `shared-manifest` lint for the manifests it owns.
  - `milestones` (at most 16 `{id, label?, status?}`) — `id` a lowercase slug (letters, digits,
    `-` or `_`) of at most 64 characters and unique, `label` at most 200 characters, `status`
    `pending` (default), `in_progress` or `done`. Until all are `done`, `check_done` reports
    `finish_status: needs_review` with `partial: true`.
  - `gates` (array of at most 64 node ids, sorted on write) — on a spike, the nodes that wait
    for it: each listed node effectively depends on the spike (build order, `blocked_by`,
    waves). Each id must exist, may not be the node itself and may not create a dependency
    cycle (checked with `depends_on` and pure containers expanded, like `depends_on`). On load a
    dangling gate id is a warning and a cycle through gates is an error. Renaming a node
    rewrites the ids in `gates`.
- `contract_hash` — optional string. A fingerprint of the node's contract plus the
  contracts of every relationship it is an endpoint of, recorded whenever `status` is set to
  `in_progress`, `needs_review` or `done`. When the current fingerprint differs, the `contract-changed` lint
  rule, `get_build_order` (`contract_changed`) and the brief flag the node
  ([decision 0014](../decisions/0014-drift-and-contract-tracking.md)). Written by ambit;
  do not edit by hand.

#### Optional fields

- `parent_id` — the ID of this node's parent. Absent or `null` for a root-level node.
- `implementation` — list of glob patterns for the files this node's code occupies. Absent
  or empty means the node has no code yet, which is normal for a node in `draft`.
- `scope` — list of glob patterns an assigned agent may modify. **When absent or empty, it
  defaults to `implementation`.** Set it explicitly only when an agent needs access beyond,
  or narrower than, the node's own files.
- `protected` — boolean, default `false`. When `true`, no agent may modify files matching
  this node's `implementation`, regardless of assignment.
- `icon` — closed registry id for canvas glyph (see below). Absent means the UI draws a
  colour dot in the header slot. Unknown ids on disk still load; the canvas falls back to a
  generic circle. HTTP and MCP reject **new** unknown ids on write.
- `color` — closed palette token overriding the UI default derived from `type`. Absent means
  the architect canvas uses the type map below. MCP does not set `color` in v1.
- `renderer` — closed canvas renderer id. Absent or `card` draws the standard node card.
  `schema` draws a database-schema frame; `placeholder` draws a dashed stub; `group` draws a
  labeled frame around that node only (it does not contain children). Unknown ids on disk
  still load and the canvas falls back to `card`. HTTP rejects **new** unknown ids. MCP does
  not set `renderer` in v1.
- `schema` — optional list of `{ "title", "type" }` rows, drawn when `renderer` is `schema`.
  At most 32 rows; each title and type is at most 64 characters. Empty or absent draws the
  schema header only. MCP does not set `schema` in v1.
- `depends_on` — list of node ids this node must be built after. Explicit only; never
  inferred from relationships. Written sorted. Ids must exist, no self-reference, no
  duplicates, and the graph must be acyclic. Unknown keys inside a node or edge `contract` object are preserved on write. A buildable
  container (one with `implementation` globs) counts as done from its children alone; its
  own `status` is ignored for the done rule. Cycles are checked on the expanded graph (a
  dependency on a pure container means its descendants), so `x -> container` with a child
  that depends on `x` is rejected like any cycle. Drives the derived build order
  ([decision 0012](../decisions/0012-greenfield-node-model.md)); advisory, never blocks a
  status change. Within one proposal each operation is cycle-checked as it is staged; a
  move plus a `depends_on` change works in any written order, but swaps only valid in one
  order should be split into separate proposals.
- `contract` — optional object `{ "payloads": string, "invariants": string[], "errors":
  string[], "owns_data": string[] }`; every key optional, empty keys omitted. `payloads` is
  at most 8000 characters; each list has at most 32 items of at most 300 characters. Large
  machine-readable specs (OpenAPI, JSON Schema) are referenced by repo path in text, not
  stored here.

Node file key order on write: `id, name, type, parent_id, relationship_id, implementation,
scope, protected, risk, review, verify, external, human_gates, deliverables, references, shared, milestones, gates, icon, color, renderer, schema, depends_on, contract, contract_hash, status`, then unknown
keys sorted. Over-limit values already on disk still load; new values over a limit are
rejected on write.

**Renderer ids (normative):** `card`, `schema`, `placeholder`, `group`.

Relationship `kind` selects the edge drawing on the canvas (not stored as a separate style):
`sync` is a button label, `async` is an animated path, `data` shows the label and endpoint
names. Any other kind uses the default labeled edge. Edges stay node-to-node.

**Icon ids (normative, 40):** `box`, `hexagon`, `layers`, `component`, `package`, `server`, `cpu`,
`cloud`, `globe`, `workflow`, `database`, `hard-drive`, `archive`, `table`, `file-text`,
`scroll-text`, `book-open`, `sticky-note`, `users`, `shield`, `lock`, `git-branch`, `share-2`,
`network`, `smartphone`, `monitor`, `key-round`, `mail`, `bell`, `message-square`,
`credit-card`, `folder`, `code`, `terminal`, `zap`, `clock`, `settings`, `bot`, `search`,
`cable`. Each has a one-line label served by `GET /icons` and the `list_icons` MCP tool; every
id must exist in lucide-react and in the frontend icon map.

**Colour tokens (normative):** `neutral`, `slate`, `blue`, `teal`, `green`, `amber`, `violet`,
`rose`.

**Type → default colour (non-stored, UI only):** case-insensitive exact match on the whole
`type` string: `service` → `blue`; `module` → `teal`; `component` → `violet`; `package` →
`slate`; `note`, `doc`, `adr` → `amber`; `database`, `cache`, `queue` → `green`; `saas`,
`external` → `rose`; otherwise `neutral`.

Legacy `shape` keys on disk are preserved in the unknown-field bag on round-trip and are not
used by the UI.

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

- `id` — stable slug. Required on write; assigned on load when missing from older files.
- `from` — source node ID. Required.
- `to` — target node ID. Required.
- `label` — free-form string describing the relationship (`"places order"`,
  `"reads from"`). Optional but strongly encouraged; an unlabelled edge tells a reader
  almost nothing.
- `kind` — one of `sync`, `async`, `data`. Optional. Used only to style the rendered edge.
- `contract` — optional object `{ "format": "openapi" | "jsonschema" | "text", "ref":
  string, "text": string }` describing what is on the wire. Stored as opaque text; ambit
  parses no schemas. `text` is at most 4000 characters; `ref` is a repo path of at most 300
  characters and must not contain `..` segments. An unknown `format` already on disk loads
  as-is; new unknown values are rejected on write.

Relationship key order on write: `id, from, to, label, kind, contract`, then unknown keys.

Relationships are **directed**. `from → to` is not the same as `to → from`, and both may
exist independently.

Hierarchy is *not* stored here as edges — it lives in each node's `parent_id`. Node files
may also set `relationship_id` for interior membership (mutually exclusive with
`parent_id`). `index.json` holds the membership list and the cross-cutting graph only.

### `config.json`

Committed, non-secret, project-level configuration. Deliberately minimal in v1. It exists so
there is an established place for settings to land rather than having to be introduced
later.

Recognised keys (all optional; unknown keys are preserved on write):

- `group_max` — positive integer, default `8`. A node with this many children or fewer may
  default to an expanded group on the canvas.
- `level_budget` — positive integer, default `24`. Visible node count on a level above which
  groups default to collapsed summaries.

- `drift_ignore` — array of glob strings, added to the built-in tooling list
  (`.gitignore`, `.github/**`, `go.mod`, `go.sum`, `package.json`, lockfiles, `tsconfig*.json`,
  `LICENSE`, `README.md`, and similar). `drift` does not report matching files as unowned,
  unless some node's `implementation` explicitly matches them
  ([decision 0014](../decisions/0014-drift-and-contract-tracking.md)).
- `constraints` — ordered list of project-wide rules `{ "id", "text", "severity" }`. `id` is
  a unique lowercase slug; `text` is 1 to 300 characters; `severity` is `must` (default
  when absent) or `should`. At most 24 entries on write. An invalid or duplicate entry on
  load is skipped with a warning, not a load error. Global only
  ([decision 0012](../decisions/0012-greenfield-node-model.md)). Unknown keys inside a
  constraint entry are preserved on write. `SetConstraints` (the UI, or an accepted
  `set_constraints`) rewrites `config.json` with its keys sorted alphabetically and the
  constraints array on one compact line; other keys and their values are kept. Saved JSON
  is not HTML-escaped (`List<Order>` stays readable). A running `ambit mcp` reloads when
  `config.json` changes on disk, so rules edited through `ambit start` reach its briefs.

- `method` — id of the active workflow method ([decision 0013](../decisions/0013-workflow-methods.md)).
  Absent means `freeform`. A non-string value, or an id that names no built-in or
  `.arch/methods/` method, is ignored with a warning and the project behaves as `freeform`.
- `method_overrides` — object applied on top of whichever method is active, so one parameter
  can change without a method file: `lint` (rule id to `off|advisory|enforced` or
  `{severity, params}`, for example `{"max-depth": {"severity": "advisory", "params":
  {"max_depth": 4}}}`), `completion` (`agent_may_complete`, `review_required`) and
  `check.include_drift`. Invalid entries warn and are ignored.
- `stage` — id of the current project stage. Only meaningful when the method declares stages;
  an unknown stage uses the method's initial stage (warning), and a stage-less method ignores
  it. Written only by human surfaces (UI, `ambit stage set`, `PUT /stage`), never by an agent
  tool. Changing `method` or `stage` rewrites `config.json` through the same sorted,
  unescaped, atomic writer as `constraints`; unknown keys are kept.

### `methods/<id>.json`

Optional, committed. A user method is a JSON file whose `id` equals its filename stem
(`methods/team.json` has `"id": "team"`). Shape: `schema_version` (1), `id`, `name`,
`description`, `guidance`, `brief_preamble`, `stages` (`[{id, name, preamble}]`),
`initial_stage`, `gates.status` (stage id to refused statuses), `lint` (rule id to
`off|advisory|enforced` or `{severity, params}`), `check.include_drift`, `completion` (`agent_may_complete`, `review_required`), `extends`. Long text
may be `{"file": "x.md"}`, a plain file name beside the method. `extends` names a built-in
method; missing keys are inherited, and `lint` entries merge per rule. A user id equal to a
built-in id shadows it, with a warning. Limits: `guidance` at most 6000 characters, each
preamble at most 1500, at most 8 stages, slug ids. An invalid file is skipped with a warning
and the project uses `freeform`; it is never a load error. Unknown lint rule ids load with a
warning. A running `ambit mcp` or `ambit start` reloads when `config.json` or anything in
`methods/` changes.

The `shared-manifest` lint rule (advisory in `design-first` and `map-existing`, off elsewhere)
lists the files git can see that are named `go.mod`, `go.sum`, `go.work`, `go.work.sum`,
`package.json`, `package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`, `bun.lock`, `bun.lockb`,
`Cargo.toml`, `Cargo.lock`, `uv.lock` or `poetry.lock` that no node's `implementation` globs
match, or that more than one node's match, unless a node with `shared: true` owns the file. It
is silent when git cannot list files.

`group_max` and `level_budget` are view defaults only (decision 0010). A missing key uses the default. A value that is
not a positive integer falls back to the default and produces a warning, not a load error.
The effective values are returned as `view_defaults` on level reads.

There is **no LLM or model provider configuration**, and no API key field anywhere in the
format. ambit does not call a model provider. See
[ADR-0002](../adr/0002-agent-interface-and-review-gate.md).

### `local.json`

**Gitignored.** Machine-local state that is meaningless on another machine and must never
be committed.

- `assignment` — the currently assigned node, if any. Absent means no assignment is
  active, which changes how unmapped files are treated — see
  [`docs/architecture/enforcement-model.md`](../architecture/enforcement-model.md).
  When present it is an object:

  - `node_id` — the assigned node's id.
  - `assigned_at` — RFC 3339 timestamp of when the assignment was recorded.

  ```json
  { "assignment": { "node_id": "payments-service", "assigned_at": "2026-09-27T10:00:00Z" } }
  ```

- `gates_cleared` — array of `"<nodeID>#<index>"` strings: the human gates this user has
  cleared (the index is the gate's position in the node's `human_gates`). Written only by
  `PUT /gates`; entries for nodes or gates that no longer exist are dropped on the next write
  and a node rename keeps them attached. Never an MCP-writable key.
- UI preferences and similar machine-local state. Unknown keys are preserved.

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
- **Dangling `depends_on`** (names a missing node) — model integrity warning. The node still
  loads and the entry is ignored by the build order.
- **Dependency cycle** — rejected at mutation time. If found on load (hand-edited), reported
  as an error diagnostic; the nodes involved get no build order.
- **Deleting a node others `depends_on`** — rejected, naming the dependents.
- **Wrong-typed `depends_on` or `contract`** (node or relationship) — hard error naming the
  file, like other malformed values.
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
