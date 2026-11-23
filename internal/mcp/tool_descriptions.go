package mcp

import "github.com/KoJaco/ambit/internal/core"

// Authoring tool descriptions include modeling guidance; keep aligned with
// docs/contracts/arch-model-format.md (Modeling guidance) and mcp-tools.md.
var (
	seedModelDescription = `Propose a model from a harness-generated graph (staged, not applied; canonical model unchanged until architect accepts).

Optional node id: a free lowercase slug you choose, so parent_id and relationships can reference nodes created in this same call. Optional updates, delete_relationships and delete_nodes restructure an existing model in one proposal (accept is all-or-nothing).

Modeling: use parent_id for containment/drill-down; use relationships for runtime flow (calls, delegates, hosts, stages) between nodes at a level — not for every sibling pair. Every node should be reachable from the product root via parent_id. External actors (harness, browser): type external, one directed inbound edge per channel. Thin entry nodes should relate to the module they host (e.g. entry hosts tool surface). See docs/contracts/arch-model-format.md Modeling guidance. ` + core.ShallowModelGuidance + " " + core.AuthoringGuidance

	setRelationshipDescription = `Propose creating or updating a cross-cutting relationship (staged, not applied). Directed edge: from, to, label, kind (sync|async|data). Do not use for parent/child — use parent_id on create_node/update_node. Add edges for control/data flow you need on the canvas; omit when hierarchy alone is enough. Endpoints must name existing node ids. See docs/contracts/arch-model-format.md Modeling guidance. ` + core.ShallowModelGuidance

	deleteRelationshipDescription = "Propose deleting one relationship (edge) by relationship_id (staged, not applied). Deleting an edge also removes any nodes inside it (relationship interior). Nodes at the endpoints are kept."

	listIconsDescription = "List the allowed icon ids with a one-line label each. Use these ids for the icon field of create_node, update_node and seed_model nodes."

	createNodeDescription = `Propose one new node (staged, not applied). Optional id: a lowercase slug you choose (letters, digits, single hyphens) so later calls can reference this node in parent_id or edges; rejected if taken or reserved by another pending proposal. Set parent_id for hierarchy (or relationship_id for relationship interior). Put runtime wiring in set_relationship, not parent_id. See docs/contracts/arch-model-format.md Modeling guidance. ` + core.ShallowModelGuidance + " " + core.AuthoringGuidance

	updateNodeDescription = "Propose changes to a node (staged, not applied). " + core.ShallowModelGuidance
)
