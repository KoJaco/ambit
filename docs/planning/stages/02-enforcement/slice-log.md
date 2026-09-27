# Slice log — Enforcement

Completed implementation passes land here. One section per pass. Status stays in
[`tasks.md`](tasks.md) and the [checklist](../../checklist.md).

## 2026-09-28 — Scope check and pre-commit hook

- **Landed:** `CheckScope` judges a file list against `implementation` globs, effective
  scope, and `protected`. Empty `scope` is evaluated as `implementation`. `ambit check`
  reads the assignment from `local.json`, collects paths with
  `git diff --name-only --no-renames HEAD`, warns when `.arch/local.json`,
  `.arch/.cache/`, or `.arch/.proposals/` is not ignored, and prints violations and
  informational drift. That diff command is the only one used, for both the manual
  command and the hook. Exit code is 0 when the model loads, including when violations
  exist or the ignore warning fires. A missing `.arch/` is an error. `ambit hook install`
  writes a `pre-commit` hook that runs this binary's `check` command and refuses to
  overwrite an existing hook. `ambit init` still only prints the suggestion.
- **Deferred:** the MCP `check_scope` tool, `--strict`, and `ambit start`. The ignore
  warning is a function `ambit start` can call later.
- **Verified:** `go test ./...` passes, including the release-gate cases: empty scope,
  narrower and wider scope, `protected` overriding assignment, two failing matches, both
  unmapped-file paths, and exit code 0 when the report contains violations. A scratch repo
  with two nodes and a dirty file outside the assigned globs reported
  `src/orders/create.ts outside_scope orders-service` and exited 0.
