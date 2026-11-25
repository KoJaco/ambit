# Decision Note: Workflow methods (stage, model lint, method text hooks)

## Date

2026-11-24

## Summary

Ambit core stays neutral. It offers three mechanisms: a project `stage`, a model lint engine, and text hooks (MCP server instructions, a `get_started` tool, and a `get_context` preamble). A selected **method** supplies the policy for all three. A method is a data bundle (guidance text, stages, gated transitions, lint severities, brief preamble). Built-ins are embedded in the binary; users can add their own under `.arch/methods/`. Three built-ins ship: `freeform` (default, today's behaviour, nothing enforced), `design-first`, and `map-existing`. The only hard gate in v1: under `design-first` with `stage = design`, the MCP `update_node_status` tool refuses `assigned`, `in_progress` and `done`. Everything else is advisory. Existing projects stay on `freeform`.

## Context

-   Decision 0012 let ambit drive a greenfield build, but nothing says when to design and when to build. Agents start coding against half-modelled graphs.
-   Modelling quality rules are prose in tool descriptions (0008, 0010). Nothing checks them. `check_drift` only covers code versus model.
-   Teams want different workflows (model first, or map an existing repo). Hard-coding one in core breaks neutrality and portability.
-   Harness-specific files (skills, rules) drift from the model. Method text must be served over MCP, which every harness speaks.

## Options

1.  Hard-code design-first in core. Simple, but forces one workflow on everyone.
2.  Prose-only skills per harness. No enforcement, drifts per harness, violates portability. Rejected.
3.  **Neutral mechanisms plus a method data bundle (chosen).**
4.  Method as an executable plugin or script. Too much surface and a security risk. Rejected.

## Decision

1.  **Stage.** A project-level string. The method defines the allowed stage ids and order. Only a human advances it (UI, `ambit stage set <id>`, `PUT /stage`). No MCP tool or agent path writes it. Stored in committed `config.json` so every clone and agent agrees, with changes visible in git.
2.  **Lint engine.** `core.CheckModel(idx, method)` runs a fixed rule registry. The method maps rule id to `off`, `advisory` or `enforced`. Findings are `{rule, severity, node_id?, relationship_id?, message}`. "Enforced" only means `ambit check-model --strict` exits non-zero and `check_model` reports `ok: false`. It never blocks a mutation or a stage change (the UI warns).
3.  **Text hooks.** Method `guidance` is the MCP server `instructions`, is returned by the `get_started` tool, and a method/stage preamble is prepended to every `get_context` brief. `get_graph` returns `method` and `stage`.
4.  **Method = files.** JSON, with long text allowed as `{"file": "x.md"}` beside it. Built-ins are embedded; user methods live in `.arch/methods/<id>.json`. A user id equal to a built-in id shadows it with a warning. `extends` (built-in id only) inherits missing keys.
5.  **Selection.** `config.json` key `method` (absent = `freeform`). `ambit init` prompts on a TTY, accepts `--method <id>`, and defaults to `freeform` non-interactively. `ambit method list|show|set` and a UI setting change it. Switching changes policy only, never model data. If the current stage is invalid in the new method, it resets to the new method's initial stage and says so.
6.  **One hard gate, agents only.** `gates.status` maps a stage to refused statuses. In v1 only `design-first` declares one: while `stage = design`, MCP `update_node_status` rejects `assigned`, `in_progress` and `done` with `ErrStageGate`. `draft`, `specified` and `blocked` stay allowed. Authoring tools, `check_scope` and `get_context` are never gated. The human UI status control is not gated. The gate binds the MCP tool path only: an agent with a shell could run `ambit stage set` or edit `config.json`. It is a guard rail, not a security boundary. `update_node_status` re-reads `config.json` and writes the status under one lock so a stage change made by another process takes effect immediately.
7.  **Portability.** `ambit method export-skill [--target claude|cursor]` writes the active method as a skill/rule file marked "generated, MCP is the source of truth". Core has no harness-specific logic.

## Method file format

```json
{
  "schema_version": 1,
  "id": "design-first",
  "name": "Design first",
  "description": "...",
  "guidance": "text or {\"file\":\"x.guidance.md\"}",
  "brief_preamble": "text or {file}",
  "stages": [{"id": "design", "name": "Design", "preamble": "..."}, {"id": "build", "name": "Build", "preamble": "..."}],
  "initial_stage": "design",
  "gates": {"status": {"design": ["assigned", "in_progress", "done"]}},
  "lint": {"rule-id": "off|advisory|enforced"},
  "check": {"include_drift": false},
  "extends": "freeform"
}
```

-   `stages` absent means a stage-less method; `PUT /stage` returns 409 `stage_unsupported`.
-   Limits: `guidance` <= 6000 chars; each `preamble` <= 1500; <= 8 stages; ids are slugs. Unknown lint rule ids load with a warning. An invalid user method file is skipped with a warning and the project falls back to `freeform` (never a load error).
-   A `lint` value may also be `{"severity": "advisory", "params": {"max_depth": 3}}`. Unlisted rules default to `off`.

## Initial lint rules

`leaf-implementation`, `leaf-contract`, `spec-present`, `icon-set`, `edge-label`, `edge-kind`, `max-depth` (param `max_depth`, default 3), `glob-overlap`, `orphan-node`, `dependency-cycle`. `note` and `external` node types are exempt from the leaf rules and from `orphan-node`. Method files and `{file}` texts must be regular files of at most 64 KiB inside `.arch/methods/` (symlinks and special files are rejected and the method is skipped with a warning).

## Built-in methods

-   **freeform** (default): one-paragraph guidance, no stages, no gates, all lint off. Output is byte-identical to today's for existing projects.
-   **design-first**: stages `design` (initial) and `build`; gate refuses `assigned`/`in_progress`/`done` in `design`; lint: `leaf-implementation`, `spec-present`, `glob-overlap`, `dependency-cycle` enforced; `leaf-contract`, `icon-set`, `edge-label`, `edge-kind`, `max-depth`, `orphan-node` advisory.
-   **map-existing**: no stages or gates; guidance is read the layout first, set globs from real paths, run `check_drift`, change no application code, keep models shallow; `check.include_drift` true; lint: `leaf-implementation`, `glob-overlap`, `dependency-cycle` enforced; `max-depth`, `orphan-node`, `edge-label`, `spec-present` advisory.

## Surface changes

-   **MCP** 15 -> 17 tools: `get_started` (`{method, stage, guidance, stages, next_steps}`) and `check_model` (`{include_drift?, all_rules?}` returns `{ok, method, stage, findings[], counts, drift?}`). `update_node_status` can return `stage_gate`. `get_graph` returns `method` and `stage`. `get_context` has a preamble. No tool sets stage or method.
-   **HTTP:** `GET /method`, `GET /methods`, `PUT /method`, `GET /stage`, `PUT /stage`, `GET /check-model`. Error bodies gain an additive `code` field.
-   **CLI:** `ambit init [--method id]`, `ambit method list|show|set|export-skill`, `ambit stage [show|set <id>]`, `ambit check-model [--strict] [--drift|--no-drift] [--all]`. CLI usage errors exit 1 (as elsewhere in `ambit`). `--strict` also exits 1 when the configured method did not load. `ambit init` prompts only for a fresh project (no `method` key and `.arch` newly created or empty); `--method` validates the id before scaffolding and reports a change on an existing project. `export-skill` refuses to overwrite a file it did not generate unless `--force`.
-   **Errors:** `ErrStageGate`, `ErrUnknownMethod`, `ErrBadMethod`, `ErrUnknownStage`, `ErrStageUnsupported`, `ErrBadLintRule` (core only; surfaced over HTTP as `bad_method` because a bad lint value makes the method file invalid).
-   **Compatibility:** no `method` key = `freeform`; `schema_version` stays 1; old binaries keep new keys as unknown config. A project naming a missing user method falls back to `freeform` with a warning.

## Rationale

-   Policy as data keeps core neutral and makes methods shareable by committing `.arch/methods/`.
-   Server-served guidance reaches every harness; exports are optional copies.
-   A single, human-owned stage and one agent-only gate keeps the safety story simple: agents cannot move the line they are bound by.
-   Linting is a pure function of the model, shared by CLI, HTTP and MCP.
-   Advisory-by-default matches 0012's stance on build order.

## Impact

-   Docs: `mcp-tools.md` (17 tools), `local-http-api.md`, `arch-model-format.md` (config keys, `.arch/methods/`), an amendment to 0008 (guidance now method-sourced).
-   `AuthoringGuidance` and `ShallowModelGuidance` constants stay; built-in method text references them so there is one copy.

## Follow-ups

-   [ ] More gate kinds (accept proposal while stage=build, scope writes).
-   [ ] Per-subtree method or lint overrides.
-   [ ] Pre-commit hook running `check-model --strict`.
-   [ ] Method sharing or registry; stage history log.

## References

-   Related: 0005 (brief), 0008 (guidance), 0010 (shallow models), 0012 (build order, constraints)
