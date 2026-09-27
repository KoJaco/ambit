# Stage 02 tasks

Working list. Status is mirrored in [`checklist.md`](../../checklist.md).

## 02.1 `CheckScope`

- [ ]

**Touches:** `internal/core`.

**Done when:** one function takes a node id (the assignment), a file list, and the index,
and returns a per-file verdict. Empty or absent `scope` is evaluated as `implementation`.
`protected` on the node that owns a file rejects the file even when that node is the
assignment. A file matching more than one node's `implementation` considers every match.
The function does not shell out to git. Stage 05's MCP tool and this stage's CLI both call
it.

## 02.2 Diff mapping

- [ ]

**Touches:** `internal/core`, `cmd/ambit`.

**Done when:** `ambit check` collects paths with `git diff --name-only` and passes that
list to `CheckScope`. The diff command is the only git use on this path besides the ignore
check in 02.5.

## 02.3 Assignment

- [ ]

**Touches:** `internal/core`.

**Done when:** the check reads `assignment` from `local.json`. Absent file or absent field
means no assignment, which selects the informational-drift rule for unmapped files. The
check never writes `local.json` and never reads assignment from a committed node field.

## 02.4 Report

- [ ]

**Touches:** `internal/core`, `cmd/ambit`.

**Done when:** every violation names the path, the node, and the rule (`protected` or
outside the assigned scope). Unmapped files with an active assignment are violations.
Unmapped files with no assignment are informational and the report says that no assignment
is active. The process exits 0 in every v1 outcome, including when violations exist.

## 02.5 Ignore check

- [ ]

**Touches:** `internal/core`.

**Done when:** a shared function runs `git check-ignore` for `local.json` (under `.arch/`),
`.arch/.cache/`, and `.arch/.proposals/`, and returns a loud warning when any path is not
ignored. `ambit check` calls it before reporting violations. The function is callable from
`ambit start` without copying it. A warning does not change the exit code.

## 02.6 `ambit check` subcommand

- [ ]

**Touches:** `cmd/ambit`.

**Done when:** `ambit check` from a repo with a `.arch/` loads the model, runs 02.5, runs
02.2, and prints 02.4. Missing `.arch/` is an error that names what is missing.

## 02.7 Hook install

- [ ]

**Touches:** `cmd/ambit`.

**Done when:** `ambit hook install` writes a `pre-commit` hook that runs `ambit check`
only if that hook file does not already exist. An existing hook is left byte-for-byte
unchanged and the command tells the user it refused. `ambit init` already prints the
suggestion (stage 01); this task does not move install into `init`.

## 02.8 Release gate

- [ ]

**Touches:** `internal/core` tests.

**Done when:** the cases in [`test-plan.md`](test-plan.md) under "Release gate" pass. The
stage is not done without them.
