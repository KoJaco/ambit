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
};

const missingNode = "node is missing";

// proposalRows shapes a server diff into one compact row per operation.
// Stale is copied from the payload. This function does not compare hashes.
export function proposalRows(diff: ProposalDiff): ReviewRow[] {
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
