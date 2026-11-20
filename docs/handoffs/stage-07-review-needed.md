# Stage 07 review queue (Paper theme + flat-first, decision 0010)

Running log of things that need a human look. Plan: `~/.claude/plans/we-re-trying-to-finish-cozy-bumblebee.md`.
Branch: `stage/07-node-presentation`. Nothing is pushed; no PR is open.

## Status

All of 07.1-07.11 are built and committed. Last run: `go test ./... -race -count=1` pass,
`tsc --noEmit` clean, Vitest 74/74. An independent reviewer pass found 4 real bugs; all four
plus three nits are fixed with regression tests (`0d41bae`). **No UI has ever been run in a
browser** (the sandbox blocked starting the server), so start with "Do this first" below.

## Do this first (only you can)

1. Run `go run ./cmd/ambit` + `pnpm dev` on the ambit self-model and look at it in light and dark.
   Walk through the manual list in `docs/planning/stages/07-node-presentation/test-plan.md`.
2. Run the `ui-review` skill (contrast, focus, 3 breakpoints). Suspects: invented slate/green
   tones, small `text-muted-foreground`, clamped member drag, edges attached to container handles.
3. Answer the three questions under "Needs your decision".

## Needs your decision

- **`.gitignore`**: your uncommitted change adds `.arch/` and `.cursor/`. `.arch/` has no tracked
  files, so the ambit self-model is not versioned. Left out of every commit. Intended?
- **`frontend/pnpm-lock.yaml`**: created by a pnpm auto-install, left untracked. Confirm pnpm is
  the package manager and whether to commit the lockfile.
- **Endpoint name**: the handoff says `GET /api/level/:id?expand=`; the real route is
  `/levels/{nodeId}?expand=`. Implemented on the real route.

## Judgment calls made while you were away

### Phase 1 (Paper theme)
- Tone system: `--node-X` is now the strong colour and `--node-X-bg` the tint (no `--tone-*`).
- **Slate and green tones were invented** (the patch had no pairs): slate `#e8ebf0`/`#475166`,
  green `#e4f1e0`/`#3a6b2c`, plus dark equivalents. Check by eye.
- `.node-glyph` grew from 20px to 28px; new `.node-glyph-lg` (40px) for the inspector.
- `--chart-*` now use the tone colours.
- Data-edge label is a two-line pill (label, then `from → to`).
- Protected helper text I wrote: "Agents cannot edit this node's files, regardless of assignment."
- Arrowhead CSS uses `!important` because React Flow sets inline style on the marker.

### Phase 2 (core roll-up) - behavior change, mention in release notes
- Two existing tests were changed on purpose: the root level now shows two aggregated edges
  (`orders<->billing` roll up onto `platform<->billing`), and `payments->capture` on the platform
  level is no longer a crossing; it is hidden and counted in `payments.InternalRelCount`.
- `RelationshipLevel` was left unchanged (rolling up there would change its behavior); `anchorOf`
  takes a visible-set map so it can be reused later.
- Aggregated edge ids are `agg:<from>><to>` even with one member; `kind` is set only when all
  members share it.

### Phase 4 backend (group / ungroup / reparent)
- `POST /proposals/group|ungroup|reparent` reuse the existing multi-op proposal machinery.
- **Ungroup default keeps the container** (conservative). `delete_container:true` is refused with
  400 when the container has relationships, so edges are never silently dropped. The existing
  delete-node proposal DOES silently drop attached relationships; I did not change that.
- Protected nodes can be grouped and moved: `protected` only guards implementation paths and has
  no rule on `parent_id`. Add a rule if you want one.
- The `get_context` brief now ends with the shallow-model line, which contradicts the old 0008
  text ("brief carries no modeling doctrine"); 0008 was amended to say so. Confirm you want it.

## Not verified (needs eyes or a browser)

- Phase 1 was **not** checked in a browser and lint was not run. Contrast of `.node-glyph-*` and
  small `text-muted-foreground` needs a `ui-review` pass (WCAG 2.2 AA).

## Known leftovers (not fixed, out of scope)

- 24 pre-existing `golangci-lint` findings (errcheck 21, staticcheck 1, unused 2) and many files
  that are not gofmt-clean on main.
- `docs/contracts/proposals.md` does not list the new sources `group_nodes`, `ungroup_node`,
  `reparent_node`.
- Nothing is wired to emit `proposals-changed` for the new endpoints yet.
- `ControlBar`, `proposal-review` and others still use `foreground/xx` borders and `red-600`.
- `node-inspector-presentation.tsx` still has `text-foreground/60` uppercase labels.
- "Open ›" footer on stacked cards is plain text until the Phase 3 drill interaction lands.

### Phase 3 (rendering)
- **View modes are in `localStorage`** (`ambit:view-modes:<baseKey>`), not `.arch/.cache`: the
  server's typed `core.Layout` would drop an extra field and I did not change it. Per-browser,
  not shared. `frontend.md` was amended to allow this. Moving them server-side is a small follow-up.
- Layout validity uses a client-side signature in localStorage (`ambit:layout-sig:<key>`);
  layouts cached before this change are recomputed once.
- The aggregated-edge member list shows ids, not names.
- An expanded node with a `schema` or `placeholder` renderer becomes a container; a collapsed one
  keeps its renderer with no expand toggle.

### Review (reviewer pass, 2c0afac..e1e6da0)
Fixed in `0d41bae`: drag-out onto empty canvas reparented to the model root on nested levels;
two same-name group proposals shared a container id; cached layouts reused after moving a node
between open groups; `StageGroup` nil-deref on a dangling parent; no-op reparent to root inside
a relationship; inert Expand button; focus loss after ungroup.

## Browser pass (2026-11-20, Playwright against a copy of the self-model)

Fixed in the follow-up commit: Paper edge/handle CSS lost the cascade to xyflow (edges were default
grey 1px; now xyflow CSS variables, contrast-checked); cyclical/backward edges looped and cut
through cards (side-aware routing, lanes, reciprocal offsets); labels hidden under node z-index;
nested groups had identical fill (`--group-2`); stacked card overflowed its group; no tooltips on
icon-only buttons (new `tooltip.tsx`, verified `role=tooltip` + `aria-describedby`); groups not
resizable (right/bottom handles, min = content, sizes in localStorage).
Verified visually: light and dark, nested tint, U-shaped backward edge into Existing App.
**Not yet verified:** group resize by drag (handles were under the Review panel), inspector,
group/ungroup forms, drag-out/in, keyboard focus order.

Open from that pass:
- **Edge labels crowd near short gaps** (Existing App -> Upload Service): pills are wider than
  the gap, so they overlap each other and card text ("presigned upload" and "uses" doubled).
  Ideas: larger ELK layer spacing when labelled, truncate pills with title, collapse colliding
  labels to a count until hover.
- **Right rail covers the canvas**: fit-view pads only the left sidebar, so with Review/Inspector
  open the graph sits under it (the AWS Collapse button was unreachable).
- **Drill rule (decided with you):** Open exists only on summary cards. Expanded groups have no Open
  link and ignore double-click; collapse to a summary card first, then Open. Your last sentence
  about "untoggle example boundary ... then drill down" read as the opposite of this rule, so
  I treated it as a slip; tell me if not.
- Aggregated popover z-order fixed (wrapper raised to 20000 while open).
- **Console noise:** opening an aggregated popover probes each member and logs up to N 404s
  (`/levels/relationships/<id>`). Proper fix: the server returns a per-member `drillable` flag.
- Obstacle avoidance only for forward edges; backward U paths can still cross nodes.
- Resize handles have no aria-label (xyflow limitation); mouse-only.

## Still open (reviewer nits, deliberately not fixed)

- Multi-select drag only reparents the node under the pointer.
- The aggregated edge list sends up to 20 `GET /levels/relationships/{id}` probes per open;
  the server could return a per-member `drillable` flag.
- The first level load often costs 2-3 requests (bounded at 3, no loop).
- Each distinct expanded set writes a `<key>~<hash>.json` cache file; only cleaned on invalidation.
- Dropping a collapsed group onto a container returns 409 if it exceeds depth.
- Ungroup is on the card/container header, not in the inspector.
- The Paper restyle shares a branch with the 0010 feature; consider two PRs (it is its own
  commit, `6cb9055`).

## Blocked

_None._ (Nothing needed a two-attempt skip.)

## Commits

Stage 07 and 08 shipped as two PRs; see `git log` on `main` (the commit hashes recorded earlier in this
file were rewritten before the first push).

## Not done (outside this run)

- Stage 06.7 (iterate the brief against Codex/Cursor/Claude Code) and stage 08 (distribution:
  `go:embed`, `npx` shim) are untouched. No push, no PR, nothing on `main`.
