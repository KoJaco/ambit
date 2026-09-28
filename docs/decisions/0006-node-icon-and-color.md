# Decision Note: Node icon, shape, and colour

## Date

2026-09-28 (shapes added same day)

## Summary

Add optional **icon** and **shape** (from closed registries), plus **colour** (user override
with type defaults) so the canvas and MCP-authored models stay visually consistent. Icons and
shapes are agent-friendly; colour is architect-first unless we later extend MCP.

## Decision

- **Icon:** closed registry of string ids → known SVG/components. Agents and UI pick ids only.
  Independent of `type` and `shape`.
- **Shape:** closed registry of string ids → NodeCard (or React Flow node) geometry. Sparse
  v1 set; **`standard`** (or `normal`) is the current rounded card and is the default when
  unset. Agents and UI pick ids only — no free-form SVG paths.
  - The registry may document **suggested** pairings (e.g. shape `cylinder` with common
    `type` strings like `database`, or with icon ids). Suggestions are hints for humans and
    MCP prompts, **not** validation — free-form `type` stays.
  - Shapes do not change enforcement or scope; presentation only.
- **Colour:** optional field on the node; inspector colour picker for the architect; default
  palette keyed off `type` when unset. Agents do not set colour in the first slice unless
  product iteration explicitly opens it.
- **Ordering:** ship as [stage 07](../planning/stages/07-node-presentation/README.md) before
  [distribution](../planning/stages/08-distribution/README.md) (embed + npx shim).

## Rationale

- Product exploration on the meta-model showed nodes need identity beyond free-form type strings.
- Closed registries avoid path injection and broken asset references from MCP.
- Shapes give a second visual channel (package vs service vs datastore) without a type enum.
- Colour tied to type aids scanning; override respects architect judgment without a type enum.

## Impact

- `arch-model-format.md` gains optional `icon`, `shape`, and `color` (or similar) fields —
  requires contract version note, not breaking existing nodes.
- `NodeCard` and MCP tool schemas document all registries (allowed ids + suggested pairings
  as non-normative tables in docs).
- Not the same as spec Section 12 “node type registry” (still out).

## Follow-ups

- [ ] Choose hex vs token names for colour storage.
- [ ] Whether MCP `update_node` may set colour after dogfooding.
- [ ] Initial shape set (e.g. `standard`, `hex`, `cylinder`, `cloud`, `document`) — slice at
  implementation time; keep sparse.
- [ ] Whether “suggested relations” includes suggested **relationship** `kind`/labels when an
  edge connects two shaped nodes, or only shape↔type/icon hints (start with shape↔type/icon only).

## References

- Stage: [`07-node-presentation`](../planning/stages/07-node-presentation/README.md)
- Iteration: [`product-iteration-notes.md`](../planning/product-iteration-notes.md)
