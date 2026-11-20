import type { Assignment } from "./api";

type NamedNode = { id: string; name: string };

export type AssignmentState = "none" | "this" | "other" | "dangling";

export function assignmentState(assignment: Assignment | null, nodeId: string): AssignmentState {
    if (!assignment || !assignment.node_id) return "none";
    if (assignment.dangling) return "dangling";
    return assignment.node_id === nodeId ? "this" : "other";
}

// One-line status for the inspector. nodes lets the label name another node when it is known.
export function assignmentLabel(
    assignment: Assignment | null,
    nodeId: string,
    nodes: readonly NamedNode[] = [],
): string {
    switch (assignmentState(assignment, nodeId)) {
        case "none":
            return "No active assignment";
        case "this":
            return "Assigned to this node";
        case "dangling":
            return `Assigned to ${assignment?.node_id} (node no longer exists)`;
        default: {
            const id = assignment?.node_id ?? "";
            const name = nodes.find((node) => node.id === id)?.name;
            return `Assigned to ${name ?? id}`;
        }
    }
}
