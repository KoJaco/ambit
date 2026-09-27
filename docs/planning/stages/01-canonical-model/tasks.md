# Stage 01 tasks

Working list. Status is mirrored in [`checklist.md`](../../checklist.md).

Do these in order. The index and the mutations assume the file format holds.

## 01.1 Read and write a node pair

- [ ]

**Touches:** `internal/core`.

**Done when:** loading `nodes/<id>.json` plus `nodes/<id>.md` returns the contract fields,
writing them back keeps the pair together, and an unknown JSON field survives a round-trip
untouched. `id` in the file matches the filename. Renaming changes `name` only.

## 01.2 Project files

- [ ]

**Touches:** `internal/core`.

**Done when:** `index.json` round-trips `schema_version`, the node membership list, and
relationships (`from`, `to`, optional `label` and `kind`). `config.json` loads as committed
non-secret config with no provider or key field. `local.json` loads and saves assignment
and is never required for a model to open.

## 01.3 In-memory index

- [ ]

**Touches:** `internal/core`.

**Done when:** startup builds `map[NodeID]*Node` from `nodes/*.json`, parent links come
from `parent_id`, and cross-cutting relationships live in a separate adjacency structure.
A relationship is not stored as a hierarchy edge. Hierarchy is not stored in
`index.json` as edges.

## 01.4 Mutations

- [ ]

**Touches:** `internal/core`.

**Done when:** create, update, delete, set relationship, and set scope and protected all
go through one validation path. A cycle is rejected. A `parent_id` or relationship endpoint
that names a missing node is rejected at mutation time. Deleting a node removes both files
and the membership entry. Empty `scope` is stored empty; callers apply the default to
`implementation` rather than the file being rewritten. An unknown `type` is accepted.
`protected` defaults to false.

## 01.5 Slug derivation

- [ ]

**Touches:** `internal/core`.

**Done when:** create derives `id` from `name` as `[a-z0-9]+(-[a-z0-9]+)*`. A collision
with an existing id gets a numeric suffix (`orders-service-2`). The caller cannot supply
`id`. A later rename does not change it.

## 01.6 Load-time integrity

- [ ]

**Touches:** `internal/core`.

**Done when:** a `.json` missing from `index.json`, or a `.md` without a `.json`, is an
integrity warning and is neither deleted nor adopted. A dangling `parent_id` or relationship
endpoint found on load warns, the node still loads, and the dangling edge is not traversed.
A cycle found in a hand-edited tree is an error and the offending edge is not traversed.
Malformed JSON names the file and refuses a partial load.

## 01.7 File watch

- [ ]

**Touches:** `internal/core`.

**Done when:** a change to a node file or `index.json` rebuilds the index without a process
restart. A rebuild uses the same loader as startup, including integrity warnings.

## 01.8 `ambit init`

- [ ]

**Touches:** `cmd/ambit`, `internal/core`.

**Done when:** `ambit init` creates `.arch/nodes/`, `index.json` at `schema_version` 1,
`config.json`, and the gitignored directories `.arch/.cache/` and `.arch/.proposals/`.
If `.gitignore` is missing it is created. Ignore entries for `local.json` (the
`.arch/local.json` path), `.arch/.cache/`, and `.arch/.proposals/` are appended only when
absent, so a second `init` does not duplicate them. `init` prints the suggestion to run
`ambit hook install` and does not install a hook.

## 01.9 Release gate

- [ ]

**Touches:** `internal/core` tests, or a test package that can exec the built CLI.

**Done when:** the integration test in [`test-plan.md`](test-plan.md) passes. This task is
the release gate. The stage is not done without it.
