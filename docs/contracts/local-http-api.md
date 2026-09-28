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
  node plus the relationships among those children, plus the node itself for breadcrumb
  context. `GET /levels` is the root: nodes with no `parent_id`, and `node` is null.

  A relationship with one endpoint outside that child set is absent from `relationships`.
  It is listed in `crossings` as `node_id`, `direction` (`out` or `in`), `label`, `kind`,
  and `other_id`. `other_id` is an id string. The response does not include that node, and
  the client must not invent one.

  **This endpoint must never return the whole graph.** It is the one genuinely
  performance-relevant decision in the architecture: React Flow is fed a level, and
  filtering a full graph in the browser is the thing being avoided. See
  [`docs/architecture/frontend.md`](../architecture/frontend.md).

- **Get a single node** — `GET /nodes/{id}` returns the structured fields and the
  markdown. Unknown id is 404 and the message names the id.

- **Get model integrity warnings** — `GET /integrity` returns orphans and dangling
  references. The same list rides on `GET /levels` and `GET /levels/{nodeId}` as
  `warnings`. Warnings never block a read. Each warning names the node or the file.

### Graph mutations

These **write directly** to the canonical model through `ambit-core`. Architect edits made
in the UI need no review gate — the human is already in the seat. This is the asymmetry
with MCP, where the same operations are staged.

- `POST /nodes` — create. The id is derived from the name.
- `PATCH /nodes/{id}` — update. `status` is a normal field. Omitted fields are left
  unchanged.
- `DELETE /nodes/{id}` — delete.
- `PUT /relationships` — set the directed edge `from` → `to`, with `label` and `kind`.

A validation failure is 409 and the message is the core error, which names the node and
the rule. There is no HTTP-specific validator.

- `PUT /assignment` — body `{ "node_id", "assigned_at"? }`. Calls `SetAssignment`. Stored
  only in `local.json`. Does not change the node's `status`.
- `DELETE /assignment` — calls `ClearAssignment`. Leaves every node file alone.

### Layout

- `GET /layout/{key}` — cached positions for one level, or `{"positions":[]}` when that
  file is missing. A missing cache is not an error.
- `PUT /layout/{key}` — stores `{ "positions": [{ "id", "x", "y" }] }` under
  `.arch/.cache/layout/<key>.json`.

`{key}` for `/node/:nodeId` is that node id. `/` has no node id; its key is `_root`.
`_root` cannot collide with a node id, because ids match `^[a-z0-9]+(-[a-z0-9]+)*$`.

Layout writes touch only `.arch/.cache/layout/`. They never write to the canonical model,
which has no coordinates by design. Create, delete, and reparent delete that level's cache
file so the client recomputes. A field edit does not.

### Proposals

- **List proposals** — pending proposals with their operations and per-operation staleness
  computed against current state. Staleness is computed server-side at read time, not stored
  in the manifest, because the node can change after the manifest is written.
- **Get a proposal diff** — for each operation, the current state and the proposed state,
  ready to render.
- **Accept an operation** — applies it atomically. Requires an explicit confirmation flag
  when the operation is stale.
- **Reject an operation** — marks it rejected.
- **Accept-all / reject-all within a proposal** — convenience over the per-operation
  endpoints. Accept-all **skips stale operations** and reports which were skipped; it must
  never sweep a stale operation through.
- **Delete a proposal** — discards it wholesale.

### Events (SSE)

A single one-way stream at `/events`, driven by `fsnotify` in the Go process.

Event kinds, sent as SSE `event:` names. The client cannot send on this stream.
Proposal events are not emitted yet.

- `model-changed` — a node file or `index.json` changed on disk, whether from a UI write,
  an MCP status write, or the architect's editor. `data` is `{ "node_ids": ["…"] }`.
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
