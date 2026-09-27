# Stage 04 test plan

The release gate is Go. The UI's diff shaping is Vitest. Clicking through the review panel
is manual. Decision: [0004](../../../decisions/0004-testing-strategy.md).

## Release gate — task 04.8

- Accepting `create_node` writes the `.json`, the `.md`, and the `index.json` membership
  entry. A forced failure between those writes (inject a failure after the first file, or
  apply against a read-only `index.json`) leaves neither the node pair nor the membership
  list half-updated.
- Accepting `update_node` replaces both files or neither.
- A stale `base_hash` is not applied. The same call with the confirmation flag does apply,
  and then passes normal validation (a cycle in the proposed `parent_id` stays `pending`
  and names the reason).
- Accept-all on a proposal that mixes fresh and stale operations applies the fresh ones,
  leaves the stale ones `pending`, and the result lists the skips.
- Reject does not delete the proposal while a sibling operation is `pending`.
- When the last operation becomes resolved, the proposal directory is gone.
- A manifest with the wrong `manifest_version` is not applied.
- A proposed node with no `.md` is not applied.
- `delete_node` staged against a node that has children fails at stage time.

## Other Go

- Staging does not create files under `.arch/nodes/`.
- Two proposals touching one node: accepting the first changes the hash the second is
  compared against, and a subsequent read marks the second stale.
- Operation `status` survives a reload of the proposal from disk.

## Vitest — task 04.10

Pure mapping from a diff payload to rows: create, update with a `fields` list, stale
update, `set_relationship`. The test feeds the mapping a fixture. It does not reimplement
hash comparison.

## Manual

Once, on a scratch model with `ambit start` and the dev server:

- Stage a proposal from a test helper or a tiny dev-only call used by the test, not from a
  new public CLI.
- Confirm the review panel appears through SSE without a refresh.
- Edit the same node in the inspector, confirm the operation flips to stale, confirm
  accept-all skips it, confirm an explicit confirm applies it.
- Accept one operation and reject another, reload, confirm the manifest remembered both.

## Not in this stage

- MCP tool responses.
- Component tests of the review panel.
