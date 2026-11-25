# Decision Note: Completion policy, review metadata, proposal records, check_done

## Date

2026-11-25

## Summary

Methods gain a completion policy and a `needs_review` status, nodes gain optional `risk`, `review` and `verify`, projects can override method parameters in `config.json`, finished proposals stay queryable and restageable, and agents get `check_done`, `get_proposal` and `wait_for_proposal`.

## Context

A design-first build with about 20 parallel agents showed: the brief told agents to finish by calling `update_node_status`, which clashed with "do not set done, I review first"; the user wrote risk and review text into each prompt by hand; nothing could change the built-in `max-depth` parameter; the model had to ask the user to type "accepted" after staging; and agents assembled "am I done?" from three tools. Undo after accepting a proposal reverted the model but showed nothing to review.

## Decision

1.  Status `needs_review` sits between `in_progress` and `done`. It is not done for the build order.
2.  Method `completion`: `agent_may_complete` (default true) and `review_required` (default false). The gate applies to MCP `update_node_status` only, like the stage gate. A node's `review` overrides `review_required`. Built-in methods keep the open default so existing projects behave as before; projects opt in.
3.  `config.json` `method_overrides` (`lint`, `completion`, `check`) applies over the active method without writing a method file. The built-in method object is copied, never mutated.
4.  Node `risk`, `review`, `verify` are plain optional fields. Risk has no behaviour of its own: it appears in the brief and `check_done`. `verify` commands are listed, not run, so ambit executes nothing.
5.  `check_done` derives changed files from git (merge-base with `main` or `master`, plus untracked), ignores `.arch/` (status updates and accepted proposals are not agent code), and combines scope, node drift, contract-change and policy.
6.  Finished proposals are copied to `.arch/.proposals/.resolved/<id>/` (the 100 most recently resolved, gitignored) with final statuses. `get_proposal` reads pending or finished; `wait_for_proposal` polls. Chosen over a new event channel: it works over stdio MCP with no server push.
7.  `Restage` copies accepted operations into a new pending proposal, keeping base hashes. The UI undoes an accepted proposal by restoring the model and restaging, so the architect sees the changes again. Chosen over reverting without a trace.
8.  `seed_model` orders creates parent-first and dependency-first, so forward references in one call work.

## Impact

Additive format changes (`risk`, `review`, `verify`, `method_overrides`, `completion`). Older versions preserve unknown keys. The `superseded` proposal state in the earlier plan became `withdrawn` (a deleted proposal); nothing supersedes proposals automatically.

## Follow-ups

Machine-checkable contracts (OpenAPI, JSON Schema). A pre-commit or CI scope check per node (`ambit check --node`). Running `verify` commands behind an explicit opt-in. A `shared` node type with its own lint, if the modelling guidance proves not to be enough.
