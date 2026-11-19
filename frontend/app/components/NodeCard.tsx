import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";
import clsx from "clsx";
import type { SchemaColumn } from "../api";
import type { CanvasNodeType } from "../canvas-presentation";
import {
    nodeDotClass,
    nodeGlyphClass,
    resolveIconComponent,
    resolveNodeColor,
    type ColorToken,
} from "../node-presentation";

export type CrossingMark = {
    direction: string;
    label: string;
    otherId: string;
    relationshipId?: string;
    drillable?: boolean;
};

export type ArchNodeData = {
    name: string;
    type: string;
    status: string;
    protected: boolean;
    crossings: CrossingMark[];
    icon?: string;
    color?: string;
    renderer?: string;
    schema?: SchemaColumn[];
    childCount?: number;
};

export type ArchNode = Node<ArchNodeData, CanvasNodeType>;

function HeaderGlyph({
    iconId,
    colorToken,
}: {
    iconId?: string;
    colorToken: ColorToken;
}) {
    if (iconId) {
        const Icon = resolveIconComponent(iconId);
        return (
            <span className={nodeGlyphClass(colorToken)} aria-hidden>
                <Icon className="size-3" strokeWidth={2.25} />
            </span>
        );
    }
    return <div className={nodeDotClass(colorToken)} aria-hidden />;
}

export function NodeCard({ data, selected }: NodeProps<ArchNode>) {
    const colorToken = resolveNodeColor(data.type, data.color);

    return (
        <div
            className={clsx(
                "min-w-48 rounded-lg border bg-card shadow-sm select-none",
                selected
                    ? "border-foreground border-2 shadow-xl ring-primary"
                    : "border-foreground/25",
            )}
        >
            <Handle type="target" position={Position.Left} />
            <div className="flex flex-col">
                <div className="flex items-center gap-2 border-b border-foreground/50 px-3 py-2">
                    <HeaderGlyph iconId={data.icon} colorToken={colorToken} />
                    <div className="text-xs font-semibold uppercase tracking-wide text-foreground">
                        {data.name}
                    </div>
                    {data.childCount && data.childCount > 0 ? (
                        <span
                            className="ml-auto rounded bg-foreground/10 px-1.5 py-0.5 text-[10px] font-medium tabular-nums text-foreground/80"
                            title="Direct children"
                        >
                            {data.childCount}
                        </span>
                    ) : null}
                    {data.protected ? (
                        <span
                            className={clsx(
                                "text-[10px] uppercase tracking-wide text-foreground/70",
                                data.childCount && data.childCount > 0 ? "" : "ml-auto",
                            )}
                        >
                            protected
                        </span>
                    ) : null}
                </div>
                <div className="flex items-center justify-between gap-3 px-3 py-2 text-xs text-foreground/80">
                    <span>{data.type}</span>
                    <span>{data.status}</span>
                </div>
                {data.crossings.length > 0 ? (
                    <ul className="border-t border-foreground/20 px-3 py-1.5 text-[10px] text-foreground/70">
                        {data.crossings.map((crossing) => (
                            <li key={`${crossing.direction}-${crossing.otherId}-${crossing.label}`}>
                                {crossing.drillable && crossing.relationshipId ? (
                                    <a
                                        href={`/relationship/${crossing.relationshipId}`}
                                        className="underline"
                                        onClick={(e) => {
                                            e.preventDefault();
                                            e.stopPropagation();
                                            window.location.assign(`/relationship/${crossing.relationshipId}`);
                                        }}
                                    >
                                        {crossing.direction === "in" ? "←" : "→"} {crossing.label || "relationship"}
                                    </a>
                                ) : (
                                    <>
                                        {crossing.direction === "in" ? "←" : "→"} {crossing.label || "relationship"}{" "}
                                        {crossing.otherId}
                                    </>
                                )}
                            </li>
                        ))}
                    </ul>
                ) : null}
            </div>
            <Handle type="source" position={Position.Right} />
        </div>
    );
}

export default NodeCard;
