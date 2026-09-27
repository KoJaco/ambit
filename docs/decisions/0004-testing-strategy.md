# Decision Note: Testing Strategy

## Date

2026-09-27

## Summary

Go carries the testing weight: unit tests across `ambit-core`, and one integration test
promoted to a release gate. The frontend gets Vitest on pure logic only — no component
tests and no end-to-end tests in v1.

## Decision

### Go

- **Unit tests across `ambit-core`**, concentrated on the parts where a bug is a
  correctness or safety problem rather than a cosmetic one: glob matching and scope
  evaluation, `protected` handling, the unmapped-file rules, hierarchy validation and cycle
  prevention, ID derivation and collision suffixing, proposal apply atomicity, and
  staleness hash comparison.
- **One release-gate integration test**, described below.
- **Shared-implementation assertion:** a test proving `ambit check` and `check_scope` return
  the same verdict for the same inputs. The contract says they cannot disagree; that should
  be enforced by a test, not by intent.

### The `.gitignore` release gate

The gating test runs the full `ambit init` flow in a temp directory, actually runs
`git init`, actually runs `git add -A`, and asserts that `local.json`, `.arch/.cache/`, and
`.arch/.proposals/` do **not** appear in `git status --porcelain`.

**Build order step 1 is incomplete until this passes.** It is a gate, not a test in the
suite's general population.

### Frontend

- **Vitest on pure logic only:** proposal diffing, scope glob matching in the inspector,
  layout cache keying.
- **No component tests, no Playwright, in v1.**

## Rationale

- **The gate tests the failure mode, not the artifact.** Asserting on `.gitignore`'s
  contents proves a string was written; it does not prove the file stays out of a commit.
  The original spec made this point about a leaked API key. There is no key any more, but
  the reasoning survives intact for `.proposals/`, which would mean unreviewed
  agent-authored content entering git disguised as the model — the exact failure the review
  gate exists to prevent. A content assertion would pass while `init` ran before `git init`,
  or while a pre-existing `.gitignore` merged oddly.
- **Enforcement logic is where bugs are expensive.** A wrong glob verdict either lets an
  agent out of its boundary or blocks legitimate work. Both erode trust in the mechanism the
  product is built on.
- **The UI is the least-settled part of v1.** Component and E2E tests written against a
  moving target are maintenance cost with little signal, and the drill-down and proposal
  review flows are exactly the parts most likely to change shape during the build.
- **Pure frontend logic is cheap to pin and has real stakes.** Scope glob matching in the
  inspector must agree with the Go implementation, and proposal diffing determines what the
  architect sees before accepting. Both are pure functions.
- **The `check_scope` / `ambit check` equivalence is a contract claim.** Claims that two
  code paths agree decay unless tested.

## Impact

- The drill-down and proposal review flows have no automated coverage in v1. Regressions
  there will be found by hand, which is accepted while the UI is moving.
- Scope glob matching is implemented twice — Go for enforcement, TypeScript for the
  inspector's preview — and the two are tested separately rather than against a shared
  fixture set. This is a real divergence risk and is the strongest candidate for a shared
  test corpus later.
- The release gate adds a real `git` dependency to the test suite, so CI (if added) needs
  git available and configured with a committer identity.

## Follow-ups

- [ ] Add Playwright coverage for drill-down and proposal review once the proposal review
      flow has stabilised.
- [ ] Consider a shared fixture file of glob-matching cases, consumed by both the Go and
      TypeScript test suites, to stop the two implementations drifting.
- [ ] Decide whether the release gate runs in CI or stays a local pre-release step; CI is
      out of scope for the product, but the project's own test suite is a different question.

## References

- Canonical spec: [`docs/planning/spec-v1.md`](../planning/spec-v1.md) Section 2
- Build plan: [`docs/planning/v1-build-plan.md`](../planning/v1-build-plan.md)
- Related ADR: [`docs/adr/0004-frontend-platform.md`](../adr/0004-frontend-platform.md)
- Enforcement:
  [`docs/architecture/enforcement-model.md`](../architecture/enforcement-model.md)
