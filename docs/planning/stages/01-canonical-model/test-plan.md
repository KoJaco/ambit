# Stage 01 test plan

Go tests in `internal/core`, plus one integration test that shells out to `git`. Decision:
[0004](../../../decisions/0004-testing-strategy.md).

## Unit

- Round-trip of a fully populated node and of a minimal node (`id`, `name`, `type`,
  `status` only).
- Unknown JSON field preserved and not interpreted.
- Create writes `.json` and `.md` together. Delete removes both and drops the id from
  `index.json`.
- Rename changes `name` and leaves `id` and the filename.
- Slug rules, including a collision suffix and a second collision (`-3`).
- Cycle rejected on `parent_id` update. Self-parent rejected.
- Relationship to a missing node rejected. Duplicate directed edge updated in place when
  set again; the opposite direction is a different edge.
- Empty `scope` stored as empty, not rewritten to a copy of `implementation`.
- Unknown `type` accepted, including strings with spaces.
- Orphan `.json`, orphan `.md`, dangling `parent_id`, and a hand-edited cycle each produce
  the contract's warning or error and do not mutate the files to "fix" them.
- Malformed JSON fails the load and names the file.
- `init` against an existing `.gitignore` that already contains one of the three entries
  appends only the missing ones.

## Release gate — task 01.9

Not a unit test and not a content assertion on `.gitignore`.

1. Temp directory.
2. Run `ambit init`.
3. Run `git init` and configure a temporary committer identity inside that directory only.
4. Create files under `.arch/local.json`, `.arch/.cache/`, and `.arch/.proposals/`.
5. Run `git add -A`.
6. Assert those three paths do not appear in `git status --porcelain`.
7. Assert a file under `.arch/nodes/` would appear (the canonical tree is not ignored).

Also run the same assertion when `git init` happens before `ambit init`, and when
`.gitignore` already exists with unrelated entries. Those are the merges the content
assertion would miss.

The test needs `git` on `PATH`. It must not touch the developer's global git config.

## Manual

None required to close the stage. `ambit init` in a scratch repo is worth doing once while
reading the resulting tree, and it does not replace the gate.

## Not in this stage

- Glob matching and `protected`. Stage 02.
- Frontend tests.
- Proposal apply.
