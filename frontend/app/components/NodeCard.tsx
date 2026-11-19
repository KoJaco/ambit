import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";
import clsx from "clsx";
import { Lock } from "lucide-react";
import type { SchemaColumn } from "../api";
import type { CanvasNodeType } from "../canvas-presentation";
import {
    nodeDotClass,
    nodeGlyphClass,
    resolveIconComponent,
    resolveNodeColor,
    statusLabel,
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
                <Icon className="size-4" strokeWidth={2} />
            </span>
        );
    }
    return <div className={nodeDotClass(colorToken)} aria-hidden />;
}

export function NodeCard({ data, selected }: NodeProps<ArchNode>) {
    const colorToken = resolveNodeColor(data.type, data.color);
    const childCount = data.childCount ?? 0;
    const stacked = childCount > 0;

    return (
        <div className="relative w-[220px] select-none">
            {stacked ? (
                <>
                    <div
                        aria-hidden
                        className="absolute inset-0 translate-x-2 translate-y-2 rounded-[16px] border border-border bg-[var(--sheet-2)]"
                    />
                    <div
                        aria-hidden
                        className="absolute inset-0 translate-x-1 translate-y-1 rounded-[16px] border border-border bg-[var(--sheet-1)]"
                    />
                </>
            ) : null}
            <div
                className={clsx(
                    "relative rounded-[14px] border bg-card transition-colors",
                    selected
                        ? "border-[1.5px] border-primary shadow-[0_0_0_4px_var(--selection-ring)]"
                        : "border-border hover:border-foreground/25",
                )}
            >
                <Handle type="target" position={Position.Left} />
                <div className="flex items-center gap-2.5 px-3 py-2.5">
                    <HeaderGlyph iconId={data.icon} colorToken={colorToken} />
                    <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-1.5">
                            <span className="truncate text-sm font-medium text-foreground" title={data.name}>
                                {data.name}
                            </span>
                            {data.protected ? (
                                <Lock
                                    className="size-3 shrink-0 text-muted-foreground"
                                    aria-label="Protected"
                                />
                            ) : null}
                        </div>
                        <div className="truncate text-xs text-muted-foreground">
                            {data.type} · {statusLabel(data.status)}
                        </div>
                    </div>
                </div>
                {data.crossings.length > 0 ? (
                    <ul className="border-t border-border px-3 py-1.5 text-[10px] text-muted-foreground">
                        {data.crossings.map((crossing) => (
                            <li key={`${crossing.direction}-${crossing.otherId}-${crossing.label}`}>
                                {crossing.drillable && crossing.relationshipId ? (
                                    <a
                                        href={`/relationship/${crossing.relationshipId}`}
                                        className="underline hover:text-primary"
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
                {stacked ? (
                    <div className="flex items-center justify-between border-t border-border px-3 py-1.5 text-xs text-muted-foreground">
                        <span className="tabular-nums" title="Direct children">
                            {childCount} inside
                        </span>
                        <span className="font-medium">Open ›</span>
                    </div>
                ) : null}
                <Handle type="source" position={Position.Right} />
            </div>
        </div>
    );
}

export default NodeCard;
