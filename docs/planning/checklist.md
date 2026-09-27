# v1 Checklist

## Status

Live rollup. The working lists are the stage `tasks.md` files. When a box changes here,
change it there in the same edit. Slice logs record what a finished pass shipped; they do
not replace these boxes.

## Date

2026-09-27

## How to use this

Work the stages in order. A stage is done when every task in it is checked and, where the
stage has a release gate, that gate's test passes. Task 05.7 is the exception: brief
iteration continues after the other stage 05 tasks and does not hold the equivalence gate.
Gates are also listed on their own below so they can be read without scanning the whole list.

## Release gates

These five stop a stage being declared done early. Detail is in
[`v1-build-plan.md`](v1-build-plan.md).

- [x] **01.9** `.gitignore` integration test — real `ambit init`, real `git init`, real
  `git add -A`, and `local.json`, `.arch/.cache/`, `.arch/.proposals/` absent from
  `git status --porcelain`.
- [x] **02.8** Enforcement unit coverage — glob matching, `protected`, unmapped-file rules.
- [ ] **04.8** Proposal apply atomicity and staleness hash comparison.
- [ ] **05.6** `check_scope` and `ambit check` return the same verdict for the same inputs.
- [ ] **06.5** npx shim checksum verification. The shim does not ship without it.

## Stage 00 — Foundation

Done. [Stage](stages/00-foundation/README.md).

- [x] **00.1** Project root is the only git repository; `frontend/` is a subdirectory.
- [x] **00.2** Root `.gitignore` covers Node, Go, and the ambit working paths.
- [x] **00.3** Go module `github.com/KoJaco/ambit` with stub packages, and `make test` /
  `make build` succeed.

## Stage 01 — Canonical model

[Stage](stages/01-canonical-model/README.md).

- [x] **01.1** Read and write a node pair, preserving unknown fields.
- [x] **01.2** Read and write `index.json`, `config.json`, and gitignored `local.json`.
- [x] **01.3** Build the in-memory index: hierarchy by `parent_id`, relationships apart.
- [x] **01.4** Mutations: create, update, delete, set relationship, set scope and protected.
- [x] **01.5** Derive immutable slugs, with a numeric suffix on collision.
- [x] **01.6** Report orphans, dangling references, and hand-edited cycles on load.
- [x] **01.7** Rebuild the index when watched model files change.
- [x] **01.8** `ambit init` scaffolds `.arch/` and appends ignore entries idempotently.
- [x] **01.9** Release gate: the `.gitignore` integration test passes.

## Stage 02 — Enforcement

[Stage](stages/02-enforcement/README.md).

- [x] **02.1** `CheckScope` in `ambit-core`, one function for the CLI and later MCP.
- [x] **02.2** Map `git diff --name-only` onto nodes through `implementation` globs.
- [x] **02.3** Read the active assignment from `local.json`.
- [x] **02.4** Reports name the node and the rule hit. Exit code 0.
- [x] **02.5** `git check-ignore` warning, shared for `ambit check` and later `ambit start`.
- [x] **02.6** `ambit check` subcommand.
- [x] **02.7** `ambit hook install` refuses to clobber an existing `pre-commit` hook.
- [x] **02.8** Release gate: unit coverage for globs, `protected`, and unmapped files.

## Stage 03 — Architect canvas

[Stage](stages/03-architect-canvas/README.md).

- [x] **03.1** `ambit start` binds to localhost and refuses other interfaces.
- [x] **03.2** Drill-down query and endpoint return one level.
- [x] **03.3** Single-node read, including prose, for the inspector.
- [x] **03.4** Integrity warnings ride along with reads and have their own fetch.
- [x] **03.5** Direct graph mutations over HTTP, through `ambit-core`.
- [x] **03.6** Assignment write into `local.json`, and the contract line that names it.
- [x] **03.7** Layout cache storage in core, and get/put endpoints that never touch canonical files.
- [x] **03.8** SSE for model-changed and integrity-changed.
- [ ] **03.9** `ambit start` runs the ignore check before serving.
- [ ] **03.10** Delete the pipeline domain and the abandoned canvas skeleton.
- [ ] **03.11** Routes `/` and `/node/:nodeId`, on `@xyflow/react` v12.
- [ ] **03.12** Canvas renders one fetched level, with labelled relationships.
- [ ] **03.13** Inspector for ambit fields, including empty-scope and protected copy.
- [ ] **03.14** Sidebar becomes hierarchy navigation and node creation.
- [ ] **03.15** Typed API client and SSE client.
- [ ] **03.16** elkjs per level, recompute only on structural change, drag writes the cache.
- [ ] **03.17** Vitest on layout-cache keying and the inspector's scope preview.
- [ ] **03.18** Remove confirmed-dead template leftovers and replace the frontend README.

## Stage 04 — Review gate

[Stage](stages/04-review-gate/README.md).

- [ ] **04.1** Read and write a proposal directory: manifest plus materialised node files.
- [ ] **04.2** Stage a proposal from core, callable by tests before MCP exists.
- [ ] **04.3** Compute staleness at read time from `base_hash`.
- [ ] **04.4** Accept and reject per operation; accept is atomic; accept-all skips stale.
- [ ] **04.5** Remove a proposal once every operation is resolved.
- [ ] **04.6** Proposal HTTP endpoints, including the stale-confirm `409`.
- [ ] **04.7** SSE event `proposals-changed`.
- [ ] **04.8** Release gate: apply atomicity and staleness hash comparison.
- [ ] **04.9** Review UI: per-node accept and reject, stale state visually distinct.
- [ ] **04.10** Vitest on proposal diffing.

## Stage 05 — Agent interface

[Stage](stages/05-agent-interface/README.md).

- [ ] **05.1** `ambit mcp` over stdio, independent of `ambit start`.
- [ ] **05.2** Five authoring tools, each staging a proposal and saying it was not applied.
- [ ] **05.3** `get_context` returns an imperative brief with the required framing.
- [ ] **05.4** `check_scope` calls the stage 02 function against a caller-supplied file list.
- [ ] **05.5** `update_node_status` writes the status field directly.
- [ ] **05.6** Release gate: `check_scope` and `ambit check` agree.
- [ ] **05.7** Iterate the brief against Codex, Cursor, and Claude Code, and record what changed.

## Stage 06 — Distribution

[Stage](stages/06-distribution/README.md).

- [ ] **06.1** `react-router build` produces the client bundle the Go build embeds.
- [ ] **06.2** `ambit start` serves the embedded SPA.
- [ ] **06.3** Unmatched non-API paths fall back to the SPA entry document.
- [ ] **06.4** `npx ambit` detects the platform, downloads the binary, and caches it.
- [ ] **06.5** Release gate: the shim verifies a checksum before it runs a downloaded binary.

## Explicitly not v1

From [`spec-v1.md`](spec-v1.md) Section 12. These stay unchecked on purpose. Building one
requires an ADR or a spec change, not a quiet task.

- [ ] Codebase scanner
- [ ] Governance beyond `protected` and `scope`
- [ ] Human-escalation UI
- [ ] Multi-agent orchestration
- [ ] CI enforcement
- [ ] Cloud sync, accounts, multiplayer, billing
- [ ] Freeform natural-language-to-architecture generation
- [ ] An LLM client inside ambit
- [ ] A node type registry
- [ ] SQLite or any database
- [ ] Component and end-to-end frontend tests
- [ ] `--strict` / blocking `ambit check` by default

## References

- Ordering: [`v1-build-plan.md`](v1-build-plan.md)
- Stages: [`stages/`](stages/)
- Unresolved: [`open-questions.md`](open-questions.md)
