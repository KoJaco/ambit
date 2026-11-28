import { BaseEdge, EdgeLabelRenderer, type EdgeProps } from "@xyflow/react";
import { KindGlyphIcon } from "./kind-glyph";
import { edgeLabelStyle, edgeTitle, useEdgeGeometry, edgePillClass, EdgeHalo } from "./edge-shared";

// Compact single-line pill: glyph + label. The from -> to detail lives in the tooltip and the panel.
export function DataEdge(props: EdgeProps) {
    const { data, edgePath, labelX, labelY } = useEdgeGeometry(props);
    const rel = data?.rel;
    const label = rel?.label || "data";
    const fromName = data?.fromName || rel?.from || "";
    const toName = data?.toName || rel?.to || "";
    return (
        <>
            <EdgeHalo path={edgePath} />
            <BaseEdge id={props.id} path={edgePath} markerEnd={props.markerEnd} interactionWidth={24} />
            <EdgeLabelRenderer>
                <div
                    className="nodrag nopan pointer-events-auto absolute max-w-[11rem]"
                    style={edgeLabelStyle(labelX, labelY)}
                >
                    <span
                        title={edgeTitle(label, fromName, toName)}
                        className={`flex items-center gap-1 text-center ${edgePillClass}`}
                    >
                        <KindGlyphIcon glyph="data" className="size-3 shrink-0" />
                        <span className="min-w-0 truncate text-foreground">{label}</span>
                    </span>
                </div>
            </EdgeLabelRenderer>
        </>
    );
}
