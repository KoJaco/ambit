# Stage 02 test plan

Go unit tests against `CheckScope` and the report builder. Git is stubbed or invoked in a
temp repo only for the diff-collection and hook tests. Decision:
[0004](../../../decisions/0004-testing-strategy.md).

## Release gate — task 02.8

These cases are the gate. Each expected violation names a node and a rule.

- A file matching the assigned node's `implementation`, with empty `scope`, is allowed.
- A file outside that default, while assignment is active, is `outside_scope` (or the
  contract's rule id — pick one string in the report and use it everywhere, including the
  later MCP result).
- `scope` set narrower than `implementation` forbids the gap.
- `scope` set wider than `implementation` allows the extra glob.
- A file matching a `protected` node's `implementation` is `protected` even when the
  assignment is that same node.
- A file matching two nodes is reported against each match that fails a rule.
- An unmapped file with an active assignment, outside that scope, is a violation.
- An unmapped file with no assignment is informational drift, and the report text says no
  assignment is active.
- Exit code of the check command is 0 when the report contains violations.

## Other unit

- `git check-ignore` warning when one of the three paths is not ignored, and silence when
  all three are.
- `ambit hook install` in a temp repo with no hook writes a hook. A second run, and a run
  against a repo that already has a `pre-commit` file, leave that file unchanged.

## Manual

Run `ambit check` once in a scratch repo with a two-node fixture and a dirty file outside
the assigned globs, and read the report. This does not replace the gate.

## Not in this stage

- Equivalence with the MCP tool. Stage 06 task 06.6 calls the same function through both
  front doors.
- A shared fixture file with the TypeScript inspector. That is an open question, triggered
  when the inspector preview starts to matter. Stage 03 tests its preview separately.
- `--strict`.
