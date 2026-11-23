import { BaseEdge, EdgeLabelRenderer, type EdgeProps } from "@xyflow/react";
import clsx from "clsx";
import { edgeLabelStyle, useEdgeGeometry, type FlowEdgeData } from "./edge-shared";

export type GhostVariant = "new" | "changed" | "removed";

export type GhostEdgeData = FlowEdgeData & {
    ghost?: {
        variant: GhostVariant;
        label: string;
        hasContract?: boolean;
        highlighted: boolean;
        onActivate: () => void;
        onHover: (on: boolean) => void;
    };
};

const STROKE: Record<GhostVariant, string> = {
    new: "var(--primary)",
    changed: "var(--ghost-changed)",
    removed: "var(--ghost-removed)",
};
const pillBase = "min-h-6 max-w-[11rem] rounded-full border px-2.5 py-1 text-[11px] leading-normal font-medium";
const BADGE: Record<GhostVariant, string> = { new: "+ New", changed: "● Changed", removed: "✕ Removed" };
const PILL: Record<GhostVariant, string> = {
    new: "border-primary bg-card text-foreground",
    changed: "border-[var(--ghost-changed)] bg-[var(--ghost-changed-bg)] text-[var(--ghost-changed-fg)]",
    removed: "border-[var(--ghost-removed)] bg-[var(--ghost-removed-bg)] text-[var(--ghost-removed-fg)]",
};

// A dashed edge for a pending change: new (primary), changed (amber) or removed (red). Static
// dashes only, so it needs no reduced-motion variant. The pill is a button that opens the
// proposal item; it names the change in text as well as colour.
export function GhostEdge(props: EdgeProps) {
    const { data, edgePath, labelX, labelY } = useEdgeGeometry(props);
    const ghost = (data as GhostEdgeData | undefined)?.ghost;
    if (!ghost) return null;
    const { variant } = ghost;
    const title = `${BADGE[variant].slice(2)}: ${ghost.label || "connection"}${ghost.hasContract ? " · contract" : ""}`;
    return (
        <>
            <BaseEdge
                id={props.id}
                path={edgePath}
                markerEnd={props.markerEnd}
                interactionWidth={24}
                style={{ stroke: STROKE[variant], strokeDasharray: "7 5", strokeWidth: ghost.highlighted ? 3.5 : 2 }}
            />
            <EdgeLabelRenderer>
                <button
                    type="button"
                    className={clsx(
                        "nodrag nopan pointer-events-auto absolute flex items-center gap-1 border-dashed",
                        pillBase,
                        PILL[variant],
                        ghost.highlighted && "outline-2 outline-offset-2 outline-[var(--ring)]",
                    )}
                    style={edgeLabelStyle(labelX, labelY - 14, ghost.highlighted)}
                    title={title}
                    aria-label={`${title}. Open in Review`}
                    onClick={(event) => {
                        event.stopPropagation();
                        ghost.onActivate();
                    }}
                    onMouseEnter={() => ghost.onHover(true)}
                    onMouseLeave={() => ghost.onHover(false)}
                    onFocus={() => ghost.onHover(true)}
                    onBlur={() => ghost.onHover(false)}
                >
                    <span className="shrink-0 font-semibold">{BADGE[variant]}</span>
                    {ghost.label ? <span className={clsx("min-w-0 truncate", variant === "removed" && "line-through")}>{ghost.label}</span> : null}
                    {ghost.hasContract ? <span className="shrink-0">· contract</span> : null}
                </button>
            </EdgeLabelRenderer>
        </>
    );
}
