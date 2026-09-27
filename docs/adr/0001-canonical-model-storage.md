# ADR-0001: Canonical Model Storage and Format

## Status

Accepted

## Date

2026-09-27

## Context

ambit's entire value rests on the architecture model being a source of truth that a human
reviews in git and an agent consumes as a brief. That forces the storage format to satisfy
constraints most data formats do not have to care about:

- **Diffs are the review surface.** The architect approves agent work by reading
  `git diff`. Any format decision that makes a diff harder to read directly degrades the
  product's core workflow.
- **The graph is small.** Dozens to low hundreds of nodes for a single client engagement.
  Optimising for scale that will not arrive costs complexity now for nothing later.
- **Two independent processes write to it.** The HTTP API and the MCP server both mutate
  `.arch`, and neither can assume the other is running.
- **The format will grow.** Fields are already anticipated (`observed_implementation`,
  extended `agent_authority`) that v1 will not implement.

This ADR consolidates every decision about what is on disk and what shape it takes. The
runtime and process decisions are in [ADR-0003](0003-runtime-and-distribution.md); the
agent interface is in [ADR-0002](0002-agent-interface-and-review-gate.md).

## Decision

### One file per node, not one model file

The canonical model is `.arch/nodes/<node-id>.json`, one file per node, plus `index.json`
holding hierarchy and cross-cutting relationships.

### Prose lives in a sibling `.arch/nodes/<node-id>.md`

Structured fields stay in the JSON. The node's written specification — the part a human
actually authors and an agent actually reads — is a separate markdown file next to it.

### Node IDs are immutable human-readable slugs

Derived from the node's name at creation (`orders-service`), immutable thereafter, numeric
suffix on collision (`orders-service-2`). A rename changes `name`, never `id`.

### No coordinates in the canonical model

Layout positions are never written to `.arch/nodes/` or `index.json`. They live in the
gitignored `.arch/.cache/layout/`, keyed by drill-down level. The format documentation
carries an explicit note saying the omission is deliberate.

### Node `type` is a free-form string

No enum, no registry, no validation. The UI matches known strings to icons on a best-effort
basis and falls back to a generic shape. An unrecognised type is never an error.

### `schema_version` appears once, in `index.json`

Node files inherit it. Reserved-but-unbuilt fields are documented as optional, and no v1
code is written against them.

### The derived layer is gitignored and disposable

`.arch/.cache/` (layout) and `.arch/.proposals/` (staged mutations) are rebuildable or
discardable, never a source of truth. `local.json` holds machine-local state — the active
assignment, UI preferences — and is likewise gitignored.

### The `.gitignore` guarantee is a release gate with an integration test

`ambit init` writes the entries idempotently; `ambit start` and `ambit check` verify at
runtime with `git check-ignore`; and a gating integration test runs a real `init`, a real
`git init`, a real `git add -A`, and asserts the ignored paths do not appear in
`git status --porcelain`.

## Alternatives Considered

- **A single `schema.json` for the whole model** — every node edit rewrites the whole file,
  so diffs stop being scoped to what changed and merge friction grows with graph depth.
  A thirty-node proposal would produce one unreadable diff instead of thirty readable ones.
  Rejected.

- **Inline `markdown_spec` field in the node JSON** (what the original spec specified) — a
  node's full prose spec becomes a single escaped one-line string. It diffs as one changed
  line no matter how much of the prose moved, and it cannot be edited comfortably in any
  editor. This actively defeats the per-node-file rationale, which is the same argument one
  level down. Rejected.

- **Markdown with YAML frontmatter, one file per node** — genuinely attractive, and keeps a
  node to a single file. Rejected because two machine writers (HTTP API and MCP server) must
  round-trip the structured half without disturbing the prose half, and frontmatter
  round-tripping through a YAML library reorders keys and mangles formatting in ways JSON
  does not. The two-file split keeps machine-written and human-written content in separate
  files with separate serialisation concerns.

- **UUID node IDs** — stable and collision-free by construction, at the cost of filenames
  and diffs nobody can read, and MCP calls nobody can sanity-check by eye. The collision
  problem a UUID solves does not exist at this scale. Rejected.

- **Hierarchical path IDs** (`platform/orders/api`) — encodes the tree in the ID, which
  means reparenting a node changes its ID and every reference to it. Rejected.

- **Coordinates in the canonical model** — would make every drag of a box a committed model
  change, filling the architect's diff review with noise that has no architectural meaning.
  Rejected.

- **A closed enum of node types** — was the original spec's AWS-native taxonomy. Rejected
  along with the vertical positioning; see
  [decision note 0002](../decisions/0002-node-type-taxonomy.md).

- **A validated open registry of node types** — a shipped list of known types with warnings
  for unknowns. Rejected for v1 as premature: there is no evidence yet about which types
  recur across real models, and a registry written before that evidence is a guess that
  users then have to work around.

- **`schema_version` on every node file** — makes a node self-describing in isolation, at
  the cost of a field repeated across every file that must be kept in sync during a bump.
  Rejected; nodes are never read outside the context of their `index.json`.

- **SQLite for the index** — solves a scale problem this project does not have, and
  introduces a migration story. Rejected.

## Consequences

- **Pros**
  - A node-scoped change produces a node-scoped diff, which is what makes `git diff` viable
    as the approval workflow.
  - Prose is editable directly in any editor, and reviewable as real markdown in a diff.
  - Slug IDs make `parent_id` references, relationship entries, filenames, and MCP call
    arguments all readable without a lookup.
  - Coordinate-free canonical files mean visual fiddling never appears in a review.
  - Free-form types mean ambit works in any domain from day one, with zero taxonomy
    maintenance.

- **Cons**
  - Two files per node is more filesystem bookkeeping: create, delete, and rename must keep
    the pair consistent, and an orphaned `.md` with no `.json` is a state that has to be
    handled.
  - Immutable slug IDs drift from node names after a rename, so `orders-service.json` may
    eventually hold a node named something else. Accepted deliberately: a stable identifier
    is worth more than a self-describing one, and the UI displays `name`, not `id`.
  - Free-form types give no autocomplete, no consistency enforcement, and a model can end up
    with `database`, `datastore`, and `data store` as three distinct types. This is a real
    cost, consciously taken in exchange for not guessing a taxonomy.
  - Reading the whole model is N file reads rather than one. Irrelevant at this scale, and
    it happens once at startup.

- **Follow-ups / TODOs**
  - The format contract is [`docs/contracts/arch-model-format.md`](../contracts/arch-model-format.md);
    it must stay in step with any schema change and with `schema_version`.
  - Revisit a node type registry once several real client models exist and the recurring
    types are observable rather than imagined.

## References

- Canonical spec: [`docs/planning/spec-v1.md`](../planning/spec-v1.md) Sections 2 and 3
- Contract: [`docs/contracts/arch-model-format.md`](../contracts/arch-model-format.md)
- Superseded source: [`docs/archive/spec-v0-original.md`](../archive/spec-v0-original.md) Section 2
- Related: [decision note 0002](../decisions/0002-node-type-taxonomy.md)
