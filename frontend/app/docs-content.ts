// In-app documentation as typed data, rendered by components/docs-sheet.tsx. Condensed from
// docs/planning/spec-v1.md, docs/decisions/0010-*.md, docs/contracts/mcp-tools.md and
// docs/runbooks/using-ambit-in-your-app.md. Keep it short; link out to those for detail.

export type Inline = string | { code: string } | { action: "open-shortcuts"; text: string };

export type DocBlock =
    | { type: "p"; content: Inline[] }
    | { type: "ul"; items: Inline[][] }
    | { type: "ol"; items: Inline[][] }
    | { type: "h3"; text: string }
    | { type: "code"; text: string; caption?: string };

export type DocSection = { id: string; title: string; blocks: DocBlock[] };

export const mcpConfigSnippet = `{
  "mcpServers": {
    "ambit": { "command": "ambit", "args": ["mcp"] }
  }
}`;

export const docsSections: DocSection[] = [
    {
        id: "what-its-for",
        title: "What ambit is for",
        blocks: [
            {
                type: "p",
                content: [
                    "ambit is architect-first. You draw the system; coding agents work inside the boundary you set. They read your model, propose changes to it, and are checked against it.",
                ],
            },
            {
                type: "p",
                content: [
                    "The model lives in your repo under ",
                    { code: ".arch/" },
                    ", one file per node, so it diffs and merges like code. Agents cannot change it directly: their edits are staged as proposals that you accept or reject here.",
                ],
            },
        ],
    },
    {
        id: "core-ideas",
        title: "Core ideas",
        blocks: [
            { type: "h3", text: "Nodes and hierarchy" },
            {
                type: "p",
                content: [
                    "A node is a part of your system: a service, a store, a module. A node can have children; ",
                    { code: "parent_id" },
                    " is the only hierarchy. Relationships connect nodes and belong to node ids, not to levels, so regrouping never breaks them.",
                ],
            },
            { type: "h3", text: "Groups, summary cards and Open" },
            {
                type: "ul",
                items: [
                    ["A group is a labelled frame with its children drawn inside it, on the same canvas. It has no Open link and does not drill on double-click. Collapse it to a summary card first, then Open. Expanding and collapsing is view state, not part of the model."],
                    ["A summary card is a collapsed node: one card with a child count and a type mix. Double-click it, or press Open, to drill in."],
                    ["Small groups open as frames by default and crowded ones collapse. Your own choice is never overridden. This is view state kept in the gitignored cache, not in the model."],
                ],
            },
            { type: "h3", text: "Scope" },
            {
                type: "ul",
                items: [
                    [{ code: "Implemented in" }, ": the files this node is built from (globs)."],
                    [{ code: "Agent may edit" }, ": the files an agent assigned to this node may touch. When empty it defaults to Implemented in."],
                    ["Protected: no agent may modify a protected node's files, whatever its assignment. You edit those by hand."],
                ],
            },
            { type: "h3", text: "Assignment" },
            {
                type: "p",
                content: [
                    "Select a node and choose Assign agent to this node in the inspector. The node gets an Agent chip. The assignment is local to your machine (",
                    { code: ".arch/local.json" },
                    "), is not committed, and does not change the node's status.",
                ],
            },
        ],
    },
    {
        id: "canvas",
        title: "Working on the canvas",
        blocks: [
            {
                type: "ul",
                items: [
                    ["Create nodes and child nodes from the sidebar. Select a node to inspect and edit it."],
                    ["Group: select two or more sibling nodes and choose Group, then name the container. Ungroup: use the container's Ungroup control, and optionally delete the container too."],
                    ["Both are staged as a single proposal. Nothing changes until you accept it in Proposals."],
                    ["Drill in with Open on a summary card (collapse a group first); use the breadcrumbs to come back up."],
                    ["Review proposals from agents in the Proposals panel and accept or reject each operation."],
                    ["Undo and redo are in the toolbar but stay disabled until canvas history is available. Until then, reject a staged proposal to back out of a structure change."],
                ],
            },
        ],
    },
    {
        id: "connect-agent",
        title: "Connecting an agent",
        blocks: [
            {
                type: "ol",
                items: [
                    ["In your app repo, scaffold the model:"],
                    ["Serve the UI and API (127.0.0.1:8080 by default):"],
                    ["Add the MCP server to your harness config, for example Claude Code's .mcp.json or Cursor's .cursor/mcp.json:"],
                    ["Before committing, check changed files against the assigned node:"],
                    ["Optionally enforce it on every commit:"],
                ],
            },
            { type: "code", text: "ambit init", caption: "Step 1" },
            { type: "code", text: "ambit start", caption: "Step 2" },
            { type: "code", text: mcpConfigSnippet, caption: "Step 3" },
            { type: "code", text: "ambit check", caption: "Step 4" },
            { type: "code", text: "ambit hook install", caption: "Step 5" },
            {
                type: "p",
                content: [
                    "The agent calls ",
                    { code: "get_context" },
                    " for its brief, ",
                    { code: "check_scope" },
                    " before writing and ",
                    { code: "update_node_status" },
                    " when done. Its authoring tools only stage proposals.",
                ],
            },
        ],
    },
    {
        id: "keyboard",
        title: "Keyboard shortcuts",
        blocks: [
            {
                type: "p",
                content: [
                    "V and H switch tools, Ctrl/Cmd+Z undoes, +/- zoom and 0 re-centers. Select several nodes with Shift or Ctrl/Cmd and click; box select by holding Shift and dragging. ",
                    { action: "open-shortcuts", text: "Open the full list" },
                    " or press ?.",
                ],
            },
        ],
    },
];

export function inlineText(content: Inline[]): string {
    return content.map((part) => (typeof part === "string" ? part : "code" in part ? part.code : part.text)).join("");
}

export function findSection(id: string): DocSection | undefined {
    return docsSections.find((section) => section.id === id);
}
