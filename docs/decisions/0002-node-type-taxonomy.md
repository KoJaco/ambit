# Decision Note: Node Type Taxonomy

## Date

2026-09-27

## Summary

A node's `type` is a free-form string with no enum, no registry, and no validation. The
original spec's AWS-native taxonomy (VPC, service, data store, IAM boundary, account
boundary) is dropped entirely — it was an example of what a type might be, not a commitment
to AWS, and ambit is domain-agnostic.

## Decision

- `type` accepts any string.
- No shipped registry of known types in v1.
- The UI matches recognised strings to icons on a best-effort basis and falls back to a
  generic shape for anything else.
- **An unrecognised type is never an error and never a warning.** It is the normal case.

## Rationale

- **The taxonomy is not knowable yet.** No real client model has been built with ambit. A
  registry written now is a guess about which types recur, and users would spend their time
  working around a wrong guess rather than modelling.
- **ambit is not an AWS tool.** The original spec's AWS framing carried an entire
  positioning claim with it. Dropping the taxonomy is a consequence of dropping that claim,
  not an independent simplification — see
  [decision note 0003](0003-positioning.md).
- **Types are a rendering hint, not a semantic.** Nothing in `ambit-core` branches on
  `type`. Hierarchy comes from `parent_id`, enforcement comes from `implementation`,
  `scope`, and `protected`. A field that only affects which icon is drawn does not warrant
  validation machinery.
- **A closed enum would block modelling.** An architect describing something ambit's authors
  did not anticipate would be stuck, and the workaround — picking the nearest wrong type —
  produces a model that lies.
- **Free-form is trivially upgradeable.** Adding a registry later is additive: existing
  free-form values keep working, and unrecognised ones keep falling back. Removing an enum
  later is not so easy.

## Impact

- No autocomplete and no consistency enforcement. A single model can end up with
  `database`, `datastore`, and `data store` as three distinct types that render three
  different ways. This is a real cost and is accepted deliberately.
- The UI needs an icon-matching layer with a genuine generic fallback, and the fallback has
  to look intentional rather than broken — it will be hit constantly.
- The node inspector shows a free text input, not a select.
- Schema validation of node files does not check `type` at all.

## Follow-ups

- [ ] Revisit once several real client models exist and the recurring types are observable
      rather than imagined. A shipped registry with warnings for unknowns — the middle
      option rejected for v1 — becomes reasonable at that point.
- [ ] Keep a note of which type strings actually recur during real engagements; that data is
      the input to the revisit.

## References

- Related ADR: [`docs/adr/0001-canonical-model-storage.md`](../adr/0001-canonical-model-storage.md)
- Contract: [`docs/contracts/arch-model-format.md`](../contracts/arch-model-format.md)
- Related decision: [`0003-positioning.md`](0003-positioning.md)
