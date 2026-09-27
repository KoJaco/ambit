# Stage 03 tasks

Working list. Status is mirrored in [`checklist.md`](../../checklist.md).

HTTP tasks come first. The frontend reset targets that API, not a mock. Within the reset,
deletes (03.10) land before the rebuild, and the tree is allowed not to build in between.

## 03.1 Localhost server

- [x]

**Touches:** `cmd/ambit`, `internal/httpapi`.

**Done when:** `ambit start` listens on a localhost address only. Binding `0.0.0.0` is
rejected or impossible through the flags this command exposes. There is no auth and no
outbound call. The process loads `.arch` through `ambit-core`.

## 03.2 One drill-down level

- [x]

**Touches:** `internal/core`, `internal/httpapi`.

**Done when:** a core query and its HTTP endpoint return the node, its direct children, and
the relationships whose endpoints are both in that child set. Root is the nodes with no
`parent_id`. The response never includes the rest of the graph. A relationship that leaves
the level is omitted here; the client renders a boundary marker from a separate signal the
endpoint may include, and it does not invent the missing node.

## 03.3 Single node

- [x]

**Touches:** `internal/httpapi`.

**Done when:** fetching one node returns its structured fields and its markdown. Unknown id
is 404 and names the id.

## 03.4 Integrity

- [ ]

**Touches:** `internal/core`, `internal/httpapi`.

**Done when:** orphans and dangling references from stage 01 are available on a read and on
their own endpoint. They are warnings. A model with one still loads. Warning text names the
node or the file.

## 03.5 Direct mutations

- [ ]

**Touches:** `internal/httpapi`.

**Done when:** create, update, delete, and set relationship call the stage 01 mutation
functions and write the canonical model immediately. `status` is updatable here. Validation
failures use 409 and name the node and the rule. There is no HTTP-specific validation.

## 03.6 Assignment

- [ ]

**Touches:** `internal/core`, `internal/httpapi`,
[`local-http-api.md`](../../../contracts/local-http-api.md).

**Done when:** the architect can set or clear the active assignment, stored only in
`local.json` as the format already specifies. The HTTP contract gains the endpoint in the
same change. Setting assignment does not by itself change the node's `status`; `assigned`
remains a status value the architect or, later, an agent sets explicitly. Clearing
assignment leaves the model otherwise untouched.

## 03.7 Layout cache

- [ ]

**Touches:** `internal/core`, `internal/httpapi`.

**Done when:** positions for a level can be read and written under `.arch/.cache/layout/`,
keyed by the same id the route will use (root needs a stable key, documented next to the
endpoint). A missing cache is an empty result, not an error. Writes never create or modify
files under `nodes/` or `index.json`. Structural changes (create, delete, reparent) mark
that level's cache stale so the client recomputes. A field edit does not.

## 03.8 SSE for model and integrity

- [ ]

**Touches:** `internal/httpapi`. Uses the stage 01 file watcher.

**Done when:** `/events` is a one-way stream. Model-changed carries the affected node ids.
Integrity-changed fires when the warning set changes. The client cannot send on this
stream. Proposal events are not emitted yet.

## 03.9 Ignore check on start

- [ ]

**Touches:** `cmd/ambit`.

**Done when:** `ambit start` calls the stage 02 `git check-ignore` function before it
serves, and a failure is a loud warning that does not by itself refuse to serve.

## 03.10 Delete the pipeline domain

- [ ]

**Touches:** `frontend/`, per [`frontend-refactor.md`](frontend-refactor.md) "Delete".

**Done when:** the pipeline modules and the abandoned `app/components/canvas/` tree are
gone, `screen-size-alert.tsx` has been salvaged out of that tree first, and nothing
remaining imports the deleted modules. The frontend is expected not to build at the end of
this task.

## 03.11 Routes and React Flow v12

- [ ]

**Touches:** `frontend/app/routes.ts`, `frontend/app/routes/home.tsx`, `frontend/package.json`.

**Done when:** `/` is the root level and `/node/:nodeId` is a drill-down. `reactflow` v11
and `@reactflow/node-resizer` are uninstalled. Imports use `@xyflow/react`. `serve`,
`isbot`, and `@react-router/node` are gone. `ssr` stays false. No loader or action performs
work.

## 03.12 Canvas

- [ ]

**Touches:** `frontend/app/components/FlowCanvas.tsx`, `NodeCard.tsx`, `ControlBar.tsx`.

**Done when:** the canvas renders exactly the nodes and edges the level endpoint returned.
Relationships are directed and labelled. A cross-level relationship shows as a marker on
the node that is on screen. Drill-in navigates to `/node/:nodeId`. An unknown `type` uses
a generic shape and does not warn. The control bar's tool union still works.

## 03.13 Inspector

- [ ]

**Touches:** `frontend/app/components/node-inspector.tsx`.

**Done when:** the inspector edits `name`, free-text `type`, `status`, `implementation`,
`scope`, `protected`, and the markdown, through the mutation endpoints. Empty `scope` is
labelled as defaulting to `implementation`. `protected` is labelled as applying regardless
of assignment. There is no type dropdown and no port form.

## 03.14 Sidebar

- [ ]

**Touches:** `frontend/app/components/Sidebar.tsx`, `Sidebar.module.css`.

**Done when:** the sidebar navigates the hierarchy and can create a node. It does not list
a fixed set of kinds. Collapse behaviour and the module CSS remain.

## 03.15 API client and SSE client

- [ ]

**Touches:** new modules under `frontend/app/`.

**Done when:** a typed client covers the endpoints this stage serves, and an SSE client
refetches on model-changed and integrity-changed. Unknown event names are ignored.

## 03.16 elkjs

- [ ]

**Touches:** `frontend/`, `elkjs` dependency.

**Done when:** a level with no cache is laid out in the browser and the positions are
written back through 03.7. A level with a cache renders those positions and does not rerun
elk. Dragging a node writes the same cache. Adding, removing, or reparenting a node on that
level reruns elk. Field edits do not.

## 03.17 Vitest

- [ ]

**Touches:** `frontend/` test setup.

**Done when:** Vitest runs layout-cache keying and the pure function that previews which
paths a scope covers. No component tests. The scope preview cases are listed in the test
plan. They are not imported from the Go suite.

## 03.18 Template leftovers

- [ ]

**Touches:** `frontend/README.md`, `frontend/Dockerfile`, `frontend/.dockerignore`,
`frontend/app/welcome/`.

**Done when:** `app/welcome/` is gone, the frontend README describes the ambit SPA and how
to run it against `ambit start`, and `Dockerfile` / `.dockerignore` are deleted after a
search shows nothing in the repo still invokes them. This closes the follow-up left in
[decision note 0001](../../../decisions/0001-repo-topology.md).
