# Shape registry sketch (non-normative)

Draft for stage 07 implementation. IDs and suggestions may change before ship.

## Default

| Id | Render |
| --- | --- |
| `standard` | Current NodeCard: rounded rectangle, header + type/status (default when field absent) |

Aliases in docs only: `normal` → treat as `standard` on read if we allow both; prefer one id in JSON.

## Sparse v1 candidates

| Id | Intent | Suggested (not required) |
| --- | --- | --- |
| `hex` | Generic component / module | `type`: module, component, package |
| `cylinder` | Data store | `type`: database, cache, queue; `icon`: database |
| `cloud` | External or hosted boundary | `type`: service, saas, external |
| `document` | Spec-heavy / note | `type`: note, doc, adr |
| `person` | Human / role (rare) | `type`: actor, team |

## Suggested relations

**Normative:** none — suggestions are for picker UI and MCP tool descriptions only.

**Documented hints (examples):**

- When connecting `cylinder` → `hex` with a data edge, UI copy might suggest label “reads from” and `kind` `data`.
- When connecting `cloud` → `hex`, suggest “calls” / `sync`.

Implement as markdown tables in the contract and inspector “insert relationship” helpers later; do not auto-mutate relationships when shape changes.

## Unknown id

Same posture as icons: load warning or fallback to `standard`; never crash the canvas.
