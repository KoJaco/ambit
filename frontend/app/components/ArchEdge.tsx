import { useState, type KeyboardEvent, type MouseEvent } from "react";
import { BaseEdge, EdgeLabelRenderer, type EdgeProps } from "@xyflow/react";
import { useNavigate } from "react-router";
import { relationshipDrillId } from "../api";
import { AggregatedPill } from "./flow/aggregated-members";
import { edgeLabelStyle, edgePillClass, edgePillDrillableClass, edgeTitle, useEdgeGeometry, EdgeHalo } from "./flow/edge-shared";

export function ArchEdge(props: EdgeProps) {
    const { id, markerEnd } = props;
    const navigate = useNavigate();
    const [popoverOpen, setPopoverOpen] = useState(false);
    const { data: edgeData, edgePath, labelX, labelY } = useEdgeGeometry(props);
    const rel = edgeData?.rel;
    const label = rel?.label ?? "";
    const drillId = rel ? relationshipDrillId(rel) : undefined;
    const drillable = Boolean(drillId);
    const multi = Boolean(rel?.aggregated && !rel.drillable && (rel.members?.length ?? 0) > 0);

    function openRelationship(event: MouseEvent) {
        if (!drillId) return;
        event.stopPropagation();
        navigate(`/relationship/${drillId}`, { state: { fromNodeId: edgeData?.fromNodeId } });
    }

    return (
        <>
            <EdgeHalo path={edgePath} />
            <BaseEdge id={id} path={edgePath} markerEnd={markerEnd} interactionWidth={24} />
            {label ? (
                <EdgeLabelRenderer>
                    <div
                        className={`nodrag nopan pointer-events-auto absolute max-w-[11rem] ${drillable || multi ? "cursor-pointer" : ""}`}
                        style={edgeLabelStyle(labelX, labelY, popoverOpen)}
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
                            <AggregatedPill
                                rel={rel}
                                label={label}
                                fromNodeId={edgeData?.fromNodeId}
                                detail={edgeTitle(label, edgeData?.fromName, edgeData?.toName)}
                                onOpenChange={setPopoverOpen}
                            />
                        ) : (
                            <span
                                title={edgeTitle(
                                    label,
                                    edgeData?.fromName,
                                    edgeData?.toName,
                                    rel?.aggregated && rel.kinds?.length ? `Kinds: ${rel.kinds.join(", ")}` : undefined,
                                )}
                                className={`block text-center ${edgePillClass} ${
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
