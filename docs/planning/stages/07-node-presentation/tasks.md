# Stage 07 tasks — Node presentation

Working list. Status is mirrored in [`checklist.md`](../../checklist.md) when sliced.

## 07.1 Icon registry

- [ ]

**Touches:** `frontend/`, [`arch-model-format.md`](../../../contracts/arch-model-format.md),
MCP contract additive field.

**Done when:** a fixed map of icon ids to components exists; unknown ids fail load or fall back
to a generic glyph; MCP docs list ids agents may use; optional `icon` on node JSON.

## 07.2 Shape registry

- [ ]

**Touches:** `frontend/app/components/NodeCard.tsx`, `arch-model-format.md`, inspector,
[`shape-registry-sketch.md`](shape-registry-sketch.md) finalized into contract tables.

**Done when:** closed shape ids include `standard` (default when unset = current card); sparse
additional shapes render on canvas; unknown ids fall back to `standard`; docs list allowed ids
and non-normative suggested pairings with `type`/icons; optional `shape` on node JSON.

## 07.3 Type-default colours

- [ ]

**Touches:** `frontend/`, optional committed defaults in `config.json` or frontend map.

**Done when:** each node renders with a default colour from `type` until overridden; free-form
types still work with a neutral default.

## 07.4 User colour override

- [ ]

**Touches:** inspector, `internal/core` if colour is canonical, HTTP PATCH.

**Done when:** architect can set/clear colour on a node; value persists in node JSON; canvas
and NodeCard respect it.

## 07.5 MCP (optional)

- [ ]

**Touches:** `internal/mcp`, [`mcp-tools.md`](../../../contracts/mcp-tools.md).

**Done when:** if included, `create_node` / `update_node` accept optional `icon` and `shape` from
registries; colour documented as architect-first unless explicitly added for agents.

## 07.6 Slice and docs

- [ ]

**Done when:** ADR or contract amendment if needed; slice log entry; checklist updated.

## 07.7 Drill affordance (has children)

- [ ]

**Touches:** `internal/core` summary for levels, `internal/httpapi`, `frontend/app/components/NodeCard.tsx`.

**Done when:** each child summary on a level includes whether that node has direct children;
NodeCard shows a minimal drill hint (chevron or registry icon); double-click / existing drill
navigation unchanged; relationship `drillable` crossings unchanged.

## 07.8 Hierarchy compression (lift / embed) — optional follow-on

- [ ]

**Touches:** canvas selection UX, proposal batching, docs only until sliced.

**Done when:** not part of stage 07 gate unless explicitly pulled in; track in
[decision 0007](../../../decisions/0007-drill-affordance-and-hierarchy-compression.md) until
scheduled.
