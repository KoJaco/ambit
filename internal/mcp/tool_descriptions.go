package mcp

import "github.com/KoJaco/ambit/internal/core"

// Authoring tool descriptions include modeling guidance; keep aligned with
// docs/contracts/arch-model-format.md (Modeling guidance) and mcp-tools.md.
var (
	seedModelDescription = `Propose a model from a harness-generated graph (staged, not applied; canonical model unchanged until architect accepts).

Optional node id: a free lowercase slug you choose, so parent_id and relationships can reference nodes created in this same call. Nodes in one call may reference each other in parent_id and depends_on in any order. Relationship kind is one of sync, async or data. Shared code (types, helpers, conventions several nodes use) belongs in one node that owns it, with the other nodes listing it in depends_on, so it is built once and every agent sees it in their brief. protected: true on a node makes its implementation paths off-limits to every other node (check_scope and ambit check flag edits, and briefs list them as off-limits); use it for generated or hand-curated files. Optional risk (low|medium|high), review (none|required) and verify (commands that prove the node works) feed the brief, check_done and the completion policy. Files no node should own (generated output, lockfiles, vendored code) are not modelled: they are excluded from drift with drift_ignore globs in .arch/config.json. Optional updates, delete_relationships and delete_nodes restructure an existing model in one proposal (accept is all-or-nothing).

Modeling: use parent_id for containment/drill-down; use relationships for runtime flow (calls, delegates, hosts, stages) between nodes at a level — not for every sibling pair. Every node should be reachable from the product root via parent_id. External actors (harness, browser): type external, one directed inbound edge per channel. Thin entry nodes should relate to the module they host (e.g. entry hosts tool surface). See docs/contracts/arch-model-format.md Modeling guidance. ` + core.ShallowModelGuidance + " " + core.AuthoringGuidance + " " + contractGuidance

	setRelationshipDescription = `Propose creating or updating a cross-cutting relationship (staged, not applied). Directed edge: from, to, label, kind (sync|async|data). Do not use for parent/child — use parent_id on create_node/update_node. Add edges for control/data flow you need on the canvas; omit when hierarchy alone is enough. Optional contract {format: openapi|jsonschema|text, ref: repo path, text}: what is on the wire, so both ends build to one interface; omitted leaves an existing contract unchanged, {} clears it. Endpoints must name existing node ids. See docs/contracts/arch-model-format.md Modeling guidance. ` + core.ShallowModelGuidance

	deleteRelationshipDescription = "Propose deleting one relationship (edge) by relationship_id (staged, not applied). Deleting an edge also removes any nodes inside it (relationship interior). Nodes at the endpoints are kept."

	listIconsDescription = "List the allowed icon ids with a one-line label each. Use these ids for the icon field of create_node, update_node and seed_model nodes."

	createNodeDescription = `Propose one new node (staged, not applied). Optional id: a lowercase slug you choose (letters, digits, single hyphens) so later calls can reference this node in parent_id or edges; rejected if taken or reserved by another pending proposal. Set parent_id for hierarchy (or relationship_id for relationship interior). Put runtime wiring in set_relationship, not parent_id. See docs/contracts/arch-model-format.md Modeling guidance. ` + core.ShallowModelGuidance + " " + core.AuthoringGuidance + " " + contractGuidance

	updateNodeDescription = "Propose changes to a node (staged, not applied). " + core.ShallowModelGuidance + " " + contractGuidance + " Omitted depends_on/contract are unchanged; depends_on [] and contract {} clear them."

	contractGuidance = "Put interfaces in the structured contract field (payloads, invariants, errors, owns_data), not in the spec prose. Set depends_on only for real build ordering (this node needs those nodes built first), not for flow direction; relationships already express flow. A dependency cycle is rejected. Call get_build_order before building."

	setConstraintsDescription = "Propose replacing the project-wide rules (staged, not applied; replace-all, so send the full list). Each constraint: id (slug), text (1..300 chars), severity must (default) or should. Max 24. Every agent brief shows these rules, so keep them short and checkable."

	getBuildOrderDescription = "Return the advisory build order (read-only, from depends_on): order with wave, depends_on, blocked_by and done per buildable node, plus waves (parallelisable layers), cycles and unordered. Optional scope_node_id limits it to a subtree. Call this before building to pick nodes whose blocked_by is empty."
)
