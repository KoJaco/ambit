# ADR-0003: Runtime, Process Model, and Distribution

## Status

Accepted

## Date

2026-09-27

## Context

ambit presents two front doors onto the same model: a local HTTP API serving the architect's
browser UI, and an MCP server spoken to by a coding harness over stdio. Both read and write
`.arch`. Neither can assume the other is running — the harness owns the lifecycle of the MCP
process, and the architect owns the lifecycle of the UI server, and those are independent
events.

The distribution constraint is sharp: the local-first, zero-account posture is undermined if
installation requires a toolchain. A user who has to install Node, or Go, or a package
manager before they can evaluate the tool has already met more friction than a hosted
competitor's signup form.

And there is a layout problem specific to the model's shape. Drill-down means a node at one
level *is* a nested graph at the next, so the layout engine has to reason about compound
nodes natively or be worked around.

## Decision

### `ambit-core` is a Go package holding all mutation logic

Schema read/write, the in-memory graph index, graph mutations, layout cache management,
proposal staging and apply, git-diff-to-node mapping, and the scope/protected check all live
in one package. Both front doors call it.

**One rule: there is never a second implementation of "what a valid mutation is."** The UI
editing a node and an agent creating a node over MCP traverse the identical validation and
file-write path.

### The graph index is an in-memory Go structure, rebuilt from files

`map[NodeID]*Node` keyed by ID, `parent_id` for hierarchy, and a *separate* adjacency index
for cross-cutting relationships — these are not tree edges, since `orders → payments` jumps
across the hierarchy and would corrupt a structure that assumed a tree. Built at startup
from `.arch/nodes/*.json`, held in memory, rebuilt on file-watch change.

No SQLite. No database of any kind.

### The two front doors are fully independent processes

`ambit mcp` is spawned by the harness over stdio, reads and writes `.arch` directly, and
does not require or proxy through `ambit start`. Each process watches the filesystem for
changes made by the other.

### Layout is elkjs, computed per drill-down level

Not dagre, and not over the whole graph flattened.

### Change notification to the browser is SSE

`fsnotify` in Go, pushed to the browser over a single one-way `/events` stream.

### Distribution is a single Go binary with the SPA embedded

`react-router build` produces a static client bundle, `go:embed`-ed into the binary.
`npx ambit` is a thin shim that downloads and runs the platform-specific binary — it is not
a Node runtime dependency, and no Node runtime is required on the user's machine.

## Alternatives Considered

- **Duplicating mutation logic in each front door** — never seriously considered, but worth
  recording as the thing the one-rule constraint exists to prevent. Two implementations of
  validation drift, and the drift shows up as an agent being allowed to write something the
  UI would have rejected.

- **SQLite for the graph index** — rejected. The graph is dozens to low hundreds of nodes
  for one client engagement; a flat map is O(1) at that scale. A database would add a
  schema migration story, a file lock contended by two independent processes, and a second
  source of truth that can disagree with the files. It solves a scale problem this project
  does not have.

- **A single combined adjacency structure for hierarchy and relationships** — rejected
  because the two edge kinds have genuinely different semantics. Hierarchy is a tree and
  drives drill-down; cross-cutting relationships form an arbitrary graph and drive rendered
  edges. Collapsing them means every traversal has to filter by edge kind, and the tree
  invariant stops being structurally enforced.

- **`ambit mcp` as a thin client proxying to a running `ambit start`** — tempting because it
  gives one process owning the file watch and one in-memory index. Rejected because it makes
  MCP fail whenever the architect has not started the UI, or has closed the browser and shut
  the server down. The harness spawns MCP on its own schedule; a dependency on a
  separately-launched server turns a routine action into a confusing error.

- **`ambit start` hosting MCP over HTTP, with the stdio subcommand as a bridge** — same
  coupling objection, plus it contradicts how Codex, Cursor, and Claude Code expect to
  invoke local MCP servers, which is a direct command over stdio.

- **dagre for layout** — rejected. Dagre treats layout as a flat DAG with no native compound
  node support, so nested subgraphs have to be faked. elk supports hierarchical and compound
  nodes natively, which maps directly onto the drill-down structure rather than against it.

- **Laying out the whole graph at once** — rejected in favour of per-level computation,
  which keeps both the work and the resulting cache scoped to what is on screen, and makes
  cache invalidation a per-level rather than global concern.

- **WebSocket instead of SSE** — rejected. The traffic is strictly one-directional, the
  browser already has the HTTP API for anything it needs to send, and SSE reconnects
  automatically where a WebSocket needs reconnection logic written and debugged. The
  bidirectionality a WebSocket buys is not needed by anything in v1.

- **Browser polling a `/revision` endpoint** — rejected. Simplest of all, but it trades a
  permanently-running poll loop for latency that is visible when an agent stages a proposal
  and the architect is watching the screen.

- **Requiring Node at runtime** — rejected; it contradicts the zero-friction install that
  the local-first posture depends on.

- **Distributing via Homebrew, Go install, or a downloadable release only** — reasonable
  additions later, but `npx` is where this audience already lives and it requires no prior
  setup step.

## Consequences

- **Pros**
  - A single binary with no runtime dependency makes the install a one-liner, which is what
    a zero-account posture needs to be credible.
  - The in-memory index has no migration story, no lock contention, and cannot disagree with
    the files, because it is derived from them on every change.
  - Independent processes mean MCP works whether or not the UI is running, which matches how
    an architect actually moves between windows.
  - elk's compound node support means drill-down layout is expressed directly rather than
    worked around.
  - SSE is materially less code than a WebSocket for strictly less capability that is not
    needed.

- **Cons**
  - Two processes each hold their own in-memory index of the same files, so there are two
    copies that converge via the filesystem rather than one authoritative copy. Convergence
    is only as good as the file watching.
  - Concurrent writes from both processes are possible. v1 relies on writes being small,
    scoped to distinct files, and rare; there is no cross-process locking. This is a known
    thin spot rather than a solved problem.
  - elkjs is a JavaScript library, so layout runs in the browser rather than in the Go core.
    The Go side manages the cache but does not compute it, which means layout cannot be
    computed headlessly by the CLI.
  - Embedding the SPA means any frontend change requires rebuilding the Go binary; the two
    cannot be released independently.
  - The npx shim has to handle platform detection, download, caching, and checksum
    verification — a small but real piece of distribution infrastructure to own.

- **Follow-ups / TODOs**
  - Decide on a cross-process write strategy if concurrent-write corruption is ever observed
    in practice. Candidates: a lock file, or routing all writes through whichever process
    holds a lease.
  - The npx shim needs checksum verification on download; do not ship it without.

## References

- Canonical spec: [`docs/planning/spec-v1.md`](../planning/spec-v1.md) Sections 3, 4, 6, 7
- Architecture: [`docs/architecture/system-overview.md`](../architecture/system-overview.md)
- Contract: [`docs/contracts/local-http-api.md`](../contracts/local-http-api.md)
- Related: [ADR-0001](0001-canonical-model-storage.md) for what is on disk,
  [ADR-0004](0004-frontend-platform.md) for the embedded bundle
