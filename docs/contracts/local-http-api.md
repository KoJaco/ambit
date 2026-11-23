# Contract: Local HTTP API

## Purpose

Defines the localhost HTTP surface that `ambit start` serves. It is the boundary between the
Go core and the browser SPA: graph reads scoped to a drill-down level, graph mutations,
layout cache persistence, proposal review, and a change-notification stream.

Consumer: the embedded React Router SPA, and nothing else. This is not a public API and is
not versioned for third parties.

## Scope

**In scope:** endpoint shapes, the drill-down query contract, the SSE stream, and the
security posture of a localhost-only server.

**Out of scope:** the MCP surface (see [`mcp-tools.md`](mcp-tools.md)), the model file
format (see [`arch-model-format.md`](arch-model-format.md)), and the proposal format (see
[`proposals.md`](proposals.md)).

## Versioning

- **Current version:** v1
- **Compatibility:** The API and the SPA are compiled into the same binary via `go:embed`
  and always ship together, so there is no version skew to manage between them.
- **Change policy:** Changes freely alongside the frontend. Any external consumer would be
  depending on an internal boundary.

## Definitions

- **Drill-down level** — a node's direct children plus the relationships among them. The
  unit of both rendering and layout.
- **Scoped query** — a server-side query returning exactly one level. As opposed to
  returning the whole graph and filtering in the browser.
- **Layout cache** — computed and manually-adjusted node positions, stored per level in the
  gitignored `.arch/.cache/layout/`.

## Contract Shape (Conceptual)

### Posture

Binds to **localhost only**. `ambit start` listens on `127.0.0.1:8080` unless `--addr` is
set. `--addr` accepts only a loopback IP (`127.0.0.1` or `::1`) and a port. `0.0.0.0`,
other interfaces, and hostnames are rejected before the process listens. No authentication,
because there is no remote caller and no account system — adding auth to a single-user
local process would be ceremony without a threat it addresses.

There is **no LLM proxy endpoint** and no model-provider configuration. ambit makes no
outbound calls to a model provider. This is the largest single deletion from the original
spec's HTTP surface; see [ADR-0002](../adr/0002-agent-interface-and-review-gate.md).

All mutation endpoints call `ambit-core`, the same path MCP takes. No HTTP-specific
validation exists.

### Static assets

The SPA is `go:embed`-ed and served from the root. Unmatched non-API paths fall back to the
SPA's entry document so client-side routes such as `/node/payments-service` resolve on a
hard refresh.

### Graph reads

- **Get a drill-down level** — `GET /levels/{nodeId}` returns the direct children of that
  node plus the relationships among them, plus the node itself for breadcrumb context.
  `GET /levels` is the root: nodes with no `parent_id`, and `node` is null.

  Optional `?expand=a,b` (comma-separated node ids; whitespace and empty entries ignored)
  asks for those groups to be shown open. The client sends the set it read from its layout
  cache. An id is honored only if it exists, is already visible on the level (a child of the
  level or a member of an expanded group), has children, and is at most 2 levels below the
  level. Anything else is ignored silently, and order and duplicates do not matter. The
  members of an expanded group are added to `children` as a flat list, directly after their
  group, so the client nests them by `group_id`. See
  [decision 0010](../decisions/0010-flat-first-groups-and-summary-nodes.md).

  Node summary fields (in `node` and `children`): `id`, `name`, `type`, `status`,
  `protected`, plus optional `icon`, `color`, `renderer`, and `schema` (presentation, see
  decision 0009; omitted when unset). `child_count` is the number of direct children
  (always present). `group_id` is the parent id when the node is a member of an expanded
  group. `expanded` is true on a node shown as an open group. `internal_rel_count` is the
  number of active relationships hidden inside a collapsed node that has children.
  `type_mix` is the node's direct children as `[{"type","count"}]`, sorted by count
  descending then type name (omitted for leaves). Summaries also carry `depends_on` (array of
  node ids, always present, `[]` when none) and `contract` (omitted when none), see decision
  0012.

  **Roll-up.** Let V be the visible nodes on the level. Each relationship endpoint resolves
  to its anchor: the nearest ancestor-or-self in V. For each active relationship:

  - Both anchors exist and differ: an entry in `relationships` between the anchors. When an
    endpoint was rolled up, the relationships for the same anchor pair merge into one edge
    with `aggregated: true`, `id` of the form `agg:{from}>{to}`, `members` (relationship ids,
    sorted), `kinds` (deduped, sorted), `label` `"N relationships"` (`"1 relationship"` for
    one), and `kind` set only when all members share one. `drillable` is true only when
    there is exactly one member and it is drillable. Edges between nodes that are both
    directly visible are not aggregated and keep their own `id`, `label` and `kind`.
  - Both anchors are the same node: hidden, and counted in that node's `internal_rel_count`.
  - Exactly one anchor exists: a crossing. `node_id` is the anchor in V, `direction` is
    `out` or `in`, and `other_id` is the real endpoint id (an id only). Crossings carry
    `label`, `kind`, `relationship_id` and `drillable`. The response does not include the
    other node, and the client must not invent one.

  `view_defaults` (`{"group_max": 8, "level_budget": 24}`) carries the effective settings
  from `config.json` so the client can pick default expanded groups. Edge `aggregated`,
  `members` and `kinds` are omitted on non-aggregated edges.

- **Get a relationship interior** — `GET /levels/relationships/{id}` returns member nodes,
  relationships among them, crossings, endpoint `context`, and warnings. Unknown id or an
  empty interior is 404.

- `PUT /relationships` — create when `id` is omitted (returns the new id), or update
  `label` and `kind` when `id` is set. `from` and `to` are required on create. Optional
  `contract` `{format?, ref?, text?}`: omitted leaves an existing edge's contract unchanged,
  `{}` clears it. Relationship reads (level `relationships`, relationship-level
  `relationship`/`relationships`, and this response) expose `contract` when set and omit it
  otherwise; aggregated edges never carry one. Crossings do not carry it (fetch via the
  relationship id).

- `DELETE /relationships/{id}` — removes the edge and cascades interior members.

  Layout cache key for a relationship level is `_rel_{id}` (not the route path).

  **This endpoint must never return the whole graph.** It is the one genuinely
  performance-relevant decision in the architecture: React Flow is fed a level, and
  filtering a full graph in the browser is the thing being avoided. See
  [`docs/architecture/frontend.md`](../architecture/frontend.md).

- **Get a single node** — `GET /nodes/{id}` returns the structured fields and the
  markdown. Unknown id is 404 and the message names the id.

- **Get model integrity warnings** — `GET /integrity` returns orphans and dangling
  references. The same list rides on `GET /levels` and `GET /levels/{nodeId}` as
  `warnings`. Warnings never block a read. Each warning names the node or the file.

- **Get project constraints** — `GET /constraints` returns
  `{"constraints": [{"id","text","severity"}]}` in file order (`[]` when none).

- **Get the build order** — `GET /build-order` (optional `?scope=<nodeId>`, unknown id is 404)
  returns `{"scope"?, "order": [{"id","name","status","done","wave","depends_on","blocked_by"}],
  "waves": [[id]], "cycles": [[id]], "unordered": [id]}`. Advisory, read-only. See decision
  0012.

- **List icons** — `GET /icons` returns `{"icons": [{"id", "label"}]}`, the closed icon
  registry in display order with a one-line label each.

### Graph mutations

These **write directly** to the canonical model through `ambit-core`. Architect edits made
in the UI need no review gate — the human is already in the seat. This is the asymmetry
with MCP, where the same operations are staged.

- `POST /nodes` — create. The id is derived from the name unless the body carries an optional `id`
  (a valid slug), which is used as-is; an id already in use is rejected like any
  other create conflict. Undo of a node delete relies on this to restore the same id.
- `PATCH /nodes/{id}` — update. `status` is a normal field. Omitted fields are left
  unchanged.
- `POST`/`PATCH /nodes` bodies also take `depends_on` (array of node ids) and `contract`
  (`{payloads?, invariants?, errors?, owns_data?}`). On PATCH, omitted means unchanged,
  `[]` / `{}` clears. Node responses and level summaries return `depends_on` (always an
  array) and `contract` (omitted when none). A dependency cycle or deleting a node that
  others depend on is 409; an invalid contract or unknown/duplicate/self dependency is 400.
- `PUT /constraints` — body `{"constraints": [{"id","text","severity"?}]}` replaces the whole
  list directly (the architect's own surface; agents stage `set_constraints` instead).
  Validates (unique slug ids, text 1..300, severity `must`|`should` defaulting to `must`, at
  most 24; failures are 400) and returns the stored list as `GET /constraints` does.
- `DELETE /nodes/{id}` — delete.
- `PUT /relationships` — set the directed edge `from` → `to`, with `label` and `kind`.

A validation failure is 409 (or 400 for `ErrBadOperation`, bad contract, bad constraint, bad
dependency) and the message is the core error, which names the node and
the rule. There is no HTTP-specific validator.

- `GET /assignment` — the active assignment, always 200. `{"node_id": "<id>",
  "assigned_at": "<RFC3339>"}` when set, `{"node_id": null}` when none. If the stored node
  no longer exists the assignment is returned as stored plus `"dangling": true` (never 404;
  `ambit check` treats it as a warning too).
- `PUT /assignment` — body `{ "node_id", "assigned_at"? }`. Calls `SetAssignment`. Stored
  only in `local.json`. Does not change the node's `status`. An unknown node is 404.
  Node summaries in `/levels*` and `GET /nodes/{id}` carry `"assigned": true` on the
  assigned node only (omitted otherwise). It is a view field computed from `local.json`,
  not stored in node files. A `local.json` change emits `model-changed` (with no node ids),
  so clients refetch `GET /assignment`.
- `DELETE /assignment` — calls `ClearAssignment`. Leaves every node file alone.

### Layout

- `GET /layout/{key}` — cached positions for one level, or `{"positions":[]}` when that
  file is missing. A missing cache is not an error.
- `PUT /layout/{key}` — stores `{ "positions": [{ "id", "x", "y" }] }` under
  `.arch/.cache/layout/<key>.json`.

`{key}` for `/node/:nodeId` is that node id. `/` has no node id; its key is `_root`.
When groups are open on a level (`?expand=`), the client appends `~<hash>` to that key, where
`<hash>` is the 8-hex-digit FNV-1a hash of the sorted expanded ids (nothing is appended when no
group is open). Each expanded set therefore has its own file, and positions of group members
are relative to their group. The `~` suffix is the client's convention; the server treats the
key as opaque.

Create, delete, and reparent remove the base key and every `<key>~*` variant file for that
level (exact prefix match `<key>~` inside the layout directory only). The client still uses a
cached layout only when it has a position for every visible node and no others, as a safety net.

The stored `Layout` is typed (`positions`, `ports`, `attachments`) and has no field for view
modes, so the group/summary view modes are not stored here. The client keeps them in browser
`localStorage` per base level key.
`_root` cannot collide with a node id, because ids match `^[a-z0-9]+(-[a-z0-9]+)*$`.

Layout writes touch only `.arch/.cache/layout/`. They never write to the canonical model,
which has no coordinates by design. Create, delete, and reparent delete that level's cache
file and its `~<hash>` variants so the client recomputes. A field edit does not.

### Proposals

- **List proposals** — pending proposals with their operations and per-operation staleness
  computed against current state. Staleness is computed server-side at read time, not stored
  in the manifest, because the node can change after the manifest is written.
- **Get a proposal diff** — for each operation, the current state and the proposed state,
  ready to render. A `set_relationship` row's `relationship` carries `contract_cleared: true`
  (omitted otherwise) when the staged contract is explicitly empty and clears the edge's
  contract.
- **Accept an operation** — applies it atomically. Requires an explicit confirmation flag
  when the operation is stale.
- **Reject an operation** — marks it rejected.
- **Accept-all / reject-all within a proposal** — convenience over the per-operation
  endpoints. Accept-all **skips stale operations** and reports which were skipped; it must
  never sweep a stale operation through.
- **Delete a proposal** — discards it wholesale.
- **Stage a group / ungroup / reparent proposal** — canvas structure edits never write
  silently; each stages exactly one proposal (multi-operation, reviewed like any other) and
  answers `201` with the same body as `GET /proposals/{id}`. Nothing in `.arch/nodes/`
  changes until the operations are accepted.
  - `POST /proposals/group` `{ "node_ids": ["a","b"], "name": "Backend", "type"?: "container",
    "icon"?: "…", "color"?: "…" }`. Source `group_nodes`. Operations: `create_node` for the
    container (id derived from the name like `create_node`, so a collision gets `-2`; parent
    is the selection's shared parent or root; status `draft`; no renderer), then one
    `update_node` (`fields: ["parent_id"]`) per selected node. Errors: `400` fewer than 2
    nodes, duplicates, different parents, an ancestor/descendant pair, a node inside a
    relationship; `404` unknown id; `409` empty name; invalid icon/color is `400`.
  - `POST /proposals/ungroup` `{ "node_id": "container", "delete_container"?: false }`.
    Source `ungroup_node`. Operations: one `update_node` per direct child moving it to the
    container's parent (or root, or the container's relationship), then, only when
    `delete_container` is true, `delete_node` for the container. Default keeps the container
    as a leaf. `400` if the container has no children, or if `delete_container` is set while
    relationships are attached (deleting a node drops its relationships; keep the container
    or remove them first). Relationships reference node ids and are never rewritten.
  - `POST /proposals/reparent` `{ "node_id": "x", "new_parent_id": "y" }` (`""` = root).
    Source `reparent_node`. A single `update_node` (`parent_id`). `400` if already there,
    `409` on a cycle or unknown parent, `404` unknown node. Used when a node is dragged out of a group.

### Events (SSE)

A single one-way stream at `/events`, driven by `fsnotify` in the Go process.

Event kinds, sent as SSE `event:` names. The client cannot send on this stream.
Proposal events are not emitted yet.

- `model-changed` — a node file, `index.json`, or `local.json` changed on disk, whether from a UI write,
  an MCP status write, or the architect's editor. `data` is
  `{ "node_ids": ["…"], "relationship_ids": ["…"] }`.
- `proposals-changed` — a proposal was staged, resolved, or deleted. Not emitted until the
  review gate. A client built now must ignore event names it does not handle.
- `integrity-changed` — the warning set changed.

SSE rather than WebSocket because the traffic is strictly one-directional, the browser has
the HTTP API for anything it needs to send, and SSE reconnects automatically where a
WebSocket needs reconnection logic written and debugged. See
[ADR-0003](../adr/0003-runtime-and-distribution.md).

### Startup checks

On `ambit start`, before serving: verify via `git check-ignore` that `local.json`,
`.arch/.cache/`, and `.arch/.proposals/` are actually ignored. Warn loudly if not. This
covers `init` having run before `git init`, a pre-existing `.gitignore` merging oddly, or an
entry being edited out later.

## Invariants (Must Always Hold)

1. **Binds to localhost only.** Never `0.0.0.0`.
2. **The level endpoint returns one level.** Never the full graph.
3. **All mutations go through `ambit-core`.** No HTTP-specific write or validation path.
4. **Layout writes never touch canonical files.**
5. **A stale proposal operation cannot be accepted without an explicit confirmation flag**,
   and accept-all skips rather than sweeps.
6. **Accepting an operation is atomic** — `.json`, `.md`, and `index.json` together.
7. **The SSE stream is one-way.** The client never sends over it.
8. **Unmatched non-API routes fall back to the SPA document**, so client routes survive a
   refresh.
9. **The ignore check runs before serving**, every start.

## Error Handling

Conventional HTTP semantics: `404` for an unknown node or proposal, `400` for a malformed
request, `409` for a validation conflict such as a hierarchy cycle or accepting a stale
operation without confirmation, `500` for an unreadable or malformed model.

Every error body carries a human-readable message naming the specific node, operation, or
rule involved. These surface directly in the architect's UI, so "invalid request" is not an
acceptable message.

Model integrity warnings are **not** errors. They ride alongside successful responses; a
model with a dangling reference still loads and still renders.

## Examples

### Minimal valid example

A drill-down level response for `platform`:

```json
{
    "node": { "id": "platform", "name": "Platform", "type": "boundary" },
    "children": [
        { "id": "orders-service", "name": "Orders Service", "type": "service", "status": "done" },
        { "id": "payments-service", "name": "Payments Service", "type": "service", "status": "assigned" }
    ],
    "relationships": [
        {
            "from": "orders-service",
            "to": "payments-service",
            "label": "requests authorisation",
            "kind": "sync"
        }
    ],
    "crossings": [
        {
            "node_id": "orders-service",
            "direction": "out",
            "label": "settles",
            "kind": "async",
            "other_id": "billing"
        }
    ],
    "warnings": []
}
```

Only the children of `platform` and only the relationships among those children. A
relationship from `orders-service` to a node outside this level is a `crossings` entry.
`other_id` names the far endpoint. It is not a node in this response, and the client
renders a boundary marker instead of inventing that node. `GET /levels` uses the same
shape with `"node": null`. Each node object includes `status` and `protected`. Markdown,
`implementation`, and `scope` are not on this response.

### Invalid example, with expected handling

Accepting a stale operation without confirmation:

```json
{
    "proposal_id": "p-20260927-1102-77b0",
    "operation": "update_node:payments-service"
}
```

Expected handling: `409`, with a message stating the operation is stale, naming the node,
and stating that `confirm_stale` is required to apply over the architect's newer edit. No
write occurs.

### Operational notes

- Everything is served from an in-memory index, so reads are effectively free at this scale.
- The SSE stream is the only long-lived connection.
- A second `ambit start` on the same project is a supported-but-untested state; both
  processes would watch the same files. v1 has no cross-process locking — see
  [ADR-0003](../adr/0003-runtime-and-distribution.md).

### References

- ADR: [`docs/adr/0003-runtime-and-distribution.md`](../adr/0003-runtime-and-distribution.md)
- Canonical spec: [`docs/planning/spec-v1.md`](../planning/spec-v1.md) Section 9
- Related contracts: [`proposals.md`](proposals.md),
  [`arch-model-format.md`](arch-model-format.md)
- Frontend behaviour: [`docs/architecture/frontend.md`](../architecture/frontend.md)
