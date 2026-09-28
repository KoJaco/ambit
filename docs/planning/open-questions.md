# Open Questions

## Status

Live. Updated as questions are resolved or new ones surface.

## Date

2026-09-27

## Purpose

Things deliberately left unresolved for v1, and known thin spots in the design. Recorded so
they are decided on evidence later rather than rediscovered as surprises — and so that
"we knew about this" is distinguishable from "we missed this".

Items here are **not** the same as the deferred scope in
[`spec-v1.md`](spec-v1.md) Section 12. That list is settled: those things are out. These are
genuinely undecided.

---

## Known thin spots

### Concurrent writes from two processes

`ambit start` and `ambit mcp` are independent processes, each with its own in-memory index,
both writing `.arch`. **There is no cross-process lock in v1.**

The current position is that writes are small, scoped to distinct files, and infrequent
enough that collisions are unlikely. That is a bet, not a guarantee. The realistic failure is
a `seed_model` proposal being written while the architect saves a node edit.

Candidates if it bites: a lock file, or routing all writes through whichever process holds a
lease. **Trigger for revisit:** any observed corruption or lost write.

Recorded in [ADR-0003](../adr/0003-runtime-and-distribution.md).

### Scope glob matching is implemented twice

Go owns enforcement; TypeScript owns the inspector's preview of what a scope covers. Two
implementations of the same matching rules will drift, and a drift here means the UI tells
the architect something the check disagrees with.

Candidate: a shared fixture file of matching cases consumed by both test suites.

**Trigger for revisit:** the first observed disagreement, or before the inspector's preview
becomes something users rely on.

### Layout cannot be computed headlessly

elkjs is a JavaScript library running in the browser. The Go side manages the cache but
cannot populate it, so there is no way to compute layout from the CLI.

No v1 need. It would matter for a future static export or screenshot command.

---

## Product questions

### Positioning names a posture, not a user

The current one-liner describes what ambit is rather than who it is for. Dropping the AWS
vertical removed the part that named a person, and nothing replaced it — see
[decision note 0003](../decisions/0003-positioning.md).

This is the weakest part of the current framing, and it is a marketing problem rather than
an architectural one.

**Trigger for revisit:** before any public launch, and after the first real client
engagement.

### Does the transcript-in workflow turn out to be the valued part?

If real use shows that transcript-seeded solutioning is what users actually want, a
workflow-shaped wedge becomes available that does not require a domain commitment. Untested
assumption today.

### Should node types get a registry?

Free-form in v1 on the grounds that the recurring types are not yet knowable
([decision note 0002](../decisions/0002-node-type-taxonomy.md)). The cost is no
autocomplete and no consistency: one model can hold `database`, `datastore`, and
`data store` as three distinct types.

**Trigger for revisit:** several real client models existing, with the recurring type strings
observable rather than imagined. Track what people actually type in the meantime; that data
is the input.

### How should drillable hierarchy be indicated on the canvas?

Relationship interiors already expose `drillable` on crossings. **Child drill** (`/node/:id`)
has no equivalent affordance on `NodeCard` — architects cannot see which nodes have
children without opening each one.

Candidate: `has_children` or `child_count` on level summaries + a minimal badge (possibly
sharing the stage 07 icon registry). See
[decision note 0007](../decisions/0007-drill-affordance-and-hierarchy-compression.md).

**Trigger for revisit:** stage 07 slicing or the first canvas ergonomics pass after
dogfooding the meta-model.

### How should “lift” and “embed” hierarchy compression work?

Users want to **embed** a small subsystem inside a node (collapse siblings under a container)
and the **inverse lift** (promote children to the parent level) to cut visual clutter and
drill depth without abandoning canonical structure.

Likely implemented as **batched reparent / create_node operations** through the existing
proposal gate, not a shadow “collapsed” view — unless virtual collapse proves necessary.

Open: multi-select on canvas, relationship endpoints when members move, scope/implementation
on new container nodes. See
[decision note 0007](../decisions/0007-drill-affordance-and-hierarchy-compression.md).

**Trigger for revisit:** after drill affordance ships and we have a deep model to stress-test
(e.g. the ambit self-model).

---

## Workflow questions

### Do abandoned proposals need garbage collection?

Nothing in v1 expires a proposal with operations still `pending`. Proposal IDs are
time-ordered, so age is visible at a glance, but nothing acts on it.

Candidate: an `ambit proposals prune` command, or an age threshold in the review UI.

**Trigger for revisit:** evidence about how many proposals get abandoned in practice. Likely
to be workflow-dependent — an architect who seeds a model and walks away will produce far
more litter than one who reviews immediately.

### How well does `get_context` framing actually hold?

The brief's design is grounded in reasoning, not measurement: enumerate allowed globs,
explicitly name forbidden paths rather than omitting them, state the stop-and-report escape
hatch, instruct periodic `check_scope` calls.

**What is genuinely unknown is the violation rate across different harnesses and models.**
The layered enforcement argument holds regardless — the backstop stays mandatory either way —
but the framing's effectiveness determines how often the backstop fires and therefore how
much the architect has to do by hand.

**Trigger for revisit:** continuously during [stage 06](stages/06-agent-interface/README.md),
and after real client work. Record what was learned when the wording changes, in
[`docs/decisions/`](../decisions/).

### Is per-node review granularity right at `seed_model` scale?

Per-node was chosen over atomic because a thirty-node proposal under an all-or-nothing gate
trains the user to click accept without reading. But thirty per-node decisions in a row may
produce the same fatigue by a different route.

**Trigger for revisit:** first real transcript seeding. Watch whether accept-all gets used
reflexively; if it does, the gate is decorative and the granularity is not the fix.

### Does `--strict` need to exist?

`ambit check` warns and exits 0 by default. A `--strict` flag for a genuinely blocking
pre-commit hook is anticipated but deliberately not built.

**Trigger for revisit:** a user asking for it. Not before — and changing the *default* to
blocking requires an ADR, not just a flag.

### Is assignment-as-local-state right for more than one machine?

Assignment lives in the gitignored `local.json`, which is correct for a solo architect on one
machine. An architect moving between a laptop and a desktop mid-engagement loses the
assignment, and `ambit check` silently falls back to the no-assignment path — where unmapped
files become informational rather than violations.

**That silent fallback is the sharper edge**, not the lost state. Worth considering whether
`check` should say "no active assignment" prominently rather than quietly changing its rules.

---

## Format questions

### Do the reserved fields survive contact with reality?

`observed_implementation` and `agent_authority` are documented as optional so the format need
not change shape later. No v1 code reads or writes them.

**Risk:** they encode guesses about v2 that may be wrong, and a wrong reserved field is worse
than none — it looks like a commitment. **Trigger for revisit:** when either feature is
actually designed.

### Is the two-file-per-node split worth its bookkeeping?

`.json` plus `.md` per node means create, delete, and rename must keep the pair consistent,
and an orphaned `.md` is a state that has to be handled.

The alternative — markdown with YAML frontmatter — was rejected because two machine writers
must round-trip the structured half without disturbing the prose, and YAML round-tripping
reorders keys and mangles formatting. That reasoning stands, but the bookkeeping cost is
real and has not yet been paid in practice.

**Trigger for revisit:** if orphan handling becomes a recurring source of bugs.

---

## Resolved

Nothing yet. As questions are answered, move them here with the answer and the date, rather
than deleting them — a question that was live and got settled is more useful to a future
reader than a question that silently vanished.

## References

- Canonical spec: [`spec-v1.md`](spec-v1.md)
- Build plan: [`v1-build-plan.md`](v1-build-plan.md)
- Checklist: [`checklist.md`](checklist.md)
- Stages: [`stages/`](stages/)
- ADRs: [`docs/adr/`](../adr/)
- Decision notes: [`docs/decisions/`](../decisions/)
