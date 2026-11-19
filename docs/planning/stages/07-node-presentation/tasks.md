# Stage 07 tasks — Node presentation

Working list. Status is mirrored in [`checklist.md`](../../checklist.md) when sliced.

## 07.1 Icon registry

- [x]

**Touches:** `frontend/`, [`arch-model-format.md`](../../../contracts/arch-model-format.md),
MCP contract additive field.

**Done when:** a fixed map of icon ids to components exists; unknown ids fail load or fall back
to a generic glyph; MCP docs list ids agents may use; optional `icon` on node JSON.

## 07.2 Shape registry

- [x]

**Touches:** `frontend/app/components/NodeCard.tsx`, `arch-model-format.md`, inspector,
[`shape-registry-sketch.md`](shape-registry-sketch.md) finalized into contract tables.

**Done when:** closed shape ids include `standard` (default when unset = current card); sparse
additional shapes render on canvas; unknown ids fall back to `standard`; docs list allowed ids
and non-normative suggested pairings with `type`/icons; optional `shape` on node JSON.

## 07.3 Type-default colours

- [x]

**Touches:** `frontend/`, optional committed defaults in `config.json` or frontend map.

**Done when:** each node renders with a default colour from `type` until overridden; free-form
types still work with a neutral default.

## 07.4 User colour override

- [x]

**Touches:** inspector, `internal/core` if colour is canonical, HTTP PATCH.

**Done when:** architect can set/clear colour on a node; value persists in node JSON; canvas
and NodeCard respect it.

## 07.5 MCP (optional)

- [x]

**Touches:** `internal/mcp`, [`mcp-tools.md`](../../../contracts/mcp-tools.md).

**Done when:** if included, `create_node` / `update_node` accept optional `icon` and `shape` from
registries; colour documented as architect-first unless explicitly added for agents.

## 07.6 Slice and docs

- [ ]

**Done when:** ADR or contract amendment if needed; slice log entry; checklist updated.

## 07.7 Drill affordance (has children)

- [x]

**Touches:** `internal/core` summary for levels, `internal/httpapi`, `frontend/app/components/NodeCard.tsx`.

**Done when:** each child summary on a level includes whether that node has direct children;
NodeCard shows a minimal drill hint (chevron or registry icon); double-click / existing drill
navigation unchanged; relationship `drillable` crossings unchanged.

## 07.8 Superseded by 07.9-07.11

Lift/embed compression is replaced by flat-first groups and group/ungroup proposals
([decision 0010](../../../decisions/0010-flat-first-groups-and-summary-nodes.md), which amends
[0007](../../../decisions/0007-drill-affordance-and-hierarchy-compression.md)).

## 07.9 Core anchor roll-up (0010 slice a)

- [ ]

**Touches:** `internal/core/query.go`, `query_relationship.go`, `internal/httpapi/read.go`,
config parsing, [`local-http-api.md`](../../../contracts/local-http-api.md),
[`arch-model-format.md`](../../../contracts/arch-model-format.md).

**Done when:** `Level(parent, expanded)` resolves anchors and returns aggregated edges, group
membership, `InternalRelCount` and `TypeMix`; crossings are computed against the visible set;
`GET /levels/{id}?expand=a,b` works; `group_max` / `level_budget` parse from `config.json`
with defaults; tests cover roll-up, same-anchor hiding, crossings against V, unknown and
too-deep ids, and empty expand.

## 07.10 Group and summary rendering (0010 slice b)

- [ ]

**Touches:** `frontend/app/layout.ts`, `api.ts` (cache key), `FlowCanvas.tsx`, `NodeCard.tsx`,
`components/flow/group-node.tsx`, new pure `level-view.ts`.

**Done when:** compound ELK layout; expanded groups render as containers with a collapse
toggle and Open link; collapsed nodes render as summary cards; aggregated edges list their
members; view mode persists in the layout cache with the expanded-set hash in the key;
over-budget hint shows and nothing auto-collapses after an architect choice; dragging out of a
group opens a reparent proposal.

## 07.11 Group / ungroup proposals (0010 slice c)

- [ ]

**Touches:** `internal/core/proposal.go`, `mutate.go`, `internal/httpapi/write.go`, frontend
selection UI, MCP tool descriptions (guidance line only).

**Done when:** group selection and ungroup each stage one reviewed proposal; relationships
unaffected; tests cover round trip and protected-node rules; "Prefer shallow models" guidance
is in the MCP descriptions and `get_context`.
