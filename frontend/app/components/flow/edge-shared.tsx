import { getBezierPath, type EdgeProps } from "@xyflow/react";
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

export function edgeGeometry(props: EdgeProps) {
    const data = props.data as FlowEdgeData | undefined;
    const [edgePath, labelX, labelY] = getBezierPath({
        sourceX: props.sourceX,
        sourceY: props.sourceY,
        sourcePosition: props.sourcePosition,
        targetX: props.targetX,
        targetY: props.targetY,
        targetPosition: props.targetPosition,
    });
    const labelIndex = data?.labelIndex ?? 0;
    const labelCount = data?.labelCount ?? 1;
    const labelOffsetY = (labelIndex - (labelCount - 1) / 2) * 36;
    return { data, edgePath, labelX, labelY: labelY + labelOffsetY };
}
