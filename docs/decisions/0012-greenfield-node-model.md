# Decision Note: Richer node model for greenfield builds

## Date

2026-11-23

## Summary

External testing showed ambit models structure well but cannot yet drive a greenfield build: two agents building neighbouring nodes share no interface contract, project-wide rules never reach the agent, and there is no build order. We add four additive, optional things to the v1 format: a structured per-node `contract`, a per-edge `contract`, project `constraints` in `config.json`, and node `depends_on` with a derived build order. `get_context` briefs consume them, and one new read tool (`get_build_order`) plus one staged tool (`set_constraints`) are added. `schema_version` stays 1.

## Context

-   The node spec is free markdown. Payload shapes, invariants, error cases and data ownership are mixed into prose, so agents cannot find, diff or check them.
-   An edge (`label`, `kind`) says that A calls B, not what is on the wire. Agents on A and B invent incompatible interfaces.
-   Rules like "every query scopes by user_id" or "no secrets in logs" are per project. Today they must be pasted into every spec or they are lost.
-   Relationship edges are runtime flow and may be cyclic, so they cannot express "build X before Y".
-   Goal: design -> per-node brief -> `check_scope` -> `check_drift`, in dependency order.

## Decision

1.  **Node `contract`** (optional, fixed object): `{ payloads: string, invariants: string[], errors: string[], owns_data: string[] }`. Large machine-readable specs (OpenAPI, JSON Schema) are referenced by repo path in text; they are not stored in `.arch`.
2.  **Edge `contract`** (optional): `{ format: "openapi" | "jsonschema" | "text", ref: string, text: string }`. Stored as opaque text. ambit parses and validates no payload schemas.
3.  **Project `constraints`** in `config.json`: ordered list of `{ id, text, severity }` where `severity` is `must` (default) or `should`. Global only in this phase.
4.  **Node `depends_on`**: explicit list of node ids. Never inferred from flow edges. A cycle is rejected on mutation.
5.  **Build order** is derived: `get_build_order` returns buildable (leaf, or container with `implementation` globs) nodes in topological order with parallelisable `wave`s and `blocked_by`. A container counts as done when its children are. It is **advisory**: unmet dependencies produce a warning in the brief and the tool, never a block on `update_node_status`.
6.  **Authoring**: `create_node`, `update_node`, `seed_model` and `set_relationship` accept the new fields and stage them like any other change. `set_constraints` is a staged tool (replace-all). The UI edits constraints directly, as the architect's own surface. Agents never write constraints directly.
7.  **Edge staleness**: a relationship op's `base_hash` also covers the relationship's own JSON, so two agents editing one edge's contract cannot silently overwrite each other.
8.  `schema_version` stays 1 (additive). Older ambit keeps the new keys in `Unknown` and round-trips them.

## Format details

-   Node file key order: `... protected, icon, color, renderer, schema, depends_on, contract, status`. Omit empty. `depends_on` is written sorted for stable diffs. Relationship order: `id, from, to, label, kind, contract`.
-   Limits (new values over them are rejected, load tolerates): `payloads` <= 8000 chars; each list <= 32 items of <= 300 chars; edge `text` <= 4000; `ref` <= 300 and must not escape the repo (`..`); `format` is a closed set (unknown values on disk load as-is). Constraints: <= 24 items, `id` unique slug, `text` 1..300 chars.
-   `depends_on` validation: ids exist, no self-reference, no duplicates. Dangling on load is a warning diagnostic; a cycle on mutation returns `ErrDependencyCycle`; a cycle found on load is an error diagnostic. Deleting a node that others depend on is rejected with their names (like `ErrHasChildren`).
-   Invalid constraint entries on load are skipped with a warning, not a load error.
-   Brief sections, in order: scope/protected/siblings (siblings show contract payloads and errors, capped ~800 chars each); **Project rules** (always present when any exist, `must` first); **Interfaces you must honour** (per touching relationship, marked provider/consumer); **Your node's contract** (never truncated); **Build order** (depends on X (status); stop and report if unmet; dependents relying on you); the `check_scope` instruction plus a closing line to run `check_drift` and `update_node_status` when done.

## Refinements (review follow-up)

-   **Interfaces section lists only relationships that carry a contract.** An edge with no contract says nothing the agent can honour; omitting it keeps the brief short. The section is capped at about 4000 characters with a pointer to `get_graph` / the HTTP API for the rest.
-   **A buildable container counts as done from its children alone** (confirmed intended): its own `status` is ignored. The same rule and cascade drive the brief's build-order fallback and "dependents relying on you".
-   **Cycle checks use the expanded graph.** A dependency on a pure container means its buildable descendants, and a container's `depends_on` applies to each descendant, so mutations are validated on the same graph `get_build_order` uses.
-   **`SetConstraints` rewrites `config.json`** with keys sorted alphabetically and the constraints array on one compact line. Unknown keys inside contracts and constraints are preserved. Saved JSON is not HTML-escaped.
-   A running `ambit mcp` reloads on `config.json` changes so constraints edited via `ambit start` reach its briefs.

## Rationale

-   Additive, so no migration and no version bump.
-   Fits staging, hashing, review and diff with no new file kind or write path.
-   Structured fields let the brief hand each agent exactly the contracts for its node and edges.
-   Constraints at project level remove copy-paste and cannot be forgotten per node.
-   Explicit `depends_on` is correct where inferring from flow edges is wrong (call direction is not build direction).
-   Opaque contract text keeps ambit free of parser and provider dependencies (portability rule).

## Impact

-   `brief.go` gets longer; the 0008 "no graph doctrine in the brief" rule is narrowly amended: constraints and contracts are project facts, not modelling doctrine.
-   MCP tool count 13 -> 15 (`get_build_order`, `set_constraints`); `get_graph` gains `depends_on` and `has_contract`.
-   New HTTP: `GET/PUT /constraints`, `GET /build-order`; node and relationship bodies gain `contract` and `depends_on`.
-   Frontend: inspector contract and depends-on fields, edge contract section, constraints editor, build-order badge, review diff rendering of the new fields.
-   Proposals and reviews get bigger. Contracts are advisory text; contract-vs-code drift is out of scope.

## Follow-ups

-   [ ] Contract-vs-code checking (needs the reserved `observed_implementation`).
-   [ ] Optional per-subtree constraints (`applies_to`).
-   [ ] Optional `.arch/contracts/` files if inline contracts prove too small.
-   [ ] "Suggest depends_on from edges" UI action.
-   [ ] Record brief wording observations from real greenfield runs here.

## References

-   docs/contracts/arch-model-format.md, mcp-tools.md, proposals.md, local-http-api.md
-   Related: decisions 0005 (get_context brief), 0008 (modelling guidance), 0010 (groups)
