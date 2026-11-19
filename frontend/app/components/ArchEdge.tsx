import { type KeyboardEvent, type MouseEvent } from "react";
import { BaseEdge, EdgeLabelRenderer, getBezierPath, type EdgeProps } from "@xyflow/react";
import { useNavigate } from "react-router";
import { relationshipDrillId, type Relationship } from "../api";
import { AggregatedPill } from "./flow/aggregated-members";
import { edgePillClass, edgePillDrillableClass } from "./flow/edge-shared";

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
    const drillId = rel ? relationshipDrillId(rel) : undefined;
    const drillable = Boolean(drillId);
    const multi = Boolean(rel?.aggregated && !rel.drillable && (rel.members?.length ?? 0) > 0);

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
        if (!drillId) return;
        event.stopPropagation();
        navigate(`/relationship/${drillId}`, { state: { fromNodeId: edgeData?.fromNodeId } });
    }

    return (
        <>
            <BaseEdge id={id} path={edgePath} markerEnd={markerEnd} interactionWidth={24} />
            {label ? (
                <EdgeLabelRenderer>
                    <div
                        className={`absolute max-w-[11rem] ${
                            drillable || multi ? "pointer-events-auto cursor-pointer" : "pointer-events-none"
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
                                          if (drillId) {
                                              navigate(`/relationship/${drillId}`, {
                                                  state: { fromNodeId: edgeData?.fromNodeId },
                                              });
                                          }
                                      }
                                  }
                                : undefined
                        }
                    >
                        {multi && rel ? (
                            <AggregatedPill rel={rel} label={label} fromNodeId={edgeData?.fromNodeId} />
                        ) : (
                            <span
                                title={rel?.aggregated && rel.kinds?.length ? `Kinds: ${rel.kinds.join(", ")}` : undefined}
                                className={`block max-w-[11rem] break-words text-center leading-normal ${edgePillClass} ${
                                    drillable ? edgePillDrillableClass : ""
                                }`}
                            >
                                {label}
                            </span>
                        )}
                    </div>
                </EdgeLabelRenderer>
            ) : null}
        </>
    );
}
