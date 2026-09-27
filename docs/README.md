# Documentation Guide

This directory is the durable record for ambit's decisions, contracts, and build intent.

## What belongs here

- architecture decisions
- stable contracts
- model format and enforcement invariants
- migration plans and slice logs
- durable tradeoff records

## What does not belong here

- obvious implementation detail
- temporary brainstorming that has no execution value
- duplicated code comments or schema dumps

## Primary documentation areas

- `docs/adr/` for durable architectural decisions (historical ADRs may retain legacy names)
- `docs/architecture/` for system shape and responsibility boundaries
- `docs/contracts/` for stable public or cross-layer contracts
- `docs/decisions/` for tactical implementation choices
- `docs/planning/` for active planning and build history
- `docs/archive/` for superseded or intentionally retained historical material

## Start here

ambit is a local-first, Git-backed architecture model that an architect builds from a client
transcript and hands to coding agents as a source of truth, with a local check that flags
work falling outside the boundaries the architect drew.

New to the project, read in this order:

1. [`planning/spec-v1.md`](planning/spec-v1.md) — what the product is.
2. [`architecture/system-overview.md`](architecture/system-overview.md) — how it is shaped.
3. [`architecture/enforcement-model.md`](architecture/enforcement-model.md) — the part that
   makes the model consequential.

## Current canonical plan

- [`planning/spec-v1.md`](planning/spec-v1.md) — the canonical v1 spec. Supersedes the
  original spec in full; its deltas are enumerated at the top.
- [`planning/v1-build-plan.md`](planning/v1-build-plan.md) — build order and release gates.
- [`planning/frontend-refactor.md`](planning/frontend-refactor.md) — the file-by-file
  frontend reset inventory.
- [`planning/open-questions.md`](planning/open-questions.md) — deliberately unresolved
  questions and known thin spots.

## Active contracts (canonical)

- [`contracts/arch-model-format.md`](contracts/arch-model-format.md) — the `.arch` on-disk
  format. The most durable contract in the product; the model outlives any version of ambit.
- [`contracts/mcp-tools.md`](contracts/mcp-tools.md) — the eight MCP tools, and the framing
  requirements for `get_context`.
- [`contracts/proposals.md`](contracts/proposals.md) — the staging format that holds
  agent-authored mutations for review.
- [`contracts/local-http-api.md`](contracts/local-http-api.md) — the localhost HTTP surface
  and SSE stream serving the SPA.

## Active architecture notes

- [`architecture/system-overview.md`](architecture/system-overview.md) — components, the
  two front doors, and the system-wide invariants.
- [`architecture/enforcement-model.md`](architecture/enforcement-model.md) — `ambit check`,
  `scope` and `protected` semantics, and the layered enforcement argument.
- [`architecture/frontend.md`](architecture/frontend.md) — drill-down routing, scoped
  rendering, layout caching, and the review surface.

## Decisions

Durable, in `docs/adr/`:

- [ADR-0001](adr/0001-canonical-model-storage.md) — canonical model storage and format.
- [ADR-0002](adr/0002-agent-interface-and-review-gate.md) — harness-delegated generation,
  the MCP authoring surface, and the staging gate.
- [ADR-0003](adr/0003-runtime-and-distribution.md) — runtime, process model, distribution.
- [ADR-0004](adr/0004-frontend-platform.md) — frontend platform and the domain reset.

Tactical, in `docs/decisions/`:

- [0001](decisions/0001-repo-topology.md) — repository topology.
- [0002](decisions/0002-node-type-taxonomy.md) — free-form node types.
- [0003](decisions/0003-positioning.md) — dropping the vertical wedge.
- [0004](decisions/0004-testing-strategy.md) — testing strategy.

## Superseded material

- [`archive/spec-v0-original.md`](archive/spec-v0-original.md) — the original build spec,
  verbatim. Retained because the canonical spec departs from it materially: the product
  name, the frontend framework, the node type taxonomy, the presence of an LLM client, the
  size of the MCP surface, and the introduction of a staging layer. Keeping it intact means
  every delta can be traced to what it replaced.
