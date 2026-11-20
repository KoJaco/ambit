import { BaseEdge, EdgeLabelRenderer, type EdgeProps } from "@xyflow/react";
import { edgeLabelStyle, useEdgeGeometry, edgePillClass } from "./edge-shared";

export function DataEdge(props: EdgeProps) {
    const { data, edgePath, labelX, labelY } = useEdgeGeometry(props);
    const rel = data?.rel;
    const label = rel?.label || "data";
    const fromName = data?.fromName || rel?.from || "";
    const toName = data?.toName || rel?.to || "";
    return (
        <>
            <BaseEdge id={props.id} path={edgePath} markerEnd={props.markerEnd} interactionWidth={24} />
            <EdgeLabelRenderer>
                <div
                    className={`nodrag nopan pointer-events-auto absolute max-w-[12rem] text-center leading-snug ${edgePillClass}`}
                    style={edgeLabelStyle(labelX, labelY)}
                >
                    <div className="font-medium text-foreground">{label}</div>
                    <div className="text-[10px] font-normal">
                        {fromName} → {toName}
                    </div>
                </div>
            </EdgeLabelRenderer>
        </>
    );
}
