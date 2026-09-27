# Stage 02 — Enforcement

## Status

Done. 2026-09-28. See [`workflow.md`](../../workflow.md) for the merge back to `main`.

## What this stage proves

Given a model and a git diff, `ambit check` names files that fall outside the boundary the
architect drew. The same function will later answer `check_scope`, so the two cannot grow
apart.

## Build-plan steps

Step 3. Ordering: [`v1-build-plan.md`](../../v1-build-plan.md).

## Depends on

[Stage 01](../01-canonical-model/README.md). The check reads the index and `local.json`.
It does not need the HTTP API or the UI. Tests can write `.arch` fixtures directly.

## In scope

- `CheckScope` in `ambit-core`.
- `ambit check` against `git diff --name-only`.
- Opt-in `ambit hook install`.
- The runtime `git check-ignore` warning, as a function `ambit check` calls now and
  `ambit start` calls in stage 03.
- The enforcement unit-coverage release gate.

Semantics: [`enforcement-model.md`](../../../architecture/enforcement-model.md).

## Out of scope

- `check_scope` the MCP tool. The function lives here; the tool is
  [stage 05](../05-agent-interface/README.md). The equivalence gate is stage 05, once both
  callers exist. This stage's tests call `CheckScope` directly.
- `--strict`, a blocking exit code, CI, a policy language, per-agent roles.
- A second glob implementation. The inspector preview in stage 03 is a known duplicate and
  stays out of this package.

## Release gate

Task 02.8. Unit tests cover glob matching, `protected` overriding assignment, empty `scope`
defaulting to `implementation`, and both unmapped-file paths (violation when an assignment
is active, informational drift when it is not). A report that does not name the node and
the rule fails the gate.

## Watch-items

- **Silent fallback when no assignment is set.** Unmapped files become informational rather
  than violations. The report should say that no assignment is active, so the rule change
  is visible. Whether that copy is enough is still open; do not change the rules to paper
  over it. See [`open-questions.md`](../../open-questions.md).
- **`--strict`.** Do not add it in this stage. A user asking is the trigger, and changing
  the default to blocking needs an ADR.

## Links

- Tasks: [`tasks.md`](tasks.md)
- Tests: [`test-plan.md`](test-plan.md)
- Slice log: [`slice-log.md`](slice-log.md)
- Enforcement: [`enforcement-model.md`](../../../architecture/enforcement-model.md)
- Spec: [`spec-v1.md`](../../spec-v1.md) Section 10
