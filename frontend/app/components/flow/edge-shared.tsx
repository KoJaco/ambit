import { useMemo, type CSSProperties } from "react";
import { getSmoothStepPath, useStore, type EdgeProps, type ReactFlowState } from "@xyflow/react";
import { routeEdges, type EdgeRoute, type Rect } from "../../edge-routing";
import type { Relationship } from "../../api";

export type FlowEdgeData = {
    drillable?: boolean;
    rel?: Relationship;
    fromNodeId?: string;
    fromName?: string;
    toName?: string;
    labelIndex?: number;
    labelCount?: number;
};

export const edgePillClass =
    "rounded-full border border-border bg-card px-2.5 py-0.5 text-[11px] font-medium text-muted-foreground";
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

export function useEdgeGeometry(props: EdgeProps) {
    const data = props.data as FlowEdgeData | undefined;
    const { id, source, target } = props;
    // The selector returns a serialized route so React only re-renders when the geometry changes.
    const serialized = useStore((s: ReactFlowState) => {
        const rects = new Map<string, Rect>();
        for (const [nodeId, node] of s.nodeLookup) {
            const w = node.measured.width ?? node.width;
            const h = node.measured.height ?? node.height;
            if (w == null || h == null || node.hidden) continue;
            rects.set(nodeId, { x: node.internals.positionAbsolute.x, y: node.internals.positionAbsolute.y, w, h });
        }
        const incident = s.edges
            .filter((e) => e.source === source || e.target === source || e.source === target || e.target === target)
            .map((e) => ({ id: e.id, source: e.source, target: e.target }));
        const route = routeEdges(incident, rects).get(id);
        return route ? JSON.stringify(route) : "";
    });
    const route = useMemo(() => (serialized ? (JSON.parse(serialized) as EdgeRoute) : null), [serialized]);
    let edgePath: string;
    let labelX: number;
    let labelY: number;
    if (route) {
        ({ path: edgePath, labelX, labelY } = route);
    } else {
        [edgePath, labelX, labelY] = getSmoothStepPath({
            sourceX: props.sourceX,
            sourceY: props.sourceY,
            sourcePosition: props.sourcePosition,
            targetX: props.targetX,
            targetY: props.targetY,
            targetPosition: props.targetPosition,
            borderRadius: 12,
        });
    }
    const labelIndex = data?.labelIndex ?? 0;
    const labelCount = data?.labelCount ?? 1;
    const labelOffsetY = (labelIndex - (labelCount - 1) / 2) * 30;
    return { data, edgePath, labelX, labelY: labelY + labelOffsetY };
}
