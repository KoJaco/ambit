// Undo/redo for accepted proposals (group, ungroup, reparent, agent edits). A proposal only
// changes the model when an operation is accepted, so history is recorded at accept time and
// replays the operations' before/after snapshots through the direct edit API.

import {
    createNode,
    deleteNode,
    deleteRelationship,
    setRelationship,
    updateNode,
    type NodePatch,
    type NodeSnapshot,
    type OpDiff,
    type RelSnapshot,
} from "./api";
import type { HistoryEntry } from "./history";

export type Step =
    | { kind: "create"; node: NodeSnapshot }
    | { kind: "patch"; id: string; patch: NodePatch }
    | { kind: "delete"; id: string }
    | { kind: "set-rel"; rel: RelSnapshot }
    | { kind: "delete-rel"; id: string };

// Fields an update operation may list that history knows how to restore. "spec" is the markdown.
const RESTORABLE = ["name", "type", "status", "parent_id", "implementation", "scope", "protected", "spec", "icon", "color", "depends_on", "contract"];

function patchFrom(node: NodeSnapshot, fields: string[]): NodePatch | null {
    const patch: NodePatch = {};
    for (const field of fields) {
        if (!RESTORABLE.includes(field)) return null;
        switch (field) {
            case "spec":
                patch.markdown = node.markdown;
                break;
            case "parent_id":
                patch.parent_id = node.parent_id ?? "";
                break;
            case "icon":
                patch.icon = node.icon ?? "";
                break;
            case "color":
                patch.color = node.color ?? "";
                break;
            case "depends_on":
                patch.depends_on = node.depends_on ?? [];
                break;
            case "contract":
                patch.contract = node.contract ?? {};
                break;
            default:
                (patch as Record<string, unknown>)[field] = node[field as keyof NodeSnapshot];
        }
    }
    return patch;
}

// Steps that move the model from before an op to after it (redo), and back (undo).
// Returns null when an op cannot be reversed exactly, so the caller clears history instead.
function opSteps(op: OpDiff): { forward: Step[]; backward: Step[] } | null {
    switch (op.op) {
        case "create_node":
            if (!op.proposed) return null;
            return {
                forward: [{ kind: "create", node: op.proposed }],
                backward: [{ kind: "delete", id: op.node_id }],
            };
        case "delete_node":
            if (!op.current) return null;
            return {
                forward: [{ kind: "delete", id: op.node_id }],
                backward: [{ kind: "create", node: op.current }],
            };
        case "update_node": {
            if (!op.current || !op.proposed || !op.fields?.length) return null;
            const forward = patchFrom(op.proposed, op.fields);
            const backward = patchFrom(op.current, op.fields);
            if (!forward || !backward) return null;
            return {
                forward: [{ kind: "patch", id: op.node_id, patch: forward }],
                backward: [{ kind: "patch", id: op.node_id, patch: backward }],
            };
        }
        case "set_relationship": {
            if (!op.relationship) return null;
            const before = op.current_relationship;
            const { contract_cleared: cleared, ...proposed } = op.relationship;
            // An edge contract left out of a set keeps the old one, so a clear is sent as {} on redo too.
            const forwardRel: RelSnapshot = cleared && !proposed.contract ? { ...proposed, contract: {} } : proposed;
            return {
                forward: [{ kind: "set-rel", rel: forwardRel }],
                // Undo clears an added contract explicitly and restores a cleared one.
                backward: before
                    ? [{ kind: "set-rel", rel: forwardRel.contract && !before.contract ? { ...before, contract: {} } : before }]
                    : op.relationship.id
                      ? [{ kind: "delete-rel", id: op.relationship.id }]
                      : [],
            };
        }
        default:
            return null;
    }
}

export type AcceptedPlan = { redo: Step[]; undo: Step[] };

// ops are in application order. Undo runs them backwards.
export function planAccepted(ops: OpDiff[]): AcceptedPlan | null {
    const redo: Step[] = [];
    const undo: Step[] = [];
    for (const op of ops) {
        const steps = opSteps(op);
        if (!steps) return null;
        redo.push(...steps.forward);
        undo.unshift(...steps.backward);
    }
    return { redo, undo };
}

export type StepApi = {
    createNode: typeof createNode;
    updateNode: typeof updateNode;
    deleteNode: typeof deleteNode;
    setRelationship: typeof setRelationship;
    deleteRelationship: typeof deleteRelationship;
};

const liveApi: StepApi = { createNode, updateNode, deleteNode, setRelationship, deleteRelationship };

export async function runSteps(steps: Step[], api: StepApi = liveApi): Promise<void> {
    for (const step of steps) {
        switch (step.kind) {
            case "create": {
                const n = step.node;
                const created = await api.createNode({
                    name: n.name,
                    type: n.type,
                    markdown: n.markdown,
                    icon: n.icon,
                    color: n.color,
                    ...(n.depends_on?.length ? { depends_on: n.depends_on } : {}),
                    ...(n.contract ? { contract: n.contract } : {}),
                    ...(n.relationship_id ? { relationship_id: n.relationship_id } : { parent_id: n.parent_id ?? "" }),
                });
                // Ids come from the name; a collision would yield a different id and break later steps.
                if (created.id !== n.id) {
                    await api.deleteNode(created.id);
                    throw new Error(`could not restore ${n.name} with its original id`);
                }
                await api.updateNode(n.id, {
                    status: n.status,
                    implementation: n.implementation,
                    scope: n.scope,
                    protected: n.protected,
                });
                break;
            }
            case "patch":
                await api.updateNode(step.id, step.patch);
                break;
            case "delete":
                await api.deleteNode(step.id);
                break;
            case "set-rel":
                await api.setRelationship(step.rel);
                break;
            case "delete-rel":
                await api.deleteRelationship(step.id);
                break;
        }
    }
}

// Entry for the history stack, or null when the accepted ops cannot be reversed exactly.
export function acceptedEntry(label: string, ops: OpDiff[]): HistoryEntry | null {
    const plan = planAccepted(ops);
    if (!plan || plan.undo.length === 0) return null;
    return {
        label,
        undo: () => runSteps(plan.undo),
        redo: () => runSteps(plan.redo),
    };
}
