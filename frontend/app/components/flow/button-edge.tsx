import { type KeyboardEvent, type MouseEvent } from "react";
import { BaseEdge, EdgeLabelRenderer, type EdgeProps } from "@xyflow/react";
import { useNavigate } from "react-router";
import clsx from "clsx";
import { edgeGeometry, edgePillClass, edgePillDrillableClass } from "./edge-shared";

export function ButtonEdge(props: EdgeProps) {
    const navigate = useNavigate();
    const { data, edgePath, labelX, labelY } = edgeGeometry(props);
    const rel = data?.rel;
    const label = rel?.label || "relationship";
    const drillable = Boolean(rel?.drillable);

    function openRelationship(event: MouseEvent | KeyboardEvent) {
        if (!drillable || !rel?.id) return;
        event.stopPropagation();
        navigate(`/relationship/${rel.id}`, { state: { fromNodeId: data?.fromNodeId } });
    }

    return (
        <>
            <BaseEdge id={props.id} path={edgePath} markerEnd={props.markerEnd} interactionWidth={24} />
            <EdgeLabelRenderer>
                <button
                    type="button"
                    className={clsx("nodrag nopan pointer-events-auto absolute", edgePillClass, drillable && edgePillDrillableClass)}
                    style={{ transform: `translate(-50%, -50%) translate(${labelX}px,${labelY}px)` }}
                    onClick={openRelationship}
                    onDoubleClick={openRelationship}
                >
                    {label}
                </button>
            </EdgeLabelRenderer>
        </>
    );
}
