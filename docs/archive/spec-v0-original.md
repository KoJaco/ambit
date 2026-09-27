# Archived: Original v1 Build Spec (superseded)

## Status

Superseded by [`docs/planning/spec-v1.md`](../planning/spec-v1.md).

## Why this is retained

This is the original build spec as supplied, preserved verbatim. It is kept because the
current spec deviates from it in several material ways — the product name, the frontend
framework, the node type taxonomy, the presence of an LLM client, the size of the MCP
surface, and the introduction of a staging layer. Keeping the original intact means every
one of those deltas can be traced back to what it replaced, rather than being reconstructed
from memory.

Do not treat anything below as current. It describes a product called `archmap` with an
AWS-native vertical wedge and a local LLM proxy, none of which survived scoping.

The deltas are enumerated in [`docs/planning/spec-v1.md`](../planning/spec-v1.md) under
"Deltas from the original spec".

---

## Original text (verbatim)

### Local-First Architecture Model for AI-Assisted Solutioning — v1 Build Spec

#### The One Sentence

A local-first, Git-backed architecture model that an AWS solutions architect builds from a client transcript and hands to coding agents as a source of truth — with a simple local check that stops an agent from committing outside the boundaries the architect drew.

#### Positioning (why this, not IcePanel)

IcePanel already does hosted C4 modelling, drill-down, current/future forking, and an MCP server that answers questions about the model and syncs objects with code. Don't compete on that feature list — a funded team will always out-execute feature-for-feature.

The wedge is posture, not features: local-first, zero-account, offline by default, and a check that can actually stop an agent from committing outside a declared boundary — versus a hosted reference/sync layer. That posture implies a different business model, which is structurally harder for a SaaS collaboration tool to casually adopt than it is for you to build. Keep the enforcement check dumb; don't compete on rule sophistication.

The secondary wedge is vertical: AWS-native node types and a transcript-in solutioning workflow, aimed at solo SAs and small consultancies — underserved by a tool built for long-lived internal team alignment.

#### 1. Core Loop

Client transcript → agent proposes top-level model → architect refines via React Flow editor → architect drills into a node and fleshes it out (single-turn instruction + regenerate + accept/reject) → architect assigns a node to a coding agent → agent calls get_context for its task brief → agent builds → archmap check flags anything outside declared scope before the architect accepts the diff → agent calls update_node_status → merge.

Product principle, unchanged throughout every scoping pass: the human is in the seat driving the agents, not the agents driving the human. Every interaction point above is propose → accept/reject/edit → commit — one interaction pattern, used consistently for both the LLM-assisted editing and the agent-assignment flow.

#### 2. Canonical Model — Storage

Git-committed, canonical:

```
.arch/
├── nodes/
│   ├── <node-id>.json      # one file per node
│   └── ...
├── index.json               # hierarchy (ParentID) + cross-cutting relationships
├── config.json               # non-secret config
└── local.json                 # GITIGNORED — LLM API key lives here only
```

Why one file per node, not a single schema.json: a single large JSON file means every node edit touches the whole file — git diffs stop being scoped to what actually changed, and merge friction increases with graph depth. Per-node files keep diffs scoped to the node that actually changed, which is also closer to how a PR review should look.

Node fields (per file): id, name, type (AWS-native: VPC / service / data store / IAM boundary / account boundary / etc.), parent_id, markdown_spec, implementation (glob list, not a single path), protected (bool), scope (list of paths an assigned agent may touch), status.

No coordinates in the canonical model. This is deliberate — visual layout is a presentation concern, not part of the model's meaning. Leave a comment in the schema itself (// no coordinates by design — see .arch/.cache/layout) so this doesn't get "corrected" later by future-you or an agent that doesn't have this context.

Reserved-but-unbuilt fields: observed_implementation, extended agent_authority beyond protected/scope. Left as optional so the format doesn't need to change shape later — no code built against them in v1.

LLM key handling: the API key lives in .arch/local.json, which is gitignored from the first commit of the template — not config.json, which is committed. Get the .gitignore entry in before the first real commit; this is the one item on this whole list with a "leaked key" failure mode if skipped.

.gitignore automation is mission-critical, not a nice-to-have — treat it as a v1 release gate:

- archmap init must create .gitignore if missing, and append the local.json entry idempotently (check for existing entry before appending; safe to re-run init).
- Injection alone is insufficient. archmap start and archmap check must actively verify at runtime that local.json is actually ignored (git check-ignore) and refuse to start / warn loudly if it isn't — covers init running before git init, a pre-existing .gitignore merging oddly, or the entry being edited out later.
- Required test, as a release gate, not just a unit test: a content-string assertion on .gitignore is not sufficient — the failure mode that matters is the key ending up in a commit. The gating test runs the full init flow in a temp directory, actually git inits, actually git add -As, and asserts local.json does not appear in git status --porcelain. Section 13's build order treats step 1 as incomplete until this specific test passes.

#### 3. Derived / Disposable Layer

Everything below is gitignored, rebuildable from the canonical files, never a source of truth:

```
.arch/.cache/
├── index.db (or in-memory only — see below)
├── layout/<node-id>.json     # cached elkjs positions, per drill-down level
```

In-memory graph index (Go): map[NodeID]*Node keyed by ID, ParentID for hierarchy, a separate adjacency index for cross-cutting relationships (these aren't tree edges — Orders → Payments jumps across the hierarchy). Built once at startup from the .arch/nodes/*.json files, kept in memory, rebuilt on file-watch change. No SQLite — this graph is dozens to low hundreds of nodes for a single client engagement; a flat in-memory map is O(1) at this scale and a database would solve a scale problem this project doesn't have.

Layout cache: elkjs computed once per drill-down level, positions persisted to .arch/.cache/layout/, invalidated and recomputed only on structural change (node added/removed/reparented) — not on every load. Manual drag-to-reposition in the UI writes into this same cache. This keeps the canvas visually stable session to session instead of jittering, while the canonical model stays coordinate-free.

#### 4. Layout Algorithm

elkjs, not dagre. Rationale: elk has native support for hierarchical/compound nodes (a node containing its own subgraph), which matches the drill-down structure directly — a node at one level is a nested graph at the next. Dagre treats layout as a flat DAG and would need workarounds to fake compound nesting.

Compute per drill-down level (i.e., a node's direct children + the relationships among them), not the whole graph flattened at once — this keeps layout computation and the resulting cache scoped to what's actually being viewed.

#### 5. Frontend Rendering Strategy

React Flow is fed only the current drill-down level's nodes/edges — a scoped query (GetChildren(nodeID) + relationships among them) against the in-memory index, never a client-side filter over the entire graph. This is the actual performance-relevant decision in the whole architecture; everything else at this scale is comfortably fast by default.

#### 6. Component Architecture

```
                    ┌─────────────────────────┐
                    │   archmap-core (Go pkg)  │
                    │  - schema read/write      │
                    │  - in-memory graph index   │
                    │  - graph mutations          │
                    │  - elkjs layout + cache      │
                    │  - git diff → node mapping    │
                    │  - scope/protected check       │
                    └────────────┬────────────────────┘
                                 │
                 ┌───────────────┴───────────────┐
                 │                                │
        ┌────────▼────────┐              ┌────────▼────────┐
        │  Local HTTP API   │              │   MCP server      │
        │  (Go, localhost)   │              │  (Go, stdio)        │
        │  - serves Next.js    │              │  - get_context       │
        │  - graph CRUD          │              │  - update_node_status │
        │  - LLM proxy              │              │  used by Claude Code, │
        │    (key from local.json)   │              │  Cursor, etc.           │
        └────────┬─────────────┘              └──────────────────────────┘
                 │
        ┌────────▼─────────┐
        │  Next.js frontend  │
        │  (static export)     │
        │  React Flow + elkjs     │
        └───────────────────────┘
```

One rule: all graph mutation logic lives in archmap-core, called by both front doors. The UI editing a node and an agent creating a node via MCP go through the identical validation and file-write path — no drift between two implementations of "what a valid mutation is."

#### 7. Distribution

- npx archmap is a thin shim that downloads/runs a platform-specific Go binary — not a Node runtime dependency.
- next build → static export, go:embed-ed directly into the Go binary.
- archmap start is a single binary: serves the static frontend, runs the local HTTP API, and (optionally, or via subcommand) the MCP server. No Node runtime required on the user's machine at all.
- Practical consequence for the frontend: build it as a fully client-side Next.js app — no server components/API routes doing real work, since that logic lives in Go. Static export needs to stay straightforward.

#### 8. MCP Interface

Three tools, all thin wrappers over archmap-core:

- get_context(node_id) — returns the node's scope, interfaces, constraints, and spec formatted as a ready-to-use task brief, not a raw data dump. This is what an assigned coding agent receives.
  Prompt engineering here matters and needs real iteration, not a one-shot template. Explicitly enumerate the allowed paths (from scope) and explicitly name protected paths / sibling nodes as off-limits — don't just omit them. Include sibling nodes' interfaces without their implementation detail, so the agent has enough context to call into a neighboring system correctly without reaching inside it. Frame it imperatively: the agent may only modify files matching the given globs, and if the task requires more, it should stop and report back rather than push through.
- check_scope(node_id, files) — exposes the same CheckScope logic archmap check runs against a git diff, callable mid-task by the agent before it's gone too far. Costs almost nothing to build (same underlying function as archmap check, reached over MCP instead of git diff), and gives a compliant agent fast feedback after touching a few files instead of only at the end of a task. The get_context brief should instruct the agent to call this periodically or before finishing.
- update_node_status(node_id, status) — one field write, called by the agent on completion.

Enforcement is layered, not single-point: get_context framing reduces the rate of violations but can't constrain an agent's native file-editing tools, which aren't gated by MCP at all — good instructions lower frequency, they don't close the gap. check_scope catches it early for agents that use it voluntarily. archmap check (Section 10) is the mandatory backstop that catches everything regardless of agent behavior, and stays required even as the other two reduce how often it actually fires.

Transport: stdio, matching how Claude Code/Cursor expect local MCP servers to be invoked (a direct command, not a running HTTP server). Run as archmap mcp subcommand.

#### 9. Local HTTP API + LLM-Assisted Editing

- Thin REST/JSON wrapper over archmap-core for the frontend: graph CRUD, drill-down navigation queries.
- LLM proxy endpoint: frontend never calls the Anthropic API directly — always through this local endpoint, so the key (.arch/local.json) never reaches the browser.
- Interaction pattern: single-turn instruction + regenerate + accept/reject, not open-ended chat. At the current scope (top-level, or a drilled-into node), one text input, one instruction, a diff-style preview of the proposed change against current state, then accept/reject/edit. No thread, no message history, no multi-turn context to manage.
- Deferred, explicitly: a full conversational/Assistant-UI-style multi-turn panel. Only build this if, after real use, the single-turn-plus-regenerate loop is a proven bottleneck — not before. This was the one piece of scope actively cut this week to keep v1 buildable.

#### 10. Enforcement — archmap check

- Standalone command (or subcommand), runnable manually or as a git pre-commit hook.
- git diff --name-only → map touched files to nodes via each node's implementation glob list → if a file belongs to a protected node, or falls outside the scope declared for the node an agent was assigned, fail loudly, name the node and rule hit.
- Warns by default, does not block. Exit code 0 unless a future --strict flag is explicitly added for anyone who wants it wired into a real blocking pre-commit hook. The architect's git diff review remains the actual judgment call — this surfaces what to look at, it doesn't replace review.
- Keep this dumb. The differentiation is the local/offline/default-on posture, not rule sophistication — resist adding a policy language or more rule types.

#### 11. CLI Surface

```
npx archmap init     # scaffold .arch/, .gitignore entry for local.json
npx archmap start    # serves frontend + local API (+ MCP, optionally)
npx archmap mcp       # MCP server, stdio, for coding-agent clients
npx archmap check      # enforcement check against current git diff
```

#### 12. Explicitly Not v1

Deferred with intent — natural v2+ items once the core loop is proven on real client work, not abandoned:

- Codebase scanner (bootstrapping from existing repos) — sidesteps a crowded part of the market; revisit only for brownfield engagements.
- Full governance/permission model beyond protected + scope — no scope-expansion requests, no per-agent roles.
- Human-escalation UI — git diff review is the approval workflow for a solo user.
- Multi-agent orchestration — one agent, one assignment, one reviewer, at a time.
- CI enforcement — archmap check runs locally, pre-accept, not in a pipeline; CI is a team-tier concern.
- Cloud sync, accounts, multiplayer, billing — precisely what the local-first posture is positioned against.
- Freeform NL-to-architecture generation — transcript-seeded only.
- Conversational multi-turn editing UI (Assistant UI or similar) — single-turn + regenerate only, until proven insufficient.
- SQLite or any external database — in-memory Go structure, rebuilt from committed files.

#### 13. Suggested Build Order for the Week

1. .arch/ schema + node/index file format, archmap init scaffold, .gitignore for local.json. Not complete until the init → git init → git add -A → local.json absent from git status --porcelain integration test passes (see Section 2) — this step is a release gate, not just a code write.
2. archmap-core: file read/write, in-memory index, basic mutations (create/update/delete node, set scope/protected).
3. archmap check: git diff → node mapping → violation report. (Small, high-value, validates the core differentiator early.)
4. Local HTTP API: graph CRUD over archmap-core.
5. Wire existing React Flow frontend to the new API + schema fields (scope, protected in node inspector); scope rendering to current drill-down level only.
6. elkjs layout integration + position caching in .arch/.cache/layout/.
7. LLM proxy endpoint + single-turn instruction/regenerate UI for transcript-seeding and node refinement.
8. MCP server (get_context, check_scope, update_node_status), stdio, tested against Claude Code/Cursor directly — budget real iteration time on get_context's prompt framing, not a one-shot template.
9. go:embed static frontend into the binary; npx shim for distribution.

Steps 1-3 are the part worth getting right slowly — everything else builds on the schema and core package holding up.
