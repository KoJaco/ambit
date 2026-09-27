# Stage 03 test plan

Go tests for the query and the cache. Vitest for two pure functions. The canvas itself is
checked by hand. Decision: [0004](../../../decisions/0004-testing-strategy.md).

## Go

- Drill-down for root and for a nested node returns only that level's children and the
  relationships among them. A relationship to a node outside the level is absent from the
  edge list.
- The handler or query used by the level endpoint has a test that fails if a fixture with
  nodes outside the level is fully serialised into the response.
- Create and update go through core validation: a cycle is 409, and no second validator
  exists in `internal/httpapi`.
- Layout put writes under `.arch/.cache/layout/` and leaves `nodes/` and `index.json`
  unmodified.
- Layout get on a missing key returns empty.
- A create, delete, or reparent invalidates that level's cache entry. A name change does
  not.
- Assignment put writes `local.json` and does not modify the node's JSON.
- Server bind: a test or a constructor rejects a non-loopback address.

## Vitest — task 03.17

- Layout cache key for `/` and for `/node/:nodeId` matches the key the API uses.
- Scope preview: empty scope previews as the implementation globs; an explicit scope
  previews as itself; a protected flag is reported separately from scope. These cases
  mirror the Go rule names in spirit. They are allowed to drift until the open question
  says otherwise, and a comment in the test file should point at that question.

## Manual

Do this once before checking the stage done. No Playwright.

- `ambit init` a scratch repo, `ambit start`, Vite dev server pointed at it.
- Create two nodes, parent one under the other, follow the URL to the child level, use the
  browser back button, reload `/node/<id>` (against Vite, not the embedded binary).
- Drag a node, reload, confirm the position held.
- Add a child and confirm that level lays out again.
- Toggle `protected` and leave `scope` empty; confirm the inspector copy matches the task.
- Set an assignment and confirm `local.json` changed and no node file did.
- Edit a node JSON in an editor and confirm the canvas updates through SSE without a
  refresh.
- Confirm the listen address is loopback.

## Not in this stage

- Proposal diffing tests. Stage 04.
- Component or end-to-end tests.
- A shared Go/TypeScript glob fixture.
