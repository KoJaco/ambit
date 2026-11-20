import { BaseEdge, type EdgeProps } from "@xyflow/react";
import { useEdgeGeometry } from "./edge-shared";

export function AnimatedEdge(props: EdgeProps) {
    const { edgePath } = useEdgeGeometry(props);
    return (
        <>
            <BaseEdge id={props.id} path={edgePath} markerEnd={props.markerEnd} interactionWidth={24} />
            <circle r="3.5" className="fill-primary">
                <animateMotion dur="2.4s" repeatCount="indefinite" path={edgePath} />
            </circle>
        </>
    );
}
