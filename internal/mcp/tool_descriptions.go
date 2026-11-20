package mcp

import "github.com/KoJaco/ambit/internal/core"

// Authoring tool descriptions include modeling guidance; keep aligned with
// docs/contracts/arch-model-format.md (Modeling guidance) and mcp-tools.md.
var (
	seedModelDescription = `Propose a model from a harness-generated graph (staged, not applied; canonical model unchanged until architect accepts).

Modeling: use parent_id for containment/drill-down; use relationships for runtime flow (calls, delegates, hosts, stages) between nodes at a level — not for every sibling pair. Every node should be reachable from the product root via parent_id. External actors (harness, browser): type external, one directed inbound edge per channel. Thin entry nodes should relate to the module they host (e.g. entry hosts tool surface). See docs/contracts/arch-model-format.md Modeling guidance. ` + core.ShallowModelGuidance

	setRelationshipDescription = `Propose creating or updating a cross-cutting relationship (staged, not applied). Directed edge: from, to, label, kind (sync|async|data). Do not use for parent/child — use parent_id on create_node/update_node. Add edges for control/data flow you need on the canvas; omit when hierarchy alone is enough. Endpoints must name existing node ids. See docs/contracts/arch-model-format.md Modeling guidance. ` + core.ShallowModelGuidance

	createNodeDescription = `Propose one new node (staged, not applied). Set parent_id for hierarchy (or relationship_id for relationship interior). Put runtime wiring in set_relationship, not parent_id. See docs/contracts/arch-model-format.md Modeling guidance. ` + core.ShallowModelGuidance

	updateNodeDescription = "Propose changes to a node (staged, not applied). " + core.ShallowModelGuidance
)
