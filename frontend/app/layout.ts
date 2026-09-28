import ELK from "elkjs/lib/elk.bundled.js";
import type { ElkNode } from "elkjs/lib/elk-api";

const elk = new ELK();

const nodeWidth = 220;
const nodeHeight = 96;

export type Point = { x: number; y: number };

// Browser-only. The Go server stores these positions and does not compute them.
export async function layoutLevel(
    ids: string[],
    edges: { from: string; to: string }[],
): Promise<Record<string, Point>> {
    const graph: ElkNode = {
        id: "level",
        layoutOptions: {
            "elk.algorithm": "layered",
            "elk.direction": "RIGHT",
            "elk.spacing.nodeNode": "40",
            "elk.layered.spacing.nodeNodeBetweenLayers": "80",
        },
        children: ids.map((id) => ({ id, width: nodeWidth, height: nodeHeight })),
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
