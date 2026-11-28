import { useMemo, type CSSProperties } from "react";
import { getSmoothStepPath, useStore, type EdgeProps, type ReactFlowState } from "@xyflow/react";
import { routeEdges, type EdgeRoute, type Rect } from "../../edge-routing";
import { estimatePillSize, layoutEdgeLabels } from "../../label-layout";
import type { Relationship } from "../../api";

// Selection and keyboard focus cue that does not rely on colour: a dashed halo around the edge
// (shown by app.css on `.selected` and `:focus-visible`), next to the thicker stroke.
export function EdgeHalo({ path }: { path: string }) {
    return <path d={path} className="edge-halo" fill="none" aria-hidden />;
}

export type FlowEdgeData = {
    drillable?: boolean;
    rel?: Relationship;
    fromNodeId?: string;
    fromName?: string;
    toName?: string;
    labelIndex?: number;
    labelCount?: number;
};

// Pills are compact and single-line: label only, ellipsis past 11rem. Detail goes in the title.
export const edgePillClass =
    "max-w-[11rem] truncate rounded-full border border-border bg-card px-2.5 py-0.5 text-[11px] leading-normal font-medium text-muted-foreground";
export const edgePillDrillableClass = "hover:border-primary/50 hover:text-primary";

// Label layer sits before the node layer in the viewport and has no stacking context of its own, so
// pills need an explicit z-index to render above nodes (including subflow children).
// Pills sit above nodes. A pill whose popover is open is raised above every other pill:
// the popover lives inside the pill's own stacking context, so its z-index alone cannot
// beat a neighbouring pill that comes later in the DOM.
export const LABEL_Z = 10000;
export const LABEL_Z_RAISED = 20000;
export const edgeLabelStyle = (x: number, y: number, raised = false): CSSProperties => ({
    transform: `translate(-50%, -50%) translate(${x}px,${y}px)`,
    zIndex: raised ? LABEL_Z_RAISED : LABEL_Z,
});

// Hover text for a pill: the full label plus the from -> to detail kept off the canvas.
export function edgeTitle(label: string, fromName?: string, toName?: string, extra?: string): string {
    const parts = [label];
    if (fromName && toName) parts.push(`${fromName} → ${toName}`);
    if (extra) parts.push(extra);
    return parts.join("\n");
}

// Extra width of the small kind glyph some pills carry, so the layout estimate matches.
export const dataPillGlyphWidth = 14;

function pillSizeFor(edge: { type?: string; data?: unknown }): { w: number; h: number } | null {
    const label = (edge.data as FlowEdgeData | undefined)?.rel?.label ?? "";
    switch (edge.type) {
        case "data":
            return estimatePillSize(label || "data", dataPillGlyphWidth);
        case "button":
            return estimatePillSize(label || "relationship");
        case "arch":
            return label ? estimatePillSize(label) : null;
        default:
            return null;
    }
}

// One layout for every edge on the canvas: routes first, then pill collision avoidance. Each edge
// component asks for its own slice, so the work is cached on a cheap signature of the inputs and
// done once per change instead of once per edge.
let layoutCache: { key: string; value: Map<string, string> } | null = null;

function layoutAll(s: ReactFlowState): Map<string, string> {
    const rects = new Map<string, Rect>();
    // Node bodies the pills must stay off. Expanded group frames are backgrounds, not obstacles.
    const obstacles: Rect[] = [];
    for (const [nodeId, node] of s.nodeLookup) {
        const w = node.measured.width ?? node.width;
        const h = node.measured.height ?? node.height;
        if (w == null || h == null || node.hidden) continue;
        const rect = { x: node.internals.positionAbsolute.x, y: node.internals.positionAbsolute.y, w, h };
        rects.set(nodeId, rect);
        if (node.type !== "container") obstacles.push(rect);
    }
    const edges = s.edges.map((e) => ({ id: e.id, source: e.source, target: e.target }));
    const sizes = new Map<string, { w: number; h: number }>();
    for (const e of s.edges) {
        const size = pillSizeFor(e);
        if (size) sizes.set(e.id, size);
    }
    let key = "";
    for (const [id, r] of rects) key += `${id}:${r.x},${r.y},${r.w},${r.h};`;
    key += "|";
    for (const r of obstacles) key += `${r.x},${r.y},${r.w},${r.h};`;
    key += "|";
    for (const e of edges) key += `${e.id}:${e.source}>${e.target}:${sizes.get(e.id)?.w ?? ""};`;
    if (layoutCache?.key === key) return layoutCache.value;
    const routes = routeEdges(edges, rects);
    const labels = layoutEdgeLabels(routes, sizes, obstacles);
    const value = new Map<string, string>();
    for (const [id, route] of routes) {
        const p = labels.get(id) ?? { x: route.labelX, y: route.labelY };
        value.set(id, JSON.stringify({ ...route, labelX: p.x, labelY: p.y }));
    }
    layoutCache = { key, value };
    return value;
}

export function useEdgeGeometry(props: EdgeProps) {
    const data = props.data as FlowEdgeData | undefined;
    const { id } = props;
    // The selector returns a serialized route so React only re-renders when the geometry changes.
    const serialized = useStore((s: ReactFlowState) => layoutAll(s).get(id) ?? "");
    const route = useMemo(() => (serialized ? (JSON.parse(serialized) as EdgeRoute) : null), [serialized]);
    let edgePath: string;
    let labelX: number;
    let labelY: number;
    if (route) {
        ({ path: edgePath, labelX, labelY } = route);
    } else {
        [edgePath, labelX, labelY] = getSmoothStepPath({
            sourceX: props.sourceX,
            sourcePosition: props.sourcePosition,
            sourceY: props.sourceY,
            targetX: props.targetX,
            targetY: props.targetY,
            targetPosition: props.targetPosition,
            borderRadius: 12,
        });
    }
    return { data, edgePath, labelX, labelY };
}
