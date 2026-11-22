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

export type RowVerb = "Create" | "Update" | "Delete" | "Connect" | "Other";

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
        default:
            return "Other";
    }
}

function titleOf(op: OpDiff, names: Map<string, string>): string {
    if (op.op === "set_relationship" && op.relationship) {
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

// Rows with the same key are interchangeable edits, e.g. seven nodes moved to the root.
function groupKeyOf(op: OpDiff, names: Map<string, string>): string {
    if (op.op === "update_node" && op.fields?.length) {
        const target = op.fields.includes("parent_id") ? parentName(op.proposed?.parent_id, names) : "";
        return `update|${op.fields.join(",")}|${target}`;
    }
    return op.op === "set_relationship" ? "set_relationship" : op.op;
}

export type RowGroup = { key: string; verb: RowVerb; title: string; rows: ReviewRow[] };
export type RowItem = { kind: "row"; row: ReviewRow } | { kind: "group"; group: RowGroup };

const GROUP_MIN = 3;

// Collapses runs (same group key, 3 or more) into one expandable item, keeping order of first sight.
export function groupRows(rows: ReviewRow[]): RowItem[] {
    const counts = new Map<string, number>();
    for (const row of rows) counts.set(row.groupKey, (counts.get(row.groupKey) ?? 0) + 1);
    const items: RowItem[] = [];
    const open = new Map<string, RowGroup>();
    for (const row of rows) {
        if ((counts.get(row.groupKey) ?? 0) < GROUP_MIN) {
            items.push({ kind: "row", row });
            continue;
        }
        let group = open.get(row.groupKey);
        if (!group) {
            group = { key: row.groupKey, verb: row.verb, title: "", rows: [] };
            open.set(row.groupKey, group);
            items.push({ kind: "group", group });
        }
        group.rows.push(row);
    }
    for (const group of open.values()) group.title = groupTitle(group);
    return items;
}

function groupTitle(group: RowGroup): string {
    const n = group.rows.length;
    const [kind, fields, target] = group.key.split("|");
    if (group.verb === "Create") return `Create ${n} nodes`;
    if (group.verb === "Delete") return `Delete ${n} nodes`;
    if (group.verb === "Connect") return `Connect ${n} relationships`;
    if (kind === "update" && fields === "parent_id") return `Move ${n} nodes to ${target || "root"}`;
    return `Update ${n} nodes (${(fields ?? "").split(",").map(humanField).join(", ")})`;
}

const CANVAS_SOURCES = new Set(["group_nodes", "ungroup_node", "reparent_node"]);

// Proposals staged from the canvas are the architect's own; anything else came through MCP.
export function proposalOrigin(source: string | undefined): "You" | "Agent" {
    return source && CANVAS_SOURCES.has(source) ? "You" : "Agent";
}
