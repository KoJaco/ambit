# Decision Note: Drift in worktrees, ignore list, full briefs, contract tracking

## Date

2026-11-24

## Summary

`check_drift` now sees untracked files, can read another worktree, can be limited to one node, and skips tooling files. `get_context` stops truncating contracts. Nodes remember a contract fingerprint when marked `in_progress` or `done`, so a later contract change is visible.

## Context

A design-first build with about 20 worktree agents showed: drift only counted tracked files, read the main checkout, listed every other node's globs (about 35 lines of noise), and flagged `go.mod` and `package.json`. Sibling and relationship contracts were cut, so agents lacked the protocol they had to implement. Amending a contract left finished nodes silently stale.

## Decision

1.  Drift lists `git ls-files --cached --others --exclude-standard`. Gitignored files stay invisible.
2.  `DriftOptions{Root, FilesDir, NodeID}`. `FilesDir` must share the git common dir with `Root`, so a tool argument cannot read an unrelated tree. The model always comes from `Root`.
3.  Built-in ignore list plus `drift_ignore` in `config.json`. An explicitly owned file is never ignored. Chosen over a `workspace` node type: no extra node in every model.
4.  Briefs include sibling contracts in full and every relationship contract the node touches. The only bound is a 60000 character safety limit. Dependencies list their implementation globs.
5.  `contract_hash` in the node file, set with the status. It lives beside `status` so a worktree agent's status merge carries it. Chosen over a local cache, which would not travel through git. The cost: no text diff of what changed. Use `git log -p .arch/` for that. A stored snapshot is a possible follow-up.
6.  `contract-changed` is advisory in `design-first` and `map-existing`. Nothing reopens a node automatically.

Known limits: a node created directly with status `done` or `in_progress`, or created by an accepted proposal, has no baseline until its status is set again, because edges that give it contracts may arrive later in the same proposal. A `worktree` argument is relative to the model root, resolves to its checkout's top level, and must share the repository's git common dir. Sibling contract text and the Interfaces section each stop at the 60000 character bound with a pointer to `get_graph` (`include_contracts`).

## Impact

Format change is additive (`contract_hash`, `drift_ignore`). Older ambit versions preserve both as unknown keys. `max-depth` default moved to 3 earlier in this series (decision 0013).

## Follow-ups

Completion policy and a `needs_review` status (phase 4). A stored contract diff.
