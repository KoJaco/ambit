import { type KeyboardEvent, type MouseEvent } from "react";
import { BaseEdge, EdgeLabelRenderer, getBezierPath, type EdgeProps } from "@xyflow/react";
import { useNavigate } from "react-router";
import type { Relationship } from "../api";

type ArchEdgeData = {
    drillable?: boolean;
    rel?: Relationship;
    fromNodeId?: string;
    labelIndex?: number;
    labelCount?: number;
};

export function ArchEdge({
    id,
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
    markerEnd,
    data,
}: EdgeProps) {
    const navigate = useNavigate();
    const edgeData = data as ArchEdgeData | undefined;
    const rel = edgeData?.rel;
    const label = rel?.label ?? "";
    const drillable = Boolean(rel?.drillable);

    const [edgePath, labelX, labelY] = getBezierPath({
        sourceX,
        sourceY,
        sourcePosition,
        targetX,
        targetY,
        targetPosition,
    });

    const labelIndex = edgeData?.labelIndex ?? 0;
    const labelCount = edgeData?.labelCount ?? 1;
    const labelOffsetY = (labelIndex - (labelCount - 1) / 2) * 36;

    function openRelationship(event: MouseEvent) {
        if (!drillable || !rel?.id) return;
        event.stopPropagation();
        navigate(`/relationship/${rel.id}`, { state: { fromNodeId: edgeData?.fromNodeId } });
    }

    return (
        <>
            <BaseEdge id={id} path={edgePath} markerEnd={markerEnd} interactionWidth={24} />
            {label ? (
                <EdgeLabelRenderer>
                    <div
                        className={`absolute max-w-[11rem] ${
                            drillable ? "pointer-events-auto cursor-pointer" : "pointer-events-none"
                        }`}
                        style={{
                            transform: `translate(-50%, -50%) translate(${labelX}px,${labelY + labelOffsetY}px)`,
                        }}
                        onClick={openRelationship}
                        onDoubleClick={openRelationship}
                        role={drillable ? "link" : undefined}
                        tabIndex={drillable ? 0 : undefined}
                        onKeyDown={
                            drillable
                                ? (event: KeyboardEvent) => {
                                      if (event.key === "Enter" || event.key === " ") {
                                          event.preventDefault();
                                          if (rel?.id) {
                                              navigate(`/relationship/${rel.id}`, {
                                                  state: { fromNodeId: edgeData?.fromNodeId },
                                              });
                                          }
                                      }
                                  }
                                : undefined
                        }
                    >
                        <span
                            className={`block max-w-[11rem] break-words rounded px-1.5 py-1 text-center text-xs leading-normal bg-foreground text-background shadow-sm ring-1 ring-foreground/25 ${
                                drillable ? "hover:ring-primary/50" : ""
                            }`}
                        >
                            {label}
                        </span>
                    </div>
                </EdgeLabelRenderer>
            ) : null}
        </>
    );
}
