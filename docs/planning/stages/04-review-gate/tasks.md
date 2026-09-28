# Stage 04 tasks

Working list. Status is mirrored in [`checklist.md`](../../checklist.md).

Core and the gate tests can land before the UI. The UI consumes the HTTP endpoints, which
consume core.

## 04.1 Proposal files

- [x]

**Touches:** `internal/core`.

**Done when:** a proposal is a directory `.arch/.proposals/<id>/` containing `manifest.json`
and materialised `nodes/<id>.json` and `.md` files. The id is time-ordered
`p-<YYYYMMDD>-<HHMM>-<short-random>`. The manifest carries `manifest_version` 1,
`proposal_id`, `created_at`, `source`, and `operations`. A version mismatch is reported as
unreadable and is not applied. A proposed node missing its `.md` is an integrity error and
is not applied as empty prose.

## 04.2 Stage function

- [x]

**Touches:** `internal/core`.

**Done when:** one function writes a proposal for a set of operations and returns the id.
It validates what can be known before apply: `delete_node` of a node that has children is
rejected here, with the child count and names; a `parent_id` that would cycle is rejected
here; a relationship endpoint that does not exist is rejected here. It does not modify
`.arch/nodes/`. Tests call this function directly. `create_node` ids are derived with the
stage 01 slug rules, and `base_hash` is null. Other operations record the current content
hash of the node they name.

## 04.3 Staleness

- [x]

**Touches:** `internal/core`.

**Done when:** staleness is computed when a proposal is read, by comparing `base_hash` to
the node on disk now. It is not stored in the manifest. A missing node targeted by
`update_node` or `delete_node` is stale and cannot be applied. Two pending proposals that
touch the same node are both listed; accepting one makes the other stale with no special
case.

## 04.4 Accept and reject

- [x]

**Touches:** `internal/core`.

**Done when:** accept moves that operation's `.json` and `.md` into `.arch/nodes/` and
updates `index.json` as one unit, then runs the same validation as a direct mutation. A
failure leaves the operation `pending` and leaves the rest of the proposal alone. A stale
accept without confirmation does not write. Reject sets the operation `rejected` and leaves
the proposal directory in place. Status is persisted in the manifest so a partial review
survives a restart. Accept-all skips stale operations and reports which ones it skipped.
Reject-all marks the rest rejected.

## 04.5 Resolved proposals

- [x]

**Touches:** `internal/core`.

**Done when:** a proposal whose every operation is `accepted` or `rejected` is removed.
A proposal with any `pending` operation stays, including ones that are only stale.

## 04.6 Proposal HTTP

- [x]

**Touches:** `internal/httpapi`.

**Done when:** the endpoints in the HTTP contract exist: list (with staleness), diff,
accept, reject, accept-all, reject-all, delete. Accept of a stale operation without the
confirmation flag is 409, names the node, and writes nothing. Error bodies name the node,
operation, or rule. Unreadable proposals are listed as such and can be deleted.

## 04.7 Proposals SSE

- [x]

**Touches:** `internal/httpapi`.

**Done when:** staging, resolving, or deleting a proposal emits `proposals-changed` on the
existing `/events` stream. The stage 03 client, which ignores unknown kinds, is extended to
refresh the review surface on this kind.

## 04.8 Release gate

- [x]

**Touches:** `internal/core` tests.

**Done when:** the atomicity and staleness cases in [`test-plan.md`](test-plan.md) pass.
The stage is not done without them.

## 04.9 Review UI

- [ ]

**Touches:** `frontend/`, new review surface. The inventory names it in
[`frontend-refactor.md`](../03-architect-canvas/frontend-refactor.md).

**Done when:** pending proposals list with their summary and one row per operation, rendered
from the server diff. Accept and reject call the endpoints. Stale operations are visually
distinct and the accept control requires the confirmation the API demands. Accept-all uses
the endpoint that skips stale operations, and the UI shows which were skipped. A
`seed_model`-sized fixture of about thirty operations is readable as a list with a summary,
not one undifferentiated wall. The client does not compute staleness.

## 04.10 Proposal diffing tests

- [ ]

**Touches:** `frontend/` Vitest.

**Done when:** the pure function that turns a diff payload into the rows the UI renders is
covered for a create, an update, a stale update, and a relationship operation. No component
test.
