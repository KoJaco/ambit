import type { NodeSnapshot, OpDiff, ProposalDiff } from "./api";

export type ReviewRow = {
    key: string;
    index: number;
    op: string;
    nodeId: string;
    status: string;
    stale: boolean;
    staleReason: string;
    summary: string;
    detail: string;
    canAccept: boolean;
    needsConfirm: boolean;
    // Review-panel presentation: names instead of ids, one verb, one change line.
    verb: RowVerb;
    title: string;
    change: string;
    groupKey: string;
};

export type RowVerb = "Create" | "Update" | "Delete" | "Connect" | "Remove connection" | "Other";

const missingNode = "node is missing";

// proposalRows shapes a server diff into one compact row per operation.
// Stale is copied from the payload. This function does not compare hashes.
export function proposalRows(diff: ProposalDiff): ReviewRow[] {
    const names = nameLookup(diff);
    return diff.operations.map((op) => {
        const missing = op.stale_reason === missingNode;
        const pending = op.status === "pending" && !op.error;
        return {
            key: `${op.index}:${op.op}:${op.node_id}`,
            index: op.index,
            op: op.op,
            nodeId: op.node_id,
            status: op.status,
            stale: op.stale,
            staleReason: op.stale_reason ?? "",
            summary: summarize(op),
            detail: detailOf(op),
            canAccept: pending && !missing,
            needsConfirm: op.stale && !missing && pending,
            verb: verbOf(op),
            title: titleOf(op, names),
            change: changeOf(op, names),
            groupKey: groupKeyOf(op, names),
        };
    });
}

function summarize(op: OpDiff): string {
    if (op.op === "create_node") {
        return `Create ${op.proposed?.name ?? op.node_id}`;
    }
    if (op.op === "delete_node") {
        return `Delete ${op.node_id}`;
    }
    if (op.op === "update_node") {
        const fields = op.fields?.length ? ` (${op.fields.join(", ")})` : "";
        return `Update ${op.node_id}${fields}`;
    }
    if (op.op === "set_relationship" && op.relationship) {
        return `${op.relationship.from} → ${op.relationship.to}`;
    }
    return `${op.op} ${op.node_id}`;
}

function detailOf(op: OpDiff): string {
    if (op.error) return op.error;
    if (op.op === "create_node" && op.proposed) {
        const parent = op.proposed.parent_id ? `parent ${op.proposed.parent_id}` : "root";
        return `${op.proposed.type} · ${parent}`;
    }
    if (op.op === "set_relationship" && op.relationship) {
        return [op.relationship.label, op.relationship.kind].filter(Boolean).join(" · ");
    }
    if (op.op === "update_node" && op.fields?.length) {
        return op.fields
            .map((field) => {
                if (field === "spec") return "spec changed";
                const current = fieldText(op.current, field);
                const proposed = fieldText(op.proposed, field);
                return `${field}: ${current} → ${proposed}`;
            })
            .join("; ");
    }
    return "";
}

function fieldText(node: NodeSnapshot | null, field: string): string {
    if (!node) return "—";
    switch (field) {
        case "name":
            return node.name;
        case "type":
            return node.type;
        case "status":
            return node.status;
        case "parent_id":
            return node.parent_id || "(root)";
        case "implementation":
            return node.implementation.join(", ") || "(empty)";
        case "scope":
            return node.scope.join(", ") || "(empty)";
        case "protected":
            return node.protected ? "protected" : "not protected";
        default:
            return field;
    }
}

// ---- review-panel presentation -------------------------------------------------------------

const FIELD_LABELS: Record<string, string> = {
    parent_id: "Parent",
    spec: "Spec",
    name: "Name",
    type: "Type",
    status: "Status",
    implementation: "Implemented in",
    scope: "Agent may edit",
    protected: "Protected",
    icon: "Icon",
    color: "Colour",
};

export function humanField(field: string): string {
    return FIELD_LABELS[field] ?? field.replace(/_/g, " ").replace(/^./, (c) => c.toUpperCase());
}

// id -> display name, from every snapshot in the proposal.
function nameLookup(diff: ProposalDiff): Map<string, string> {
    const names = new Map<string, string>();
    for (const op of diff.operations) {
        for (const snap of [op.current, op.proposed]) if (snap) names.set(snap.id || op.node_id, snap.name);
    }
    return names;
}

function verbOf(op: OpDiff): RowVerb {
    switch (op.op) {
        case "create_node":
            return "Create";
        case "update_node":
            return "Update";
        case "delete_node":
            return "Delete";
        case "set_relationship":
            return "Connect";
        case "delete_relationship":
            return "Remove connection";
        default:
            return "Other";
    }
}

function titleOf(op: OpDiff, names: Map<string, string>): string {
    if (op.op === "set_relationship" && op.relationship) {
        const { from, to } = op.relationship;
        return `${names.get(from) ?? from} → ${names.get(to) ?? to}`;
    }
    if (op.op === "delete_relationship" && op.relationship) {
        const { from, to } = op.relationship;
        return `${names.get(from) ?? from} → ${names.get(to) ?? to}`;
    }
    return op.proposed?.name ?? op.current?.name ?? names.get(op.node_id) ?? op.node_id;
}

function parentName(id: string | undefined, names: Map<string, string>): string {
    return id ? (names.get(id) ?? id) : "root";
}

function changeOf(op: OpDiff, names: Map<string, string>): string {
    if (op.error) return op.error;
    if (op.op === "create_node" && op.proposed) {
        return `${op.proposed.type} in ${parentName(op.proposed.parent_id, names)}`;
    }
    if (op.op === "delete_node") return "Removes the node";
    if (op.op === "delete_relationship") return removeConnectionChange(op);
    if (op.op === "set_relationship" && op.relationship) {
        return [op.relationship.label, op.relationship.kind].filter(Boolean).join(" · ");
    }
    if (op.op === "update_node" && op.fields?.length) {
        return op.fields
            .map((field) => {
                if (field === "spec") return "Spec changed";
                if (field === "parent_id") {
                    return `Parent: ${parentName(op.current?.parent_id, names)} → ${parentName(op.proposed?.parent_id, names)}`;
                }
                return `${humanField(field)}: ${fieldText(op.current, field)} → ${fieldText(op.proposed, field)}`;
            })
            .join("; ");
    }
    return "";
}

function removeConnectionChange(op: OpDiff): string {
    const label = op.relationship?.label;
    const removed = op.removes_nodes ?? [];
    const base = label ? `Removes connection "${label}"` : "Removes the connection";
    if (!removed.length) return base;
    return `${base} and ${removed.length} inside it: ${removed.map((n) => n.name || n.id).join(", ")}`;
}

// Rows with the same key are interchangeable edits, e.g. seven nodes moved to the root.
function groupKeyOf(op: OpDiff, names: Map<string, string>): string {
    if (op.op === "update_node" && op.fields?.length) {
        const target = op.fields.includes("parent_id") ? parentName(op.proposed?.parent_id, names) : "";
        return `update|${op.fields.join(",")}|${target}`;
    }
    return op.op === "set_relationship" ? "set_relationship" : op.op;
}

const CANVAS_SOURCES = new Set(["group_nodes", "ungroup_node", "reparent_node"]);

// Proposals staged from the canvas are the architect's own; anything else came through MCP.
export function proposalOrigin(source: string | undefined): "You" | "Agent" {
    return source && CANVAS_SOURCES.has(source) ? "You" : "Agent";
}

// ---- proposal-level summary and hierarchy ----------------------------------------------------

export type ProposalSummaryLine = {
    headline: string;
    source: string;
    // Pending operations whose target changed since the proposal was staged.
    staleCount: number;
};

function plural(n: number, one: string, many = `${one}s`): string {
    return `${n} ${n === 1 ? one : many}`;
}

// Plain-language count of what accepting the proposal would still do, e.g.
// "Adds 5 nodes · 4 connections; Changes 1; Removes 1 connection".
export function summarizeProposal(diff: ProposalDiff): ProposalSummaryLine {
    const pending = diff.operations.filter((op) => op.status === "pending");
    const count = (name: string) => pending.filter((op) => op.op === name).length;
    const nodesAdded = count("create_node");
    const edgesAdded = count("set_relationship");
    const changed = count("update_node");
    const nodesRemoved = count("delete_node");
    const edgesRemoved = count("delete_relationship");

    const sections: string[] = [];
    const adds = [nodesAdded ? plural(nodesAdded, "node") : "", edgesAdded ? plural(edgesAdded, "connection") : ""].filter(Boolean);
    if (adds.length) sections.push(`Adds ${adds.join(" · ")}`);
    if (changed) sections.push(`Changes ${changed}`);
    const removes = [nodesRemoved ? plural(nodesRemoved, "node") : "", edgesRemoved ? plural(edgesRemoved, "connection") : ""].filter(Boolean);
    if (removes.length) sections.push(`Removes ${removes.join(" · ")}`);
    const other = pending.length - nodesAdded - edgesAdded - changed - nodesRemoved - edgesRemoved;
    if (other > 0) sections.push(plural(other, "other change"));

    const origin = proposalOrigin(diff.source);
    const source = origin === "You" ? "Staged by you" : diff.source ? `Proposed by ${diff.source}` : "Proposed by an agent";
    return {
        headline: sections.length ? sections.join("; ") : "No pending changes",
        source,
        staleCount: pending.filter((op) => op.stale).length,
    };
}

// Ids of existing parent zones that hierarchySections would title, but no snapshot in the
// proposal names and `known` does not yet cover.
export function unresolvedParentIds(diff: ProposalDiff, known?: ReadonlyMap<string, unknown>): string[] {
    const names = nameLookup(diff);
    const out = new Set<string>();
    for (const op of diff.operations) {
        if (op.op === "set_relationship" || op.op === "delete_relationship") continue;
        const parent = (op.op === "create_node" ? op.proposed : (op.current ?? op.proposed))?.parent_id;
        if (parent && !names.has(parent) && !known?.has(parent)) out.add(parent);
    }
    return [...out];
}

export type HierarchyItem = { row: ReviewRow; label: string };
export type HierarchySection = {
    key: string;
    kind: "new-zone" | "existing-zone" | "connections";
    title: string;
    items: HierarchyItem[];
};

// Groups a proposal's rows by where they land: each new zone (a created node with created
// descendants, with the nodes and connections inside it), each existing zone that gets other
// changes (named from the proposal, else from `existingNames`), then one Connections section for edges that reach outside a new zone.
export function hierarchySections(diff: ProposalDiff, existingNames?: ReadonlyMap<string, string>): HierarchySection[] {
    const rows = proposalRows(diff);
    const names = nameLookup(diff);
    const nameOf = (id: string) => names.get(id) ?? existingNames?.get(id) ?? id;
    const opByIndex = new Map(diff.operations.map((op) => [op.index, op]));
    const parentOf = new Map<string, string>();
    for (const op of diff.operations) {
        if (op.op === "create_node" && op.proposed) parentOf.set(op.node_id, op.proposed.parent_id ?? "");
    }
    // The created ancestor with no created parent: the root of the new subtree a created node sits in.
    const newRoot = (id: string): string => {
        let current = id;
        for (let depth = 0; depth < 64; depth++) {
            const parent = parentOf.get(current);
            if (!parent || !parentOf.has(parent)) return current;
            current = parent;
        }
        return current;
    };
    const rootsWithChildren = new Set<string>();
    for (const id of parentOf.keys()) {
        const root = newRoot(id);
        if (root !== id) rootsWithChildren.add(root);
    }

    const zones = new Map<string, HierarchySection>();
    const sectionFor = (key: string, kind: HierarchySection["kind"], title: string) => {
        let section = zones.get(key);
        if (!section) {
            section = { key, kind, title, items: [] };
            zones.set(key, section);
        }
        return section;
    };
    const connections: HierarchySection = { key: "connections", kind: "connections", title: "Connections", items: [] };
    const label = (row: ReviewRow): string => {
        const rel = opByIndex.get(row.index)?.relationship;
        return rel?.label ? `${row.title} "${rel.label}"` : row.title;
    };
    const existingZone = (parentId: string | undefined) =>
        sectionFor(`in:${parentId ?? ""}`, "existing-zone", parentId ? `In ${nameOf(parentId)}` : "At the top level");

    for (const row of rows) {
        const op = opByIndex.get(row.index);
        const item = { row, label: label(row) };
        if (!op) continue;
        if (op.op === "set_relationship" || op.op === "delete_relationship") {
            const rel = op.relationship;
            const from = rel ? newRoot(rel.from) : "";
            const to = rel ? newRoot(rel.to) : "";
            const inside = rel && parentOf.has(rel.from) && parentOf.has(rel.to) && from === to && rootsWithChildren.has(from);
            if (inside) sectionFor(`new:${from}`, "new-zone", `New zone: ${nameOf(from)}`).items.push(item);
            else connections.items.push(item);
            continue;
        }
        if (op.op === "create_node") {
            const root = newRoot(op.node_id);
            if (rootsWithChildren.has(root)) {
                sectionFor(`new:${root}`, "new-zone", `New zone: ${nameOf(root)}`).items.push(item);
            } else {
                existingZone(op.proposed?.parent_id).items.push(item);
            }
            continue;
        }
        existingZone((op.current ?? op.proposed)?.parent_id).items.push(item);
    }
    const all = [...zones.values()];
    const ordered = [
        ...all.filter((s) => s.kind === "new-zone"),
        ...all.filter((s) => s.kind === "existing-zone"),
        ...(connections.items.length ? [connections] : []),
    ];
    return ordered;
}
