# Decision Note: Architect auto-accept policy

## Date

2026-11-28

## Summary

In the build stage a human can allow an architect agent to accept the low-risk proposals that implementer agents stage, so routine spec, contract and scope edits stop queuing for review. Everything else stays human. The policy is set only through the HTTP API, the tool exists only under `ambit mcp --role architect`, every auto-accept is audited, and an agent cannot accept its own proposal.

## Decision

1. Config `proposal_policy: {"auto_accept": bool}`, default off, effective only when the effective stage is `build`. It is written only by `PUT /policy/proposals`. No MCP tool changes it.
2. `core/proposal_policy.go` has a pure classifier, `ClassifyOps`, over each proposal operation. A proposal is auto-acceptable only if every operation is.
   - Auto: `update_node` whose changed fields are only `spec`/`contract`, `depends_on` (removals, or additions that keep the graph acyclic and name no protected or external node), or `implementation`/`scope` additions; `create_node` under an existing non-protected, non-external container with no external or protected dependencies, no protected overlap and none of the sensitive fields set; `set_relationship` between existing, non-protected, non-external nodes with an unchanged kind (a new edge may be `sync` or have no kind).
   - Changed fields are computed by diffing the node on disk against the staged body (every field, unknown keys included), never from the operation's declared `fields`, because accept applies the whole staged body. Create and update share one sensitive-field list (`stage`, `risk`, `review`, `human_gates`, `shared`, `protected`, `verify`, `gates`, `external`); a create that sets any of them needs a human. Deliverables, references and milestones are allowed on a create.
   - Scope additions (update or create) are auto only for narrow globs: no `**`; an update's glob must match one of the node's own `deliverables`; every existing repo file (`ProjectFiles` of the real repo) the glob matches must be a declared deliverable of the node or already owned by it (a create owns nothing, so it may match no existing file); and the glob must not overlap a protected node's globs, judged by glob-against-glob segment comparison, sample paths and the real files. If the file list cannot be read, scope additions need a human.
   - Always human: anything touching a `protected` node; new nodes with `external` or type `external`, or under an external parent; a `depends_on` addition naming an external node; changes to `stage`, `risk`, `review`, `human_gates`, `shared`, `protected` and other fields not listed above; deletes; renames; constraint changes; `set_part`/`delete_part`; stale operations; restaged proposals; anything that does not apply cleanly or adds an enforced lint finding.
   - Each staged body is hashed when read for classification and re-hashed immediately before its operation applies; a mismatch fails the operation, rolls back and refuses (`ErrStale`).
3. After classification the proposal is applied to a scratch copy of `.arch` and `check_model` (lint only) runs on both. A new enforced finding blocks the accept. The scratch copy contains only `.arch`, so the trial index is pointed at the real repo root for file listings and repo-file rules (`shared-manifest`) are enforced in the trial. Constraints are free text with no machine evaluation, so a constraint cannot be "violated" mechanically; `set_constraints` is simply always human.
4. MCP tool `accept_proposal {proposal_id}` is registered only with `--role architect`. It refuses unless the policy is on and the stage is `build` (config is re-read first), refuses when the accepting session's origin equals the proposal's origin (UI-staged proposals have none and are fine), and refuses an accepting session with no origin. When any operation is blocking it applies nothing and returns the blocking operations with reasons. On success it runs the existing accept-all path (snapshot and rollback).
5. Audit log: append-only `.arch/.proposals/audit.jsonl` (the directory is gitignored): `{ts, proposal_id, actor: "architect-agent"|"human", origin_hash, accepter_hash?, ops: [{index, op, node, rule, diff_summary}]}`. Origins are stored as short hashes. The log is a local, agent-writable record for human review, not a tamper-evident backstop: an agent with a shell can edit or delete it. Human accept-all is logged too (rule `human`), best effort. `GET /proposals/audit?since=` lists entries and `GET /proposals/audit/digest` groups counts by rule.

## Rationale

- A 20-agent build showed the architect queue is dominated by edits nobody needs to read closely. The risky ones are few and easy to name.
- Default-deny: an operation that is not on the allow list needs a human, including future operation types.
- The scratch trial reuses the real accept and lint code instead of re-deriving what "valid after apply" means.
- The session-origin check stops the most obvious failure (an agent approving itself). It is a guard rail, like the stage gate: an agent with a shell can edit `config.json`. Branch protection and CI are the backstop; the audit log is a review aid, not a security control.

## Deviations from the plan

- The architect role is the `ambit mcp --role architect` flag, not a config key, and there is no check that the accepting session is an assigned architect.
- The self-accept guard is origin-based (the accepting session's origin must differ from the proposal's). A restaged proposal keeps its origin and is marked `restaged: true` in its manifest; auto-accept refuses restaged proposals outright.

## Impact

New config key, `accept_proposal` (opt-in role), four HTTP routes, a new log file, `ambit mcp --role architect`. `AcceptAll` over HTTP now also appends an audit entry. Existing proposals and tools are unchanged.

## Follow-ups

- [ ] UI: policy toggle and an "Architect accepted" digest panel.
- [ ] Log per-operation human accepts.
- [ ] Optionally cap the audit log size.

## References

- Related: decision 0015 (completion policy), `docs/contracts/mcp-tools.md`, `docs/contracts/local-http-api.md`, `docs/contracts/proposals.md`.
