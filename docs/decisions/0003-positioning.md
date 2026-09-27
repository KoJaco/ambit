# Decision Note: Positioning — Dropping the Vertical Wedge

## Date

2026-09-27

## Summary

ambit's positioning rests entirely on posture — local-first, zero-account, offline by
default, with an enforcement check that runs locally and by default. The original spec's
secondary, vertical wedge (AWS-native node types for solo AWS solutions architects) is
dropped. ambit is domain-agnostic in v1 and makes no vertical claim.

## Decision

- **Primary wedge, unchanged:** posture, not features. Local-first, zero-account, offline,
  and a check that can actually stop an agent from committing outside a declared boundary —
  versus a hosted reference and sync layer. That posture implies a business model which is
  structurally awkward for a SaaS collaboration tool to adopt casually, which is what makes
  it defensible.
- **Secondary wedge: none.** The AWS vertical is gone and nothing replaces it.
- **Target user, unchanged:** solo architects and small consultancies, defined by working
  practice rather than by cloud provider.
- The transcript-in solutioning workflow and the agent handoff loop remain the product's
  shape, but they are described as how ambit works, not as a defensible claim.

## Rationale

- **The vertical wedge followed from the taxonomy, and the taxonomy is gone.** Once node
  types are free-form ([decision note 0002](0002-node-type-taxonomy.md)), there is nothing
  AWS-specific left in the product. Claiming a vertical ambit does not implement would be a
  claim the product cannot cash.
- **The AWS framing was never load-bearing.** Re-reading the original spec, the AWS
  specificity appears in exactly one place: the example list of node types. Nothing in the
  storage format, the enforcement model, the MCP surface, or the UI depended on it.
- **A vertical claim is a commitment.** Positioning as an AWS tool means AWS-shaped
  expectations — service catalogues, IAM semantics, account structures, Well-Architected
  alignment — none of which are in v1 or planned.
- **Posture alone is a sufficient wedge.** It is the harder thing to copy: a hosted
  collaboration product cannot casually ship a local-first, zero-account, offline mode
  without undermining its own model. Feature-level differentiation against a funded team is
  the losing game the original spec correctly warned against.
- **Domain-agnostic widens the evaluation pool** for a product whose most urgent need is
  real usage on real engagements.

## Impact

This is the weakest part of the current positioning and should be recorded as such rather
than dressed up.

- **The product is now harder to describe in one line to a specific person.** "Local-first
  architecture model with agent boundary enforcement" is accurate but abstract, where
  "AWS architecture tool for solo SAs" named its user. Losing that costs real marketing
  clarity.
- **Nothing narrows the competitive set.** Posture differentiates against IcePanel
  specifically; it does not differentiate against the next local-first modelling tool.
- **No domain-specific affordances** means no catalogue, no sensible type defaults, no
  domain-aware templates — every model starts from nothing.
- **Free-form types are the visible consequence.** A user gets no guidance about what to
  call things.

What does not change: the storage format, the enforcement model, the MCP surface, and the
UI are all unaffected. This is a positioning decision, not an architectural one.

## Follow-ups

- [ ] Revisit after real client engagements. If the same domain keeps recurring in practice,
      a vertical wedge can be reintroduced on evidence rather than assumption — and with a
      node type registry to back it, which is the same revisit as
      [decision note 0002](0002-node-type-taxonomy.md).
- [ ] Write a one-line positioning statement that names a user, not just a posture. The
      current framing describes what ambit is rather than who it is for, and that gap is the
      substance of this decision's cost.
- [ ] Track whether the transcript-in workflow turns out to be the thing users actually
      value. If so, a workflow-shaped wedge is available and does not require a domain
      commitment.

## References

- Canonical spec: [`docs/planning/spec-v1.md`](../planning/spec-v1.md), Positioning
- Superseded framing: [`docs/archive/spec-v0-original.md`](../archive/spec-v0-original.md),
  Positioning
- Related decision: [`0002-node-type-taxonomy.md`](0002-node-type-taxonomy.md)
- Open questions: [`docs/planning/open-questions.md`](../planning/open-questions.md)
