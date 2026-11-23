# Decision note 0008 — Architecture graph modeling guidance

## Date

2026-09-29

## Summary

Documented normative **modeling guidance** for `.arch` in
[`arch-model-format.md`](../contracts/arch-model-format.md) (section “Modeling guidance”).
Surfaced the same rules in MCP authoring tool descriptions, [`mcp-tools.md`](../contracts/mcp-tools.md),
and a Cursor rule for `.arch` edits. Did **not** add the full graph checklist to default
`get_context` output or new `ambit check` connectivity heuristics.

## Decision

- **Authoring** (transcript seed, `set_relationship`, hand-editing `.arch`) should follow the
  two-mechanism model: hierarchy vs cross-cutting relationships, reachability from platform,
  external actors, entry→module and delegate patterns, optional `*-data-flow` notes.
- **Implementation** agents continue to receive scope, protected paths, siblings, and
  `check_scope` via `get_context` only (`internal/core/brief.go`).

- **Amendment (decision 0010):** authoring tool descriptions and the `get_context` brief
  each carry one shared line: "Prefer shallow models. Use a container node to group related
  nodes. Nest more than two levels only for genuinely separate subsystems." It is a single
  sentence, not the graph checklist, so the brief stays focused on file boundaries.

- **Amendment (decision 0012):** the brief also carries project facts (project rules,
  contracts, build order). These are not modelling doctrine, so the rule above is unchanged.

## Rationale

- Harnesses read MCP tool text more often than long contract docs; duplicating a short
  checklist there reduces disconnected graphs without new validation logic.
- Coding briefs stay focused on file boundaries; graph doctrine would add noise and encourage
  agents to edit `.arch` when assigned to application nodes.

## Impact

- Changes to modeling rules: update `arch-model-format.md` first, then MCP descriptions in
  `internal/mcp/tool_descriptions.go` (keep in sync with `mcp-tools.md` authoring section).
- Example graphs: `.arch` note nodes under `platform` / `agent-interface`.

## References

- [arch-model-format.md](../contracts/arch-model-format.md) — Modeling guidance
- [mcp-tools.md](../contracts/mcp-tools.md) — Authoring tools
- [ADR-0005](../adr/0005-relationship-drill.md) — Relationship drill and stable ids
