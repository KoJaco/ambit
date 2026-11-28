# Contract: MCP Tool Surface

## Purpose

Defines the tools ambit exposes over MCP, and the semantics a coding harness can rely on.
This is the boundary between ambit and every agent that touches the model — Codex, Cursor,
Claude Code, or anything else speaking MCP.

Two distinct consumers use this surface for different things, and the split matters:

- **The architect's harness**, used to author the model from a transcript or to flesh out a
  node. Calls the staged authoring tools.
- **An assigned coding agent**, building the thing a node describes. Calls `get_context`,
  `check_scope`, and `update_node_status`.

## Scope

**In scope:** all seventeen tools, their arguments, return shapes, staging behaviour, and the
framing requirements for `get_context`.

**Out of scope:** the MCP protocol itself, the proposal file format (see
[`proposals.md`](proposals.md)), and the model file format (see
[`arch-model-format.md`](arch-model-format.md)).

## Versioning

- **Current version:** v1
- **Compatibility:** Additive. New tools and new optional arguments may be added; existing
  tool names, required arguments, and staging behaviour may not change meaning.
- **Change policy:** Moving a tool between staged and direct is a breaking change to the
  product's safety posture and requires an ADR, not a version bump.

## Definitions

- **Staged** — the tool's effect is written to `.arch/.proposals/`, not to the canonical
  model. It takes effect only when the architect accepts it in the UI.
- **Direct** — the tool writes to the canonical model immediately.
- **Assignment** — the node currently assigned to an agent, recorded in the gitignored
  `local.json`. Determines whose `scope` applies during `ambit check`.
- **Brief** — the formatted, imperative task description `get_context` returns. Not a data
  dump.

## Contract Shape (Conceptual)

Transport is **stdio**, invoked as `ambit mcp`. This matches how Codex, Cursor, and Claude
Code expect to launch a local MCP server: a direct command, not a running HTTP server.

The process is **independent of `ambit start`**. MCP works whether or not the UI server is
running. See [ADR-0003](../adr/0003-runtime-and-distribution.md).

All twenty-six tools are thin wrappers over `ambit-core`, sharing the identical validation and
write path as the HTTP API. There is never a second implementation of what a valid mutation
is.

---

### Authoring tools (staged)

All of the authoring tools write to `.arch/.proposals/<proposal-id>/` and return the proposal ID. **None of
them modify the canonical model.** Every one of them must say so in its returned message, so
an agent does not report to its user that a change has been made when it has only been
proposed.

**Graph modeling.** Authoring harnesses should follow the **Modeling guidance** section in
[`arch-model-format.md`](arch-model-format.md) (hierarchy vs relationships, reachability,
external actors, entry→module patterns). That guidance is **not** injected into
`get_context` for implementation agents — see
[decision 0008](../decisions/0008-arch-graph-modeling-guidance.md). MCP tool descriptions
for `seed_model`, `create_node`, `update_node`, and `set_relationship` summarise the same rules
and end with one shared line: *"Prefer shallow models. Use a container node to group related
nodes. Nest more than three levels only for genuinely separate subsystems."* (decision 0010;
`core.ShallowModelGuidance`). The `get_context` brief carries the same line as a single closing
sentence, the one exception to the "no graph doctrine in the brief" rule. The tool surface is
unchanged: group and ungroup are canvas operations that stage ordinary create/update/delete
proposals.

#### `seed_model(transcript, parent_id?, nodes?, relationships?, updates?, delete_relationships?, delete_nodes?)`

Propose a model from a client transcript. The headline authoring tool: the architect pastes
or points their harness at a transcript, and the harness produces a set of nodes and
relationships. ambit does not call a model provider; the harness supplies the graph.

- `transcript` — the transcript text. Required.
- `parent_id` — optional. When present, proposed nodes without their own `parent_id` or
  `relationship_id` become children of that node.
- `nodes` — array of node payloads (`name`, `type`, optional `id`, `parent_id`,
  `relationship_id`, `implementation`, `scope`, `protected`, `spec`, `icon`, `depends_on`,
  `contract`). An optional
  `id` is a caller-chosen slug (see `create_node`); other nodes and `relationships` in the
  same call may reference it.
- `relationships` — optional array of edges (`from`, `to`, optional `label`, `kind`,
  `contract`, optional `relationship_id` when updating an existing edge).
- `updates` — optional array of `update_node` payloads (same fields and rules as
  `update_node`).
- `delete_relationships` — optional array of relationship ids to delete.
- `delete_nodes` — optional array of node ids to delete (leaf nodes only, as `delete_node`).

At least one of `nodes`, `updates`, `delete_relationships`, `delete_nodes` is required.
Operations are staged in the order creates, updates, relationships, relationship deletes,
node deletes, so a restructure (reparent children, then delete the old container) is one
proposal. Accept-all on a proposal is all-or-nothing for failures (see
[`proposals.md`](proposals.md)).

Returns the proposal ID and a summary of what was proposed — node count, names.

When proposing `relationships`, use them for runtime or logical flow at a level, not for
containment (use `parent_id` on nodes). Mirror patterns in the reference `.arch` graph
(`canvas-data-flow`, `agent-interface-data-flow` under `platform`).

This tool exists **only** for transcript-seeded generation. Freeform
natural-language-to-architecture is explicitly out of scope for v1; see
[`docs/planning/spec-v1.md`](../planning/spec-v1.md) Section 12.

#### `create_node(name, type, id?, parent_id?, relationship_id?, implementation?, scope?, protected?, spec?, icon?, depends_on?, contract?)`

Propose one new node. `spec` is the markdown prose that becomes the node's sibling `.md`.
Optional `icon` must be an id from the closed registry in
[`arch-model-format.md`](arch-model-format.md). Colour, canvas `renderer`, and `schema` rows are architect-only (HTTP inspector).
The node's `id` is derived from `name` by `ambit-core` unless the caller supplies `id`: a
lowercase slug (letters, digits, single hyphens). A supplied `id` is rejected when it is not a
slug, when a node already has it, or when a pending proposal reserves it, so agents can name
a node and reference it later in `parent_id` or edges. Optional `relationship_id` attaches the node to a relationship interior (mutually
exclusive with `parent_id`).

Optional build-planning fields (all bounded, defined in [`arch-model-format.md`](arch-model-format.md)):
`external` (bool), `human_gates` (`[{kind, note?}]`, kind `decision|credential|phone|deploy_approval|install`),
`deliverables` and `references` (repo-relative paths), `shared` (bool), `milestones`
(`[{id, label?, status?}]`) and `gates` (ids of nodes that wait for this one). They are accepted
by `create_node`, `update_node` (omitted means unchanged, `[]` clears a list, `false` clears a
bool) and each `seed_model` node or update. A `gates` id must exist and must not create a
dependency cycle.

Optional `depends_on` is an array of node ids that must be built first. Set it only for real
build ordering, never to mirror flow direction (relationships express flow). Ids must exist (or
be created earlier in the same proposal); a cycle is rejected. Optional `contract` is
`{payloads?: string, invariants?: string[], errors?: string[], owns_data?: string[]}`: put
interfaces here rather than in `spec` prose. Limits and validation are in
[`arch-model-format.md`](arch-model-format.md); violations are rejected at proposal time.

#### `update_node(node_id, ...fields)`

Propose changes to an existing node. Any subset of mutable fields: `name`, `type`,
`parent_id`, `relationship_id`, `implementation`, `scope`, `protected`, `spec`, `icon`,
`depends_on`, `contract`. Registry ids are documented in [`arch-model-format.md`](arch-model-format.md). `color`,
`renderer`, and `schema` are not settable via MCP.

`depends_on` and `contract` are pointers in effect: omitted means unchanged, a present value
replaces, and `[]` / `{}` clears. `status` is not settable here (the error names
`update_node_status`); an `id` field is refused with an error that names `new_id` and
`rename_node`; a `relationship_id` that does not exist is refused with an error that names
`set_relationship`. Every staged operation is still cycle-checked on its own as it is
staged. Within a contiguous run of `update_node` operations, those that move a node
(`parent_id`) are ordered first, so a move plus a `depends_on` change in one proposal works
whichever order they are written in. A set of changes that is only valid when applied in one
particular order (for example swapping two nodes' parents or dependencies) can fail the
per-operation checks; split it into separate proposals, each valid on its own.

`new_id` renames the node: it stages an `update_node` for the other fields in the call (when
there are any) followed by a `rename_node`, as in `rename_node` below.

#### `rename_node(node_id, new_id)`

Propose changing a node id. Accepting rewrites the id in the node file names (`.json` and
`.md`), `parent_id` and `depends_on` of other nodes, relationship endpoints, the active
assignment (`local.json`), cached layouts (the node's own level moves to the new key; hashed
variants are dropped; edge attachments of a layout that held the node are dropped) and the
spec file. `implementation` globs and prose are left unchanged. Rejected at staging with
`ErrExists` when `new_id` is taken, is a relationship id, or is reserved by a pending
`create_node` or `rename_node` of another proposal; `ErrBadOperation` when it is not a slug (max 64 characters) or
equals the current id. Inside one proposal every other operation refers to the node by its
**current** id: renames are ordered last. Other pending proposals that name the old id go
stale (node missing). A failed accept restores every touched file.

#### `withdraw_proposal(proposal_id)`

Withdraw a pending proposal. Only the MCP session that staged it may do so (each session has
a random origin id stored in the manifest, never shown in the UI); proposals staged from the
UI or by another session are refused with `ErrBadOperation`. The proposal stays queryable as
`withdrawn`. The architect can always delete any proposal in the UI.

#### Proposal fields on every staging tool

`seed_model`, `create_node`, `update_node`, `rename_node`, `delete_node`, `set_relationship`,
`delete_relationship` and `set_constraints` also accept:

- `change_set` (slug, max 64): groups related proposals. Stored in the manifest and returned
  as `change_set` by `list_proposals`, `GET /proposals`, `GET /proposals/{id}`, `get_proposal`
  and the proposal state. Display and accept/reject grouping in the UI is backend-ready; the
  UI does not group yet.
- `supersedes` (proposal id): a pending proposal staged by the same session (same origin;
  the staging tools may differ). After the new proposal stages successfully the old one is
  withdrawn, and its pending `create_node` ids are free for the new one to reuse. A failed
  staging leaves the old proposal in place.
- `depends_on_proposal` (proposal id): the new proposal is validated against the model as it
  will be once that pending proposal is accepted (its creates, updates, deletes, renames and
  edges are visible; base hashes match its staged files). It is stored as
  `depends_on_proposal` and accepting any of its operations is refused (`ErrStale`) while the
  named proposal is still pending, and also when it was withdrawn, superseded or fully
  rejected (re-stage against the current model). Not modelled for the predecessor:
  `delete_relationship`, `set_constraints` and multi-level chains (a predecessor that itself
  depends on another pending proposal). It cannot equal `supersedes`.

#### `delete_node(node_id)`

Propose deleting a node. Deleting a node with children is rejected at proposal time rather
than at apply time, so the agent gets the error while it still has context, instead of the
architect discovering it at review.

#### `delete_relationship(relationship_id)`

Propose deleting one edge. Rejected at proposal time when the edge does not exist. Goes stale
if the edge's `from` node is edited before review. Applying it also removes any nodes inside
the edge (relationship interior); the endpoint nodes are kept.

#### `set_relationship(from, to, label?, kind?, relationship_id?, contract?)`

Propose creating or modifying a cross-cutting relationship. Directed. `kind` is one of
`sync`, `async`, `data`. Omit `relationship_id` to upsert: an existing edge (on disk or
earlier in the same proposal) with the same `from`, `to` and `label` is updated in place
(an omitted `kind` keeps its kind), otherwise a new edge is created, so seeding the same
edge twice yields one edge; note that distinct unlabelled `A -> B` edges therefore merge
into one. Provide `relationship_id` to update an existing edge, including changing its
`from`/`to` endpoints (staged path only; the direct HTTP `PUT /relationships` never moves
endpoints). `seed_model` relationships accept `relationship_id` too.

Optional `contract` is `{format?: "openapi"|"jsonschema"|"text", ref?: string, text?: string}`:
what is on the wire, stored as opaque text. Omitted leaves an existing edge's contract
unchanged; `{}` clears it. The op goes stale if the edge's own JSON (including its contract)
changed since staging.

Do not add relationships merely to connect every sibling; add them where the architect needs
to see control or data flow. Endpoints must reference existing node ids.

---

### Read and report tools (direct)

Authoring guidance carried in tool descriptions and the `get_context` brief: model high-level
first; containers are deployment zones; nest about three levels; ask the user how they want the
top-level view before drilling down; add no edge that carries no information; set explicit ids
on created nodes; call `list_icons` before choosing icons.

#### `get_context(node_id)`

Returns the assigned agent's task brief. **This is the most important tool in the surface**,
and the one whose output quality determines how often the enforcement backstop has to fire.

It returns a formatted, imperative brief — not a serialised node. A raw data dump would make
the agent infer its constraints, and inference is exactly what should not be happening here.

**Required framing.** The brief must:

- **Enumerate the allowed paths explicitly**, from the node's `scope` (defaulting to
  `implementation` when empty). List the globs; do not describe them in prose.
- **Name protected paths and sibling nodes as off-limits explicitly.** Do not merely omit
  them. An agent that has not been told a path is forbidden will treat it as unmentioned
  rather than prohibited, and unmentioned reads as permissible.
- **Include sibling nodes' interfaces without their implementation detail.** The agent needs
  enough to call into a neighbouring system correctly, and nothing that invites it to reach
  inside one.
- **Be imperative about the boundary and the escape hatch.** The agent may modify only files
  matching the given globs; if the task appears to require more, it must stop and report
  back rather than push through. The escape hatch has to be stated, or a capable agent will
  reason its way past the constraint to complete the task.
- **Instruct the agent to call `check_scope`** periodically during the task and before
  finishing.

**Brief sections, in order** (decision 0012): scope, protected paths and siblings; **Project
rules** (the `config.json` constraints, `must` first, present whenever any exist);
**Interfaces you must honour** (only relationships that carry a contract, marked provider or
consumer; edges without one are omitted, an explicit refinement recorded in decision 0012;
no per-interface cut; only a very large safety bound of 60000 characters, decision 0014);
siblings show their full contract; **your node's contract** (never truncated); **Build order** (your
direct dependencies and dependents only, unmet dependencies, container-aware done rule); the
`check_scope` instruction; **verify commands**; **deliverables, references, milestones, human
gates (with cleared state) and the nodes you gate**, when the node declares them; the
**risk and review** line that links risk to how you finish (`risk: high` finishes at
`needs_review`); and a closing line to run `check_done` and `update_node_status` when done,
followed by the subagent guidance. `ambit brief export [--node id|--all] [--out dir] [--force]`
writes the same briefs to `.arch/briefs/<node>.md` (marked as generated; a file without the
marker is not overwritten unless `--force`; externals and pure containers are skipped by
`--all`).

**This framing needs real iteration against actual harnesses, not a one-shot template.**
Build step 9 budgets time for it. Treat the wording as a tuned artifact and record what was
learned when it changes.

#### `check_scope(node_id, files)`

Runs the same `CheckScope` logic `ambit check` applies to a git diff, against a caller-
supplied file list. Callable mid-task, before an agent has gone too far.

- `node_id` — the node whose scope applies.
- `files` — list of paths the agent has touched or intends to touch.

Returns, per file: allowed, or a violation naming the node and the rule hit (`protected`, or
outside `scope`). Protection keeps other nodes out of a protected node's files: the node
assigned to a protected node may edit that node's own files (the `protected` rule is skipped
when the protected owner is the assignee), but nothing else, and with no assignment protected
files stay off-limits.

This costs almost nothing to build — it is the same underlying function reached over MCP
instead of from a diff — and it converts end-of-task failure into mid-task correction for any
agent that uses it.

#### `set_constraints(constraints)`

Staged, replace-all. `constraints` is the full list of project rules, each
`{id, text, severity?}` (`id` a unique slug, `text` 1..300 characters, `severity` `must`
(default) or `should`; at most 24). Applying it rewrites `config.json` `constraints`. The
response says the change is proposed and NOT applied. The architect edits constraints
directly in the UI (`PUT /constraints`); agents never write them directly.

#### `get_build_order(scope_node_id?, compact?, ready_only?)`

Direct, read-only. Returns the advisory build order derived from `depends_on` and `gates`:

```
{scope?,
 order: [{id, name, status, done, wave, class, depends_on, blocked_by,
          contract_changed?, gates?, gated_by?, human_gates?: [{index, kind, note?, cleared}]}],
 waves: string[][], cycles: string[][], unordered: string[],
 externals: [{id, name, status, human_gates?}],
 ready_now: string[], max_parallel: number, parallel_set: string[]}
```

Only buildable nodes (leaves, or containers with `implementation` globs) are ordered; `wave`
is 1-based and nodes in one wave can be built in parallel. `blocked_by` lists unmet
dependencies, including spikes that gate the node (`gates` makes each listed node depend on the
spike). Nodes flagged `external` are left out of `order` and `waves`, are listed in
`externals`, and never block anything.

`class` is `blocked` (an unfinished dependency, or status `blocked`), else `needs-human`
(at least one human gate is not cleared), else `agent-parallel`. Cleared gates are per user in
the gitignored `.arch/local.json` (`gates_cleared`, `"<nodeID>#<index>"`); only a person sets
them (`PUT /gates` in the UI, no MCP tool). `ready_now` lists ordered nodes that are not
blocked, not done and not external (so it includes `needs-human` nodes; filter by `class` if
an agent must start alone). `parallel_set` is a greedy packing, in order, of `ready_now` into
nodes whose `implementation` globs do not overlap, using the same sample-path overlap test as
the `glob-overlap` lint; `max_parallel` is its size.

`compact: true` reduces each item to `{id, class, wave, done?}` and `externals` to ids (about a
tenth of the size on a large model); `ready_now`, `max_parallel`, `parallel_set`, `waves`,
`cycles` and `unordered` stay. `ready_only: true` keeps only the items in `ready_now` and
returns no `waves`. Optional `scope_node_id` limits it to a subtree (unknown id is an error).
The tool has no fixed output schema because the compact shape differs. Advisory only: it never
blocks `update_node_status`. Call it before building.

#### `get_graph(include_contracts?)`

Reads the whole model compactly and directly (no staging): `nodes` (id, name, type, parent,
relationship_id, status, `depends_on` and `has_contract` when set; no specs), `relationships` (id, from, to,
label, kind, `has_contract` when the edge has one) and
`pending_proposals` (id, source, summary, operation count; a `set_constraints` proposal has an operation with an
empty `node_id`). With `include_contracts: true`, each node and relationship that has one also
carries its full `contract` body. Also `method` (`{id, name}`) and `stage` (`{id, name}`, or `null` for a
stage-less method). Call it before authoring so ids
are reused and duplicates avoided.

#### `get_started()`

No arguments. Returns `{method: {id, name}, stage: {id, name} | null, guidance, stages:
[{id, name}], next_steps: [string]}` for the active workflow method (ADR 0013). `guidance`
is the method's text, the same string served as the MCP server `instructions`. `next_steps`
are short pointers derived from the method's data (stage gates, lint severities, stages
and `check.include_drift`), not from built-in method ids, so a user method gets advice
matching what it declares. Read-only. The server
`instructions` are fixed when the session starts (the SDK reads them at `initialize`), so
after a method change `get_started` is the live source.

#### `check_model(include_drift?, all_rules?, verbose_globs?)`

Lints the model against the active method's rules (same implementation as `ambit
check-model` and `GET /check-model`). Returns `{ok, method, stage?, findings[], counts:
{advisory, enforced, total}, drift?, drift_error?, method_warning?}`. `method_warning`
(string, omitted when none) is set when the method named in `config.json` did not load
(for example an invalid file in `.arch/methods/`) and the result was computed under
`freeform`, so its enforced rules were not checked. Each finding is `{rule, severity:
"advisory"|"enforced", node_id?, relationship_id?, message}`. `ok` is false only when an
enforced finding exists; it never blocks a mutation. `include_drift` defaults to the
method's `check.include_drift` when omitted; an explicit `false` turns drift off even when
the method enables it; `all_rules` runs every rule as advisory. In `drift`, `unmatched_globs`
is replaced by `unmatched_globs_count` unless `verbose_globs` is true (see `check_drift`).
Read-only.

#### `list_proposals()`

Returns the same proposal summaries as `GET /proposals` (id, source, summary, operations,
unreadable flag).

#### `list_icons()`

Returns `icons`: the closed icon registry as `{id, label}` rows, label being a one-line
description. Same list as `GET /icons`. Use these ids for the `icon` field.

#### `check_drift(node_id?, worktree?, verbose_globs?)`

Compares model globs with the files git can see: tracked files plus untracked files that
are not gitignored (same implementation as `ambit drift`). Returns
`unowned_files` (files no node's `implementation` matches; `.arch/` and files matching the
built-in or `drift_ignore` globs are excluded), `unmatched_globs_count` (number of
`implementation`/`scope` globs matching no file; the list itself, `unmatched_globs`
`{node, field, glob}`, is only returned when `verbose_globs` is true, because a design-stage
model has no files yet and the list is long), `overlaps` (`{path, owners}` for
files owned by two or more nodes of which at least two are not in an ancestor/descendant
relationship) and `ignored_files` (count skipped by ignore globs). `node_id` limits the
report to that node's globs and overlaps (unowned files are not reported). `worktree` reads
files from another checkout of the same repository (a linked git worktree), so an agent
can verify its own branch; the model still comes from the server's `.arch`. A path from a
different repository is refused. When more than 25 files are unowned the report carries a
`hint` (and `check_model` drift output the same) to add globs to `drift_ignore` in
`.arch/config.json`; `ambit config drift-ignore add <glob>` does that from the CLI. Read-only.

#### `update_node_status(node_id, status)`

Sets a node's `status` to one of `draft`, `specified`, `assigned`, `in_progress`,
`needs_review`, `done`, `blocked`.

**Writes directly, bypassing staging.** This is a deliberate exception. Routing it through
review would mean an agent finishing work leaves a proposal sitting unreviewed, so `status`
stops reflecting reality until the architect returns. The risk staging guards against is an
agent restructuring the model; a single enumerated field that cannot restructure anything
does not carry that risk, and it is trivially revertable.

**Stage gate.** Before writing, it asks the core `StageGate`. When the active method refuses
that status in the current stage, the call fails with a tool error carrying the gate
message (core `ErrStageGate`). Under `design-first` in the `design` stage, `assigned`,
`in_progress`, `needs_review` and `done` are refused; `draft`, `specified` and `blocked` are allowed.
`freeform` and `map-existing` never gate. The human UI and HTTP status control are not
gated.

**Completion policy** (decision 0015). Setting `done` or `needs_review` also asks the method's
`completion` policy (core `ErrCompletionGate`, HTTP code `completion_gate`). With
`review_required` an agent may not set `done`; it sets `needs_review` and a human sets `done`.
With `agent_may_complete: false` an agent may set neither. Whatever the policy, an agent may
not set `done` on a node that still has milestones not `done`, or whose `risk` is `high`: it
sets `needs_review` (this is what `check_done` reports as `finish_status`). A node's own `review` field
(`required` or `none`) overrides `review_required` for that node. Built-in methods leave the
policy open (agents may complete); a project turns it on with `method_overrides.completion`
in `config.json` or in a user method. The human UI and HTTP status control are not gated.

#### `check_done(node_id, worktree?, base?)`

One pass/fail answer to "am I done?" for a node. It takes the files changed on the branch
(committed, uncommitted and untracked, against `base`, default the merge-base with `main` or
`master`; `.arch/` is left out), checks them against the node's scope, runs drift limited to
the node, checks the node's declared `deliverables`, flags contracts that changed after the
node's status was last set, lists the node's `verify` commands and reports the status the
agent may set next (`completion`). It does not run the commands (see `run_verify`). `pass`
means no scope violation, no node drift and no missing deliverable. Read-only.

- Pass `base` (a git ref, for example the integration branch your work started from) so a
  branch that carries other nodes' commits is attributed correctly; the default merge-base
  with trunk would count their files as yours.
- A scope or implementation glob that matches no file yet is not reported as unmatched when a
  declared deliverable names it (the glob equals or matches the deliverable path); the
  missing deliverable is reported instead. Globs such as `docs/decisions/0003-*.md` match
  files present, even untracked, in the worktree.
- `deliverables` (when the node declares any) is the `check_deliverables` report; `milestones`
  lists the node's milestones.
- `completion.finish_status` is never plain `done` when the node is not fully finished:
  milestones not all `done` give `needs_review` with `completion.partial: true` and a
  `reason`; `risk: high` gives `needs_review` (`reason: risk is high`, the review gate itself
  is unchanged); otherwise `needs_review` when review is required and `done` when not, or empty
  when agents may not finish nodes.

#### `check_deliverables(node_id, worktree?)`

Direct, read-only. Reports, for each path in the node's `deliverables`, `ok`, `missing`,
`empty` (a zero-byte file, or a directory with no non-empty file) or `unsafe` (the path or a
symlink leaves the checkout; it is never read): `{node_id, pass, declared, items:[{path, status,
bytes}], problems}`. Pass `worktree` for a git worktree of the repository.

#### `run_verify(node_id, worktree?, timeout_s?)`

Direct. Runs the node's `verify` commands, taken only from the model on disk (never from a
pending proposal), one at a time with `sh -c` in the checkout (the worktree when given). Each
command has its own timeout (`timeout_s`, default 120, maximum 600). The environment is minimal:
a fixed allowlist (`PATH` with `node_modules/.bin` prepended, `HOME`, `USER`, `LANG`, `TMPDIR`,
Go and XDG cache variables, `CI`, ...) plus the variables the command itself names. Returns
`{node_id, dir, pass, results:[{command, status, exit_code, duration_ms, reason?, output?,
output_truncated?}], passed, failed, timed_out, not_runnable, warnings}`. `status` is `passed`,
`failed`, `timeout` or `not_runnable`: the command is not started when a binary it runs is not
found (a best-effort scan of the shell text, shell builtins excepted) or when a referenced
`$VAR` or `${VAR}` is unset (a default such as `${VAR:-x}` or an assignment in the command
counts as set). `output` is the last 8 KiB of stdout and stderr together. A timed-out command's
process group is killed. `pass` is true only when every command passed.

Security: these are shell commands from the model, with the user's file access, and nothing
is sandboxed beyond the environment and the timeout. Accept `verify` commands in a proposal
only if you would run them yourself. The tool is deliberately not annotated read-only: the
commands can have side effects, so clients must not auto-approve it.

#### `check_dependents(node_id, worktree?, timeout_s?)`

Direct. Runs `run_verify` for each direct dependent of the node (nodes whose `depends_on` names
it, and nodes it `gates`), to see whether finishing this node broke them. At most 16 are run
per call, within 15 minutes in total; dependents without verify commands, any beyond the cap,
and any not run before the time budget ran out (which also makes `pass` false) are listed in
`skipped`.
Returns `{node_id, pass, dependents:[{node_id, report}], skipped:[{node_id, reason}],
warnings}`. Same environment, timeout and trust rules as `run_verify`.

#### `check_contract_sync(node_id, worktree?)`

Direct, read-only. Compares a node with the docs it points at: every path in `references` and
the `ref` of every relationship contract touching the node must exist and be non-empty in the
checkout, and the contract must not have changed since the node was last marked `in_progress`
or `done`. Returns `{node_id, pass, has_contract, contract_changed, references:[{path, source,
exists, status}], problems}` where `source` is `references` or `contract`.

#### Subagent guidance

Give a subagent that implements a node the read-only tools: `get_context`, `check_scope`,
`check_done`, `check_drift`, `get_build_order`, `get_graph`, `list_proposals`, `get_proposal`,
`list_icons`, `get_started`, `check_model`, `check_deliverables` and `check_contract_sync`.
All of them carry the MCP `readOnlyHint` annotation. `run_verify` and `check_dependents`
execute shell commands, so they do not carry it (they are annotated `destructiveHint` and
`openWorldHint`), clients should not auto-approve them, and implementers do not get them: the
architect (parent) runs them. Keep `run_verify`, `check_dependents`, the staging tools, `update_node_status` and `wait_for_proposal`-driven
decisions in the parent agent, and have the parent write documentation deliverables itself
rather than delegating them. The brief ends with this rule.

#### `get_proposal(proposal_id)` and `wait_for_proposal(proposal_id, timeout_s?)`

`get_proposal` returns `{proposal_id, status, source, summary, operations[]}` where `status`
is `pending`, `accepted`, `rejected`, `partial` (some accepted, some rejected) or
`withdrawn` (deleted). Finished proposals are kept as records under
`.arch/.proposals/.resolved/` (the 100 most recently resolved), so the answer survives accept and reject.
`wait_for_proposal` blocks until the proposal is no longer pending or `timeout_s` passes
(default 60, at most 300) and returns the same state plus `state` (same as `status`) and
`timed_out`. On timeout it returns `{state: "pending", timed_out: true, hint: "still pending,
call again"}`; that is not an error, the architect has not decided yet. Staging tools point to
it, so an agent need not ask the architect to reply "accepted".

Operations within one `seed_model` call may reference each other in `parent_id` and
`depends_on` in any order; creates are ordered parent-first and dependency-first before
staging. A real cycle is still rejected.

## Invariants (Must Always Hold)

1. **No authoring tool modifies the canonical model.** They write only to
   `.arch/.proposals/`.
2. **Every staged tool's response states that the change is proposed, not applied.**
3. **Only `update_node_status` writes canonically.** Any future tool that writes directly
   requires an ADR.
3a. **Only `update_node_status` is gated, and the stage is human-only.** No MCP tool writes
   the project stage or the active method (no tool takes a `stage` or `method` argument);
   they change only through the UI, `ambit stage|method set` or `PUT /stage|/method`.
   Authoring tools, `check_scope`, `get_context`, `check_model` and `get_started` are never
   gated. The gate binds the MCP tool path only: `update_node_status` re-reads `config.json`
   and checks the gate and writes the status under one lock, so a stage change from another
   process is honoured at once, but an agent with a shell could still run `ambit stage set`
   or edit `config.json` directly. It is a guard rail for tool-driven agents, not a security
   boundary.
3b. **Clearing a human gate is human-only.** `gates_cleared` lives in the gitignored `local.json`
   and is set through `PUT /gates`; no MCP tool reads it for writing. `run_verify` and
   `check_dependents` execute the committed model's `verify` commands, so they are not annotated
   read-only (`destructiveHint` and `openWorldHint` instead); every other read tool carries the
   MCP `readOnlyHint`.
4. **All tools go through `ambit-core`.** No MCP-specific validation or write path exists.
5. **`get_context` names forbidden paths explicitly.** Omission is not prohibition.
6. **`check_scope` and `ambit check` share one implementation.** They cannot disagree about
   whether a file is in scope; if they can, the contract is broken.
7. **MCP never requires `ambit start`.** The two processes are independent.
8. **`id` is never settable.** Not on create, not on update.
9. **ambit makes no outbound model-provider calls.** No tool in this surface talks to an
   LLM. Generation is the harness's job entirely.

## Error Handling

- **Unknown `node_id`** — error naming the ID, listing nothing. Do not fuzzy-match to a
  similar ID; a wrong node silently accepted is worse than a failed call.
- **Invalid `status`** — error listing the six valid values.
- **Status refused by the stage gate** — error naming the status, stage and method, and
  saying a human must advance the stage.
- **`delete_node` on a node with children** — rejected at proposal time with the child count
  and names, so the agent can reparent or delete them first.
- **`create_node` or `update_node` producing a hierarchy cycle** — rejected at proposal time.
- **Relationship endpoint naming a missing node** — rejected at proposal time.
- **Attempt to set `id`** — rejected, with the reason: IDs are immutable, rename via `name`.
- **Model integrity warnings** (orphans, dangling references) — surfaced on the call that
  encountered them, not swallowed. An agent working against a broken model should be told.
- **Ignore entries ineffective** — warned loudly, consistent with `ambit start` and
  `ambit check`.

## Examples

### Minimal valid call and response

`check_scope`:

```json
{
    "node_id": "payments-service",
    "files": ["src/payments/authorise.ts", "src/orders/create.ts"]
}
```

```json
{
    "results": [
        { "path": "src/payments/authorise.ts", "allowed": true },
        {
            "path": "src/orders/create.ts",
            "allowed": false,
            "rule": "outside_scope",
            "node": "orders-service",
            "detail": "belongs to orders-service; not in the scope declared for payments-service"
        }
    ]
}
```

### Staged call, showing the required response framing

`create_node`:

```json
{
    "name": "Refunds Service",
    "type": "service",
    "parent_id": "platform",
    "implementation": ["src/refunds/**"],
    "spec": "# Refunds Service\n\nOwns refund issuance..."
}
```

```json
{
    "proposal_id": "p-20260927-1043-a91f",
    "applied": false,
    "message": "Proposed 1 node (Refunds Service) as proposal p-20260927-1043-a91f. This has NOT been applied to the model. The architect must accept it in the ambit UI before it takes effect."
}
```

### Invalid example, with expected handling

```json
{
    "node_id": "payments-servic",
    "status": "done"
}
```

Expected handling: error naming the unknown ID. No fuzzy match to `payments-service`, and
no write. Marking the wrong node done is a silent corruption of the architect's view of what
is finished.

### Operational notes

- `get_context` output is the highest-leverage prompt in the product. Changes to its framing
  should be recorded in [`docs/decisions/`](../decisions/) with what was observed, not made
  silently.
- `check_scope` is cheap and intended to be called repeatedly. Nothing rate-limits it.
- Proposals accumulate; nothing in v1 expires them. See
  [ADR-0002](../adr/0002-agent-interface-and-review-gate.md) follow-ups.

### References

- ADR: [`docs/adr/0002-agent-interface-and-review-gate.md`](../adr/0002-agent-interface-and-review-gate.md)
- Canonical spec: [`docs/planning/spec-v1.md`](../planning/spec-v1.md) Section 8
- Related contracts: [`proposals.md`](proposals.md),
  [`arch-model-format.md`](arch-model-format.md)
- Enforcement:
  [`docs/architecture/enforcement-model.md`](../architecture/enforcement-model.md)
