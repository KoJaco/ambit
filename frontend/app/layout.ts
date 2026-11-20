import ELK from "elkjs/lib/elk.bundled.js";
import type { ElkNode } from "elkjs/lib/elk-api";

const elk = new ELK();

const nodeWidth = 220;
const nodeHeight = 96;

export type Point = { x: number; y: number };
export type NodeSize = { width: number; height: number };

// Browser-only. The Go server stores these positions and does not compute them.
export async function layoutLevel(
    ids: string[],
    edges: { from: string; to: string }[],
    sizes?: Record<string, NodeSize>,
): Promise<Record<string, Point>> {
    const graph: ElkNode = {
        id: "level",
        layoutOptions: {
            "elk.algorithm": "layered",
            "elk.direction": "RIGHT",
            "elk.spacing.nodeNode": "40",
            "elk.layered.spacing.nodeNodeBetweenLayers": "80",
        },
        children: ids.map((id) => ({
            id,
            width: sizes?.[id]?.width ?? nodeWidth,
            height: sizes?.[id]?.height ?? nodeHeight,
        })),
        edges: edges.map((edge, index) => ({
            id: `e${index}`,
            sources: [edge.from],
            targets: [edge.to],
        })),
    };
    const laid = await elk.layout(graph);
    const positions: Record<string, Point> = {};
    for (const child of laid.children ?? []) {
        positions[child.id] = { x: child.x ?? 0, y: child.y ?? 0 };
    }
    for (const id of ids) {
        if (!positions[id]) positions[id] = { x: 0, y: 0 };
    }
    return positions;
}

export const containerPadding = { top: 48, left: 16, bottom: 16, right: 16 };

export type CompoundNode = { id: string; parentId?: string };

export type CompoundLayout = {
    // Top-level nodes are absolute; members are RELATIVE to their parent (React Flow subflows).
    positions: Record<string, Point>;
    // Sizes of every node that has members.
    groupSizes: Record<string, NodeSize>;
};

// Size of a container that must hold its members at the given relative positions.
export function containerSize(
    members: string[],
    positions: Record<string, Point>,
    sizes: Record<string, NodeSize>,
): NodeSize {
    let right = 0;
    let bottom = 0;
    for (const id of members) {
        const at = positions[id] ?? { x: 0, y: 0 };
        const size = sizes[id] ?? { width: nodeWidth, height: nodeHeight };
        right = Math.max(right, at.x + size.width);
        bottom = Math.max(bottom, at.y + size.height);
    }
    return {
        width: Math.max(right + containerPadding.right, containerPadding.left + containerPadding.right + nodeWidth),
        height: Math.max(bottom + containerPadding.bottom, containerPadding.top + containerPadding.bottom + nodeHeight),
    };
}

// Compound layout for a level with open groups (decision 0010). Groups are ELK parents with
// header padding, members are nested children. `sizes` gives leaf sizes; group sizes come from ELK.
// Edges are already anchored to visible nodes by the server. An edge between a group and one of its
// own descendants cannot be routed by ELK, so it is left out of the layout (it still renders).
export async function layoutCompound(
    nodes: CompoundNode[],
    edges: { from: string; to: string }[],
    sizes?: Record<string, NodeSize>,
): Promise<CompoundLayout> {
    const known = new Set(nodes.map((node) => node.id));
    const parentOf = new Map<string, string>();
    const members = new Map<string, string[]>();
    for (const node of nodes) {
        if (node.parentId && known.has(node.parentId)) {
            parentOf.set(node.id, node.parentId);
            members.set(node.parentId, [...(members.get(node.parentId) ?? []), node.id]);
        }
    }
    const isAncestor = (ancestor: string, id: string): boolean => {
        for (let at = parentOf.get(id); at; at = parentOf.get(at)) {
            if (at === ancestor) return true;
        }
        return false;
    };
    const build = (id: string): ElkNode => {
        const kids = members.get(id);
        if (!kids) {
            return { id, width: sizes?.[id]?.width ?? nodeWidth, height: sizes?.[id]?.height ?? nodeHeight };
        }
        return {
            id,
            layoutOptions: {
                "elk.padding": `[top=${containerPadding.top},left=${containerPadding.left},bottom=${containerPadding.bottom},right=${containerPadding.right}]`,
            },
            children: kids.map(build),
        };
    };
    const graph: ElkNode = {
        id: "level",
        layoutOptions: {
            "elk.algorithm": "layered",
            "elk.direction": "RIGHT",
            "elk.hierarchyHandling": "INCLUDE_CHILDREN",
            "elk.spacing.nodeNode": "40",
            "elk.layered.spacing.nodeNodeBetweenLayers": "80",
        },
        children: nodes.filter((node) => !parentOf.has(node.id)).map((node) => build(node.id)),
        edges: edges
            .filter(
                (edge) =>
                    known.has(edge.from) &&
                    known.has(edge.to) &&
                    !isAncestor(edge.from, edge.to) &&
                    !isAncestor(edge.to, edge.from),
            )
            .map((edge, index) => ({ id: `e${index}`, sources: [edge.from], targets: [edge.to] })),
    };
    const laid = await elk.layout(graph);
    const positions: Record<string, Point> = {};
    const groupSizes: Record<string, NodeSize> = {};
    const walk = (node: ElkNode) => {
        for (const child of node.children ?? []) {
            positions[child.id] = { x: child.x ?? 0, y: child.y ?? 0 };
            if (members.has(child.id)) {
                groupSizes[child.id] = { width: child.width ?? nodeWidth, height: child.height ?? nodeHeight };
            }
            walk(child);
        }
    };
    walk(laid);
    for (const node of nodes) {
        if (!positions[node.id]) positions[node.id] = { x: 0, y: 0 };
    }
    return { positions, groupSizes };
}

// Container sizes from member positions, bottom-up, for restoring a cached layout (the cache stores
// only x/y). `sizes` holds leaf sizes; containers are sized to wrap their members.
export function computeGroupSizes(
    nodes: CompoundNode[],
    positions: Record<string, Point>,
    sizes: Record<string, NodeSize>,
    // Sizes the architect set by resizing. A container never goes below what its members need,
    // and a nested container's override counts toward its parent's minimum.
    overrides: Record<string, NodeSize> = {},
): Record<string, NodeSize> {
    const members = new Map<string, string[]>();
    const known = new Set(nodes.map((node) => node.id));
    for (const node of nodes) {
        if (node.parentId && known.has(node.parentId)) {
            members.set(node.parentId, [...(members.get(node.parentId) ?? []), node.id]);
        }
    }
    const result: Record<string, NodeSize> = {};
    const size = (id: string): NodeSize => {
        const kids = members.get(id);
        if (!kids) return sizes[id] ?? { width: nodeWidth, height: nodeHeight };
        if (!result[id]) {
            const inner: Record<string, NodeSize> = {};
            for (const kid of kids) inner[kid] = size(kid);
            const min = containerSize(kids, positions, inner);
            const set = overrides[id];
            result[id] = set
                ? { width: Math.max(min.width, set.width), height: Math.max(min.height, set.height) }
                : min;
        }
        return result[id];
    };
    for (const id of members.keys()) size(id);
    return result;
}
