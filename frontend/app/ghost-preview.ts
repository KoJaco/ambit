import type { OpDiff, ProposalDiff } from "./api";
import { containerPadding } from "./layout";
import type { NodeSize, Point } from "./layout";

// Canvas ghost preview: the pending proposal operations that belong on the level being shown,
// as pure data. Nothing here touches the model, layout files or the group-size store; the canvas
// draws these on top of the real level and drops them when the preview is off.

export type GhostKey = string; // `${proposalId}:${op index}`

export function ghostKey(proposalId: string, index: number): GhostKey {
    return `${proposalId}:${index}`;
}

export type GhostNode = {
    id: string;
    // The id the node will have once accepted; endpoints and children refer to it.
    realId: string;
    name: string;
    type: string;
    icon?: string;
    color?: string;
    // Containing ghost (a new zone from the same preview); undefined means the level itself.
    parentGhostId?: string;
    hasChildren: boolean;
    key: GhostKey;
    proposalId: string;
    index: number;
};

export type GhostEdge = {
    id: string;
    source: string;
    target: string;
    label: string;
    kind: string;
    key: GhostKey;
    proposalId: string;
    index: number;
};

export type MarkKind = "removed" | "changed";

export type NodeMark = { nodeId: string; kind: MarkKind; key: GhostKey };
export type EdgeMark = { edgeId: string; kind: MarkKind; label: string; key: GhostKey; proposalId: string; index: number };

export type GhostPreview = {
    ghostNodes: GhostNode[];
    ghostEdges: GhostEdge[];
    nodeMarks: Record<string, NodeMark>;
    edgeMarks: Record<string, EdgeMark>;
};

export const emptyGhostPreview: GhostPreview = { ghostNodes: [], ghostEdges: [], nodeMarks: {}, edgeMarks: {} };

// What the host gives the canvas: the proposals to draw and how to talk back to the Review panel.
export type CanvasPreview = {
    diffs: ProposalDiff[];
    highlightKey: GhostKey | null;
    onActivate: (proposalId: string, index: number) => void;
    onHover: (key: GhostKey | null) => void;
};

export type GhostInput = {
    // Proposals whose preview is on; only their pending operations are used.
    diffs: ProposalDiff[];
    // The node whose level is shown; null at the root.
    parentId: string | null;
    // Real node and relationship ids drawn on this level.
    nodeIds: string[];
    edgeIds: string[];
};

function previewable(op: OpDiff): boolean {
    return op.status === "pending" && !op.error;
}

export function buildGhostPreview({ diffs, parentId, nodeIds, edgeIds }: GhostInput): GhostPreview {
    const realNodes = new Set(nodeIds);
    const realEdges = new Set(edgeIds);
    const ops: { diff: ProposalDiff; op: OpDiff }[] = [];
    for (const diff of diffs) {
        for (const op of diff.operations) if (previewable(op)) ops.push({ diff, op });
    }

    // New nodes: shown when their parent is this level or a new node that is itself shown.
    const level = parentId ?? "";
    const creates = ops.filter(({ op }) => op.op === "create_node" && op.proposed && !op.proposed.relationship_id);
    const shown = new Map<string, GhostNode>(); // by realId
    for (let changed = true; changed; ) {
        changed = false;
        for (const { diff, op } of creates) {
            const proposed = op.proposed!;
            const realId = op.node_id || proposed.id;
            if (shown.has(realId) || realNodes.has(realId)) continue;
            const parent = proposed.parent_id ?? "";
            const parentGhost = parent === level ? undefined : shown.get(parent);
            if (parent !== level && !parentGhost) continue;
            shown.set(realId, {
                id: `ghost:${ghostKey(diff.proposal_id, op.index)}`,
                realId,
                name: proposed.name || realId,
                type: proposed.type,
                icon: proposed.icon,
                color: proposed.color,
                parentGhostId: parentGhost?.id,
                hasChildren: false,
                key: ghostKey(diff.proposal_id, op.index),
                proposalId: diff.proposal_id,
                index: op.index,
            });
            changed = true;
        }
    }
    // Parents first, as React Flow requires; discovery order is kept within a depth.
    const depth = (node: GhostNode): number => (node.parentGhostId ? 1 + depth(byId.get(node.parentGhostId)!) : 0);
    const byId = new Map([...shown.values()].map((node) => [node.id, node]));
    const ghostNodes = [...shown.values()].sort((a, b) => depth(a) - depth(b));
    const parents = new Set(ghostNodes.map((node) => node.parentGhostId).filter(Boolean));
    for (const node of ghostNodes) node.hasChildren = parents.has(node.id);

    const resolve = (id: string): string | undefined => (realNodes.has(id) ? id : shown.get(id)?.id);
    const ghostEdges: GhostEdge[] = [];
    const nodeMarks: Record<string, NodeMark> = {};
    const edgeMarks: Record<string, EdgeMark> = {};
    const markNode = (nodeId: string, kind: MarkKind, key: GhostKey) => {
        if (!realNodes.has(nodeId)) return;
        const existing = nodeMarks[nodeId];
        if (existing && (existing.kind === "removed" || kind === "changed")) return;
        nodeMarks[nodeId] = { nodeId, kind, key };
    };

    for (const { diff, op } of ops) {
        const key = ghostKey(diff.proposal_id, op.index);
        switch (op.op) {
            case "update_node":
                markNode(op.node_id, "changed", key);
                break;
            case "delete_node":
                markNode(op.node_id, "removed", key);
                break;
            case "set_relationship": {
                const rel = op.relationship;
                if (!rel) break;
                const existing = op.current_relationship?.id;
                if (op.current_relationship) {
                    if (existing && realEdges.has(existing)) {
                        edgeMarks[existing] = { edgeId: existing, kind: "changed", label: rel.label, key, proposalId: diff.proposal_id, index: op.index };
                    }
                    break;
                }
                const source = resolve(rel.from);
                const target = resolve(rel.to);
                if (!source || !target) break;
                ghostEdges.push({
                    id: `ghost-edge:${key}`,
                    source,
                    target,
                    label: rel.label,
                    kind: rel.kind,
                    key,
                    proposalId: diff.proposal_id,
                    index: op.index,
                });
                break;
            }
            case "delete_relationship": {
                const id = (op.current_relationship ?? op.relationship)?.id;
                if (id && realEdges.has(id)) {
                    const label = (op.current_relationship ?? op.relationship)?.label ?? "";
                    edgeMarks[id] = { edgeId: id, kind: "removed", label, key, proposalId: diff.proposal_id, index: op.index };
                }
                for (const removed of op.removes_nodes ?? []) markNode(removed.id, "removed", key);
                break;
            }
        }
    }
    return { ghostNodes, ghostEdges, nodeMarks, edgeMarks };
}

export type Rect = { x: number; y: number; width: number; height: number };

export const ghostLeafSize: NodeSize = { width: 220, height: 96 };
const gap = 40;
const margin = 24;
const scanStep = 24;

export type GhostLayout = {
    // Top-level ghosts are absolute; ghosts inside a ghost container are relative to it.
    positions: Record<string, Point>;
    sizes: Record<string, NodeSize>;
};

// Deterministic placement that never moves a real node. Each top-level ghost starts beside the
// real node it connects to (else to the right of everything) and slides down, then right, until
// it clears every real node and every ghost already placed. `obstacles` are the real top-level
// nodes in absolute coordinates.
export function placeGhosts(
    ghosts: GhostNode[],
    edges: GhostEdge[],
    obstacles: Record<string, Rect>,
): GhostLayout {
    const sizes: Record<string, NodeSize> = {};
    const positions: Record<string, Point> = {};
    const kids = new Map<string, GhostNode[]>();
    for (const ghost of ghosts) {
        const parent = ghost.parentGhostId ?? "";
        kids.set(parent, [...(kids.get(parent) ?? []), ghost]);
    }
    // Inner ghosts are flowed in a grid inside their container; sizes are computed bottom-up.
    const flow = (ghost: GhostNode): NodeSize => {
        const members = kids.get(ghost.id);
        if (!members) return (sizes[ghost.id] = ghostLeafSize);
        const columns = Math.max(1, Math.ceil(Math.sqrt(members.length)));
        let x = containerPadding.left;
        let y = containerPadding.top;
        let rowHeight = 0;
        let right = 0;
        members.forEach((member, index) => {
            if (index > 0 && index % columns === 0) {
                x = containerPadding.left;
                y += rowHeight + gap;
                rowHeight = 0;
            }
            const size = flow(member);
            positions[member.id] = { x, y };
            x += size.width + gap;
            right = Math.max(right, x - gap);
            rowHeight = Math.max(rowHeight, size.height);
        });
        return (sizes[ghost.id] = {
            width: right + containerPadding.right,
            height: y + rowHeight + containerPadding.bottom,
        });
    };
    const top = kids.get("") ?? [];
    for (const ghost of top) flow(ghost);

    const topOf = (id: string): string => {
        let current = ghosts.find((ghost) => ghost.id === id);
        while (current?.parentGhostId) current = ghosts.find((ghost) => ghost.id === current!.parentGhostId);
        return current?.id ?? id;
    };
    const placed: Rect[] = Object.values(obstacles);
    const right = placed.reduce((max, rect) => Math.max(max, rect.x + rect.width), 0);
    const topEdge = placed.length ? Math.min(...placed.map((rect) => rect.y)) : 0;
    const clear = (rect: Rect) =>
        placed.every(
            (other) =>
                rect.x >= other.x + other.width + margin ||
                rect.x + rect.width + margin <= other.x ||
                rect.y >= other.y + other.height + margin ||
                rect.y + rect.height + margin <= other.y,
        );

    for (const ghost of top) {
        const size = sizes[ghost.id];
        let anchor: Rect | undefined;
        for (const edge of edges) {
            const [own, other] = topOf(edge.source) === ghost.id ? [edge.source, edge.target] : topOf(edge.target) === ghost.id ? [edge.target, edge.source] : [];
            if (own && other && obstacles[other]) {
                anchor = obstacles[other];
                break;
            }
        }
        const startX = anchor ? anchor.x + anchor.width + gap : placed.length ? right + gap : 0;
        const startY = anchor ? anchor.y : topEdge;
        let at: Point | null = null;
        for (let column = 0; column < 8 && !at; column += 1) {
            const x = startX + column * (size.width + gap);
            for (let step = 0; step < 60; step += 1) {
                const y = startY + step * scanStep;
                if (clear({ x, y, ...size })) {
                    at = { x, y };
                    break;
                }
            }
        }
        // Nothing fit nearby: stack below everything placed so far.
        at ??= { x: startX, y: placed.reduce((max, rect) => Math.max(max, rect.y + rect.height), 0) + gap };
        positions[ghost.id] = at;
        placed.push({ ...at, ...size });
    }
    return { positions, sizes };
}
