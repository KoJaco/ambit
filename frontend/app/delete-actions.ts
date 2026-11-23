// Deleting nodes from the canvas, with a session undo entry. Pure helpers plus an injectable
// runner so the undo behaviour is unit-tested in node.

import type { Crossing, NodeDetail, Relationship } from "./api";
import type { History } from "./history";

export type NodeDeleteCheck =
    | { ok: true; relationshipCount: number }
    | { ok: false; reason: string };

// Only leaves can be deleted. The relationship count comes from the level that holds the node:
// deleting a node also removes every relationship that touches it.
export function checkNodeDelete(
    id: string,
    childCount: number,
    parentLevel: { relationships?: Pick<Relationship, "from" | "to">[]; crossings?: Pick<Crossing, "node_id">[] },
): NodeDeleteCheck {
    if (childCount > 0) {
        return { ok: false, reason: "Only nodes without children can be deleted. Delete or ungroup its children first." };
    }
    const edges = (parentLevel.relationships ?? []).filter((rel) => rel.from === id || rel.to === id).length;
    const crossings = (parentLevel.crossings ?? []).filter((crossing) => crossing.node_id === id).length;
    return { ok: true, relationshipCount: edges + crossings };
}

export function nodeDeleteMessage(name: string, relationshipCount: number): string {
    if (relationshipCount === 0) return `Delete "${name}"? You can undo this.`;
    const noun = relationshipCount === 1 ? "connection" : "connections";
    return `Delete "${name}" and its ${relationshipCount} ${noun}? This cannot be undone.`;
}

export type NodeCreateInput = {
    id?: string;
    name: string;
    type: string;
    parent_id?: string;
    relationship_id?: string;
    status: string;
    implementation: string[];
    scope: string[];
    protected: boolean;
    markdown: string;
    icon?: string;
    color?: string;
    renderer?: string;
    schema?: NodeDetail["schema"];
};

export function nodeCreateInput(node: NodeDetail): NodeCreateInput {
    return {
        id: node.id,
        name: node.name,
        type: node.type,
        ...(node.parent_id ? { parent_id: node.parent_id } : {}),
        ...(node.relationship_id ? { relationship_id: node.relationship_id } : {}),
        status: node.status,
        implementation: node.implementation,
        scope: node.scope,
        protected: node.protected,
        markdown: node.markdown,
        ...(node.icon ? { icon: node.icon } : {}),
        ...(node.color ? { color: node.color } : {}),
        ...(node.renderer ? { renderer: node.renderer } : {}),
        ...(node.schema?.length ? { schema: node.schema } : {}),
    };
}

// Deletes a node and records how to bring it back. A node that had relationships took them with
// it and undo cannot restore those, so the stack is cleared instead. Undo recreates the node
// through the create API, with its original id; redo deletes it again.
export async function deleteNodeUndoable(
    node: NodeDetail,
    relationshipCount: number,
    deps: {
        history: Pick<History, "record" | "clear">;
        deleteNode: (id: string) => Promise<void>;
        createNode: (input: NodeCreateInput) => Promise<{ id: string }>;
    },
): Promise<void> {
    await deps.deleteNode(node.id);
    if (relationshipCount > 0) {
        deps.history.clear();
        return;
    }
    const input = nodeCreateInput(node);
    deps.history.record({
        label: `delete ${node.name}`,
        undo: async () => {
            await deps.createNode(input);
        },
        redo: () => deps.deleteNode(node.id),
    });
}
