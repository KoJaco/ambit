import { BaseEdge, EdgeLabelRenderer, type EdgeProps } from "@xyflow/react";
import { edgeGeometry } from "./edge-shared";

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
                    className="pointer-events-none absolute max-w-[12rem] rounded border border-foreground/20 bg-card px-1.5 py-1 text-center text-[10px] leading-snug shadow-sm"
                    style={{ transform: `translate(-50%, -50%) translate(${labelX}px,${labelY}px)` }}
                >
                    <div className="font-medium">{label}</div>
                    <div className="text-foreground/60">
                        {fromName} → {toName}
                    </div>
                </div>
            </EdgeLabelRenderer>
        </>
    );
}
