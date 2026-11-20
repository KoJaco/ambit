import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";
import clsx from "clsx";
import { ChevronDown, ChevronRight, Lock, Ungroup } from "lucide-react";
import { Link } from "react-router";
import type { SchemaColumn, TypeMixEntry } from "../api";
import type { CanvasNodeType } from "../canvas-presentation";
import { summaryText } from "../level-view";
import { AgentChip } from "./agent-chip";
import { Tooltip } from "./tooltip";
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
    assigned?: boolean;
    crossings: CrossingMark[];
    icon?: string;
    color?: string;
    renderer?: string;
    schema?: SchemaColumn[];
    childCount?: number;
    typeMix?: TypeMixEntry[];
    internalRelCount?: number;
    expanded?: boolean;
    // Container tint level, 1 for a top-level open group, 2 for one nested inside it.
    depth?: number;
    // Smallest size a container may be resized to (its members plus padding).
    minWidth?: number;
    minHeight?: number;
    // Called once when the architect finishes resizing an open group.
    onResizeGroup?: (id: string, size: { width: number; height: number }) => void;
    // Stable callback from the canvas; toggles group/summary for one node.
    onToggleMode?: (id: string) => void;
    // Opens the ungroup form for a group (summary card or container header).
    onUngroup?: (id: string) => void;
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

export function NodeCard({ id, data, selected }: NodeProps<ArchNode>) {
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
                            {data.assigned ? <AgentChip /> : null}
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
                    <div className="flex items-center gap-2 border-t border-border px-3 py-1.5 text-xs text-muted-foreground">
                        <span className="min-w-0 flex-1 line-clamp-2 break-words tabular-nums" title="Direct children">
                            {summaryText(childCount, data.typeMix)}
                            {data.internalRelCount ? (
                                <span className="ml-1.5" title="Relationships hidden inside this node">
                                    · {data.internalRelCount} internal
                                </span>
                            ) : null}
                        </span>
                        {data.onToggleMode ? (
                            <Tooltip label="Expand">
                                <button
                                    type="button"
                                    aria-label={`Expand ${data.name}`}
                                    aria-expanded={false}
                                    className="nodrag nopan flex size-6 shrink-0 items-center justify-center rounded-md hover:bg-muted focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary"
                                    onClick={(event) => {
                                        event.stopPropagation();
                                        data.onToggleMode?.(id);
                                    }}
                                    onDoubleClick={(event) => event.stopPropagation()}
                                >
                                    <ChevronDown className="size-4" aria-hidden />
                                </button>
                            </Tooltip>
                        ) : null}
                        {data.onUngroup ? (
                            <Tooltip label="Ungroup">
                                <button
                                    type="button"
                                    aria-label={`Ungroup ${data.name}`}
                                    className="nodrag nopan flex size-6 shrink-0 items-center justify-center rounded-md hover:bg-muted focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary"
                                    onClick={(event) => {
                                        event.stopPropagation();
                                        data.onUngroup?.(id);
                                    }}
                                    onDoubleClick={(event) => event.stopPropagation()}
                                >
                                    <Ungroup className="size-4" aria-hidden />
                                </button>
                            </Tooltip>
                        ) : null}
                        <Link
                            to={`/node/${id}`}
                            aria-label={`Open ${data.name}`}
                            className="nodrag nopan flex h-6 items-center rounded-md px-1.5 font-medium hover:bg-muted hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary"
                            onClick={(event) => event.stopPropagation()}
                            onDoubleClick={(event) => event.stopPropagation()}
                        >
                            Open <ChevronRight className="size-3" aria-hidden />
                        </Link>
                    </div>
                ) : null}
                <Handle type="source" position={Position.Right} />
            </div>
        </div>
    );
}

export default NodeCard;
