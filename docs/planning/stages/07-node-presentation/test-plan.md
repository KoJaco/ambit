# Stage 07 test plan

Logic is Go and pure TypeScript. No DOM or Playwright tests in this stage. Decision:
[0004](../../../decisions/0004-testing-strategy.md).

## Release gate

- `go test ./... -race` passes: registry round-trip, `Level(parent, expanded)` roll-up,
  group/ungroup/reparent proposals.
- `pnpm exec vitest run` passes: node presentation mapping, level-view defaults, layout,
  layout cache keys and signatures, canvas structure helpers.
- `pnpm exec tsc --noEmit` is clean.

## Go

- Roll-up: grandchild relationships appear on the parent level as aggregated edges with
  members, kinds and label; same-anchor relationships are hidden and counted; crossings are
  computed against the visible set; unknown, non-visible and too-deep expand ids are
  ignored; output is deterministic.
- Config: `group_max` and `level_budget` defaults, override, round trip, and fallback with a
  warning on invalid values.
- Group, ungroup, reparent: proposal contents, accept-all, relationships unchanged,
  validation errors, duplicate container ids across pending proposals, dependent updates
  skipped when the create is not accepted, dangling parent does not panic.
- Layout cache: invalidation removes `<key>~*` variants and ancestor variants on reparent.
- The shallow-model guidance line is in the brief and the MCP authoring descriptions.

## Vitest

- `level-view`: default expansion, architect modes win, budget and depth limits, hint,
  expanded-set hash, `canExpandChild`, layout signature.
- `layout`: compound layout matches the flat layout for flat input.
- `canvas-structure`: group name defaults, container hit test, outside-drop target on root
  and nested levels, error messages.

## Manual (needs a browser; not automated)

Run on the ambit self-model, light and dark, three widths, with `ui-review`:

- Paper theme: contrast of muted text and glyph tones, focus rings, selection ring.
- Expanded group, collapsed summary card, aggregated edge list, over-budget hint.
- Reload restores view mode and positions.
- Select two siblings, group them, review and accept the proposal; ungroup it.
- Drag a member out and a node in; each stages a proposal and snaps back.
