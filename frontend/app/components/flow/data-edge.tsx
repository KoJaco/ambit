import { BaseEdge, EdgeLabelRenderer, type EdgeProps } from "@xyflow/react";
import { edgeGeometry, edgePillClass } from "./edge-shared";

export function DataEdge(props: EdgeProps) {
    const { data, edgePath, labelX, labelY } = edgeGeometry(props);
    const rel = data?.rel;
    const label = rel?.label || "data";
    const fromName = data?.fromName || rel?.from || "";
    const toName = data?.toName || rel?.to || "";
    return (
        <>
            <BaseEdge id={props.id} path={edgePath} markerEnd={props.markerEnd} interactionWidth={24} />
            <EdgeLabelRenderer>
                <div
                    className={`pointer-events-none absolute max-w-[12rem] text-center leading-snug ${edgePillClass}`}
                    style={{ transform: `translate(-50%, -50%) translate(${labelX}px,${labelY}px)` }}
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
