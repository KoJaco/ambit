import { Handle, Position, type NodeProps } from "@xyflow/react";
import clsx from "clsx";
import { ChevronUp, Ungroup } from "lucide-react";
import { Link } from "react-router";
import { nodeDotClass, nodeGlyphClass, resolveIconComponent, resolveNodeColor } from "../../node-presentation";
import type { ArchNode } from "../NodeCard";

export function GroupFrame({ data, selected }: NodeProps<ArchNode>) {
    return (
        <div
            className={clsx(
                "relative h-[140px] w-[280px] rounded-xl border bg-foreground/[0.03] select-none",
                selected ? "border-foreground border-2" : "border-foreground/30",
            )}
        >
            <Handle type="target" position={Position.Left} />
            <div className="absolute -top-2.5 left-3 rounded bg-card px-1.5 text-[10px] font-semibold uppercase tracking-wide">
                {data.name}
            </div>
            <Handle type="source" position={Position.Right} />
        </div>
    );
}

// An open group (decision 0010): a real React Flow parent. Members are separate nodes with
// parentId set, so this renders only the frame and header.
export function ContainerNode({ id, data, selected }: NodeProps<ArchNode>) {
    const colorToken = resolveNodeColor(data.type, data.color);
    const Icon = data.icon ? resolveIconComponent(data.icon) : null;
    return (
        <div
            className={clsx(
                "relative h-full w-full select-none rounded-[20px] bg-[var(--group)]",
                selected && "shadow-[0_0_0_4px_var(--selection-ring)]",
            )}
        >
            <Handle type="target" position={Position.Left} />
            <div className="flex h-11 items-center gap-2 px-4">
                {Icon ? (
                    <span className={nodeGlyphClass(colorToken)} aria-hidden>
                        <Icon className="size-4" strokeWidth={2} />
                    </span>
                ) : (
                    <div className={nodeDotClass(colorToken)} aria-hidden />
                )}
                <span className="min-w-0 flex-1 truncate text-sm font-semibold text-foreground" title={data.name}>
                    {data.name}
                </span>
                <Link
                    to={`/node/${id}`}
                    aria-label={`Open ${data.name}`}
                    className="nodrag nopan flex h-6 items-center rounded-md px-2 text-xs font-medium text-muted-foreground hover:bg-card hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary"
                    onClick={(event) => event.stopPropagation()}
                    onDoubleClick={(event) => event.stopPropagation()}
                >
                    Open
                </Link>
                {data.onUngroup ? (
                    <button
                        type="button"
                        aria-label={`Ungroup ${data.name}`}
                        title="Ungroup"
                        className="nodrag nopan flex size-6 items-center justify-center rounded-md text-muted-foreground hover:bg-card hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary"
                        onClick={(event) => {
                            event.stopPropagation();
                            data.onUngroup?.(id);
                        }}
                        onDoubleClick={(event) => event.stopPropagation()}
                    >
                        <Ungroup className="size-4" aria-hidden />
                    </button>
                ) : null}
                {data.onToggleMode ? (
                    <button
                        type="button"
                        aria-label={`Collapse ${data.name}`}
                        aria-expanded={true}
                        title="Collapse"
                        className="nodrag nopan flex size-6 items-center justify-center rounded-md text-muted-foreground hover:bg-card hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary"
                        onClick={(event) => {
                            event.stopPropagation();
                            data.onToggleMode?.(id);
                        }}
                        onDoubleClick={(event) => event.stopPropagation()}
                    >
                        <ChevronUp className="size-4" aria-hidden />
                    </button>
                ) : null}
            </div>
            <Handle type="source" position={Position.Right} />
        </div>
    );
}
