import type { NodeSnapshot, OpDiff, RelSnapshot } from "./api";

export type DetailBlock =
    | { kind: "heading"; text: string }
    | { kind: "line"; text: string }
    | { kind: "pair"; label: string; before: string; after: string }
    | { kind: "prose"; label: string; before: string; after: string };

const missingNode = "node is missing";

export function proposalDetailBlocks(op: OpDiff): DetailBlock[] {
    const blocks: DetailBlock[] = [];
    blocks.push({ kind: "heading", text: summarizeOp(op) });
    blocks.push({ kind: "line", text: `Status: ${op.status}` });
    if (op.error) {
        blocks.push({ kind: "line", text: op.error });
    }
    if (op.stale) {
        blocks.push({
            kind: "line",
            text: staleMessage(op),
        });
    }

    switch (op.op) {
        case "create_node":
            if (op.proposed) {
                blocks.push(...nodeSnapshotBlocks("Proposed", op.proposed));
            }
            break;
        case "update_node":
            for (const field of op.fields ?? []) {
                if (field === "spec") {
                    blocks.push({
                        kind: "prose",
                        label: "Spec",
                        before: op.current?.markdown ?? "—",
                        after: op.proposed?.markdown ?? "—",
                    });
                } else {
                    blocks.push({
                        kind: "pair",
                        label: field,
                        before: fieldText(op.current, field),
                        after: fieldText(op.proposed, field),
                    });
                }
            }
            break;
        case "delete_node":
            if (op.current) {
                blocks.push(...nodeSnapshotBlocks("Current", op.current));
            }
            break;
        case "set_relationship":
            if (op.relationship) {
                blocks.push(...relationshipBlocks(op.relationship, op.current_relationship));
            }
            break;
        default:
            blocks.push({ kind: "line", text: `${op.op} ${op.node_id}` });
    }

    return blocks;
}

export function detailCanAccept(op: OpDiff): boolean {
    const missing = op.stale_reason === missingNode;
    return op.status === "pending" && !op.error && !missing;
}

export function detailNeedsConfirm(op: OpDiff): boolean {
    const missing = op.stale_reason === missingNode;
    return op.stale && !missing && op.status === "pending" && !op.error;
}

function staleMessage(op: OpDiff): string {
    if (op.stale_reason === missingNode) {
        return "Stale: the node is missing on disk. Reject this operation; it cannot be applied.";
    }
    if (op.stale_reason === "base_hash mismatch") {
        return "Stale: the node changed after this was staged. Accept applies the proposed version over your newer edit.";
    }
    return `Stale: ${op.stale_reason ?? "unknown reason"}`;
}

function summarizeOp(op: OpDiff): string {
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

function nodeSnapshotBlocks(title: string, node: NodeSnapshot): DetailBlock[] {
    return [
        { kind: "heading", text: title },
        { kind: "pair", label: "name", before: "—", after: node.name },
        { kind: "pair", label: "type", before: "—", after: node.type },
        { kind: "pair", label: "status", before: "—", after: node.status },
        {
            kind: "pair",
            label: "parent",
            before: "—",
            after: node.parent_id ? node.parent_id : "(root)",
        },
        {
            kind: "pair",
            label: "implementation",
            before: "—",
            after: node.implementation.join(", ") || "(empty)",
        },
        {
            kind: "pair",
            label: "scope",
            before: "—",
            after: node.scope.join(", ") || "(empty)",
        },
        {
            kind: "pair",
            label: "protected",
            before: "—",
            after: node.protected ? "yes" : "no",
        },
        { kind: "prose", label: "Spec", before: "", after: node.markdown || "(empty)" },
    ];
}

function relationshipBlocks(proposed: RelSnapshot, current: RelSnapshot | null): DetailBlock[] {
    const blocks: DetailBlock[] = [
        { kind: "heading", text: "Relationship" },
        { kind: "line", text: `${proposed.from} → ${proposed.to}` },
    ];
    if (current) {
        blocks.push({
            kind: "pair",
            label: "label",
            before: current.label || "(empty)",
            after: proposed.label || "(empty)",
        });
        blocks.push({
            kind: "pair",
            label: "kind",
            before: current.kind || "(empty)",
            after: proposed.kind || "(empty)",
        });
    } else {
        blocks.push({ kind: "pair", label: "label", before: "—", after: proposed.label || "(empty)" });
        blocks.push({ kind: "pair", label: "kind", before: "—", after: proposed.kind || "(empty)" });
    }
    return blocks;
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
