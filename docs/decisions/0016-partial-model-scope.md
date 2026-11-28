# Decision Note: Partial-model scope and parts

## Date

2026-11-28

## Summary

A project can declare that its model covers only some parts of the repository. `config.json` gains `scope` (`whole` or `partial`) and `parts`. In partial scope drift reports an unowned file only when it matches some part's `include` globs, so an existing monorepo can be modeled one slice at a time without every other file showing up as noise.

## Decision

1. `config.json` `scope`: `"whole"` (default, key absent) or `"partial"`. An invalid value warns and means `whole`.
2. `config.json` `parts`: `[{id, name, include: [glob], root_node_ids: [node id]}]`. At most 24 parts, 32 globs of at most 300 characters each (validated with doublestar), 64 roots; ids are unique lowercase slugs. Invalid or duplicate entries are skipped with a warning; a root id naming no node is a warning (so a part can be staged in the same proposal that creates its roots).
3. Drift (`driftLocked`): in partial scope an unowned file is reported only when it matches an `include` glob of some part; the rest are counted in a new `outside_parts` field. `ignored_files` keeps meaning "matched drift_ignore". The owned-file, overlap and unmatched-glob checks are unchanged. `DriftOptions.Part` restricts the unowned list to one part; `check_model`, `check_drift` and `check_done` use the same drift.
4. Writers: `ambit init --scope whole|partial` and `--part name=glob,glob` (repeatable; implies partial), a TTY prompt on a fresh init only, `PUT /parts` and `DELETE /parts/{id}` for the human UI, and the MCP tool `set_part`, which stages `set_part`/`delete_part` proposal operations so a human accepts them. The scope itself is never written by an agent tool.
5. `get_graph` takes an optional `part` and returns that part's `root_node_ids`, their descendants and the edges between them.
6. Briefs of a partial-scope project, and the authoring tool descriptions, add one sentence: model only the active part and put its files in `include`. Whole-scope briefs are byte-identical to before.

## Rationale

- Mapping an existing repository is the common start. Whole-repo drift buries the signal under files the team has not modeled yet.
- Globs on the part, not on a node flag, keep one source of truth for "what the model covers" and reuse the drift code path.
- A staged part operation reuses proposal mechanics (diff, staleness, snapshot, rollback), so agents can suggest a part and a human stays in control.

## Impact

Additive to `config.json`; older versions keep unknown keys. `GET /parts`, `PUT /parts`, `DELETE /parts/{id}`, `outside_parts` in drift output, a `part` argument on three MCP tools, `set_part`.

A part operation's base hash covers only the `scope` and `parts` keys, and one part operation is allowed per proposal.

## Follow-ups

- [ ] UI: a Nodes | Parts toggle in the sidebar and part-scoped canvas roots.
- [ ] Optional `ambit drift --part` flag on the CLI.

## References

- Related: `docs/contracts/arch-model-format.md`, `docs/contracts/mcp-tools.md`, `docs/contracts/local-http-api.md`, `docs/contracts/proposals.md`, decision 0014 (drift).
