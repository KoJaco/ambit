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

export type PlacedNode = { id: string; parentId?: string; position: Point };

const seedGap = 40;

// Positions for a level that changed only because groups opened or closed. Nodes already on screen
// under the same parent keep their position (top-level absolute, members relative, as React Flow
// holds them), so nothing the architect arranged moves. Only newly revealed nodes are placed: from
// `hints` (e.g. a saved layout) when every new sibling has one, else in a grid inside their group
// (or to the right of the existing top-level nodes). A collapsed group is itself a kept node, so it
// keeps its slot.
export function seedPositions(
    nodes: CompoundNode[],
    placed: PlacedNode[],
    sizes: Record<string, NodeSize>,
    hints: Record<string, Point> = {},
): Record<string, Point> {
    const known = new Set(nodes.map((node) => node.id));
    const parentOf = (node: CompoundNode) => (node.parentId && known.has(node.parentId) ? node.parentId : "");
    const before = new Map(placed.map((item) => [item.id, item]));
    const positions: Record<string, Point> = {};
    const children = new Map<string, CompoundNode[]>();
    for (const node of nodes) {
        const parent = parentOf(node);
        children.set(parent, [...(children.get(parent) ?? []), node]);
        const old = before.get(node.id);
        if (old && (old.parentId && known.has(old.parentId) ? old.parentId : "") === parent) {
            positions[node.id] = { ...old.position };
        }
    }
    const done = new Set(Object.keys(positions));
    const sizeOf = (id: string): NodeSize => {
        const kids = children.get(id);
        if (!kids) return sizes[id] ?? { width: nodeWidth, height: nodeHeight };
        const inner: Record<string, NodeSize> = {};
        for (const kid of kids) inner[kid.id] = sizeOf(kid.id);
        return containerSize(
            kids.map((kid) => kid.id),
            positions,
            inner,
        );
    };
    const place = (parent: string) => {
        const kids = children.get(parent) ?? [];
        // Place inner groups first so their size is known when their own cell is laid out.
        for (const kid of kids) if (children.has(kid.id)) place(kid.id);
        const fresh = kids.filter((kid) => !done.has(kid.id));
        if (fresh.length === 0) return;
        if (fresh.every((kid) => hints[kid.id])) {
            for (const kid of fresh) positions[kid.id] = { ...hints[kid.id] };
        } else if (parent === "") {
            let right = 0;
            for (const kid of kids) {
                if (!done.has(kid.id)) continue;
                right = Math.max(right, positions[kid.id].x + sizeOf(kid.id).width);
            }
            let y = 0;
            for (const kid of fresh) {
                positions[kid.id] = { x: right > 0 ? right + seedGap : 0, y };
                y += sizeOf(kid.id).height + seedGap;
            }
        } else {
            const columns = Math.max(1, Math.ceil(Math.sqrt(fresh.length)));
            let x = containerPadding.left;
            let y = containerPadding.top;
            let rowHeight = 0;
            fresh.forEach((kid, index) => {
                if (index > 0 && index % columns === 0) {
                    x = containerPadding.left;
                    y += rowHeight + seedGap;
                    rowHeight = 0;
                }
                const size = sizeOf(kid.id);
                positions[kid.id] = { x, y };
                x += size.width + seedGap;
                rowHeight = Math.max(rowHeight, size.height);
            });
        }
        for (const kid of fresh) done.add(kid.id);
    };
    place("");
    return positions;
}

// After a kept node grows in place (collapsed card -> container, or a container that gained
// members), later siblings it now overlaps are pushed right or down, just far enough to clear it
// by `seedGap`, in the direction they already lay from the node's old box. Pushed nodes push the
// siblings they in turn run into, so relative order is kept. The growing node never moves, and only
// nodes sharing a parent interact. `oldSizes` holds sizes of nodes that were on screen; a node with
// no old size cannot be a grower. Positions are returned as a new map.
export function pushOverlaps(
    nodes: CompoundNode[],
    positions: Record<string, Point>,
    oldSizes: Record<string, NodeSize>,
    newSizes: Record<string, NodeSize>,
): Record<string, Point> {
    const out: Record<string, Point> = {};
    for (const [id, point] of Object.entries(positions)) out[id] = { ...point };
    const known = new Set(nodes.map((node) => node.id));
    const byParent = new Map<string, string[]>();
    for (const node of nodes) {
        const parent = node.parentId && known.has(node.parentId) ? node.parentId : "";
        byParent.set(parent, [...(byParent.get(parent) ?? []), node.id]);
    }
    const eps = 0.5;
    const sizeOf = (id: string) => newSizes[id] ?? { width: nodeWidth, height: nodeHeight };
    for (const siblings of byParent.values()) {
        const growers = siblings.filter((id) => {
            const old = oldSizes[id];
            const now = newSizes[id];
            return !!old && !!now && !!out[id] && (now.width > old.width + eps || now.height > old.height + eps);
        });
        if (growers.length === 0) continue;
        const isGrower = new Set(growers);
        // `origin` is the box a node's neighbours were laid out against; pushes only go away from it.
        const push = (origin: { x: number; y: number; width: number; height: number }, source: string, budget: number) => {
            if (budget <= 0) return;
            const box = { x: out[source].x, y: out[source].y, ...sizeOf(source) };
            for (const id of siblings) {
                if (id === source || isGrower.has(id) || !out[id]) continue;
                const at = out[id];
                const size = sizeOf(id);
                const overlapX = at.x < box.x + box.width + seedGap && at.x + size.width + seedGap > box.x;
                const overlapY = at.y < box.y + box.height + seedGap && at.y + size.height + seedGap > box.y;
                if (!overlapX || !overlapY) continue;
                const right = at.x >= origin.x + origin.width - eps;
                const below = at.y >= origin.y + origin.height - eps;
                const needX = box.x + box.width + seedGap - at.x;
                const needY = box.y + box.height + seedGap - at.y;
                if (right && (!below || needX <= needY)) {
                    const from = { x: at.x, y: at.y, ...size };
                    out[id] = { x: at.x + needX, y: at.y };
                    push(from, id, budget - 1);
                } else if (below) {
                    const from = { x: at.x, y: at.y, ...size };
                    out[id] = { x: at.x, y: at.y + needY };
                    push(from, id, budget - 1);
                }
            }
        };
        for (const id of growers) {
            const old = oldSizes[id];
            push({ x: out[id].x, y: out[id].y, ...old }, id, siblings.length);
        }
    }
    return out;
}

// Per-id larger of two size maps: a group grows to fit but is never pushed below a size it had.
export function maxSizes(
    left: Record<string, NodeSize>,
    right: Record<string, NodeSize>,
): Record<string, NodeSize> {
    const result: Record<string, NodeSize> = { ...left };
    for (const [id, size] of Object.entries(right)) {
        const other = result[id];
        result[id] = other
            ? { width: Math.max(other.width, size.width), height: Math.max(other.height, size.height) }
            : size;
    }
    return result;
}
