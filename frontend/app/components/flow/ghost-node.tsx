import { Handle, Position, type NodeProps } from "@xyflow/react";
import clsx from "clsx";
import { Plus } from "lucide-react";
import { nodeDotClass, nodeGlyphClass, resolveIconComponent, resolveNodeColor } from "../../node-presentation";
import type { ArchNode } from "../NodeCard";

// A pending new node. The react-flow wrapper carries the accessible name and the keyboard and
// pointer activation (see FlowCanvas); this is only the picture: dashed outline, "+" and "New".
function NewBadge() {
    return (
        <span className="inline-flex shrink-0 items-center gap-0.5 rounded-full bg-primary px-1.5 py-px text-[11px] leading-normal font-semibold text-primary-foreground">
            <Plus className="size-3" aria-hidden strokeWidth={3} />
            New
        </span>
    );
}

export function GhostNode({ data }: NodeProps<ArchNode>) {
    const colorToken = resolveNodeColor(data.type, data.color);
    const Icon = data.icon ? resolveIconComponent(data.icon) : null;
    const glyph = Icon ? (
        <span className={nodeGlyphClass(colorToken)} aria-hidden>
            <Icon className="size-4" strokeWidth={2} />
        </span>
    ) : (
        <div className={nodeDotClass(colorToken)} aria-hidden />
    );
    if (data.ghost?.container) {
        return (
            <div className="relative h-full w-full cursor-pointer rounded-[20px] border-2 border-dashed border-primary bg-primary/5 select-none">
                <Handle type="target" position={Position.Left} />
                <div className="flex h-11 items-center gap-2 px-4">
                    {glyph}
                    <span className="min-w-0 flex-1 truncate text-sm font-semibold text-foreground">{data.name}</span>
                    <NewBadge />
                </div>
                <Handle type="source" position={Position.Right} />
            </div>
        );
    }
    return (
        <div className="relative h-full w-full cursor-pointer rounded-[14px] border-2 border-dashed border-primary bg-card/80 select-none">
            <Handle type="target" position={Position.Left} />
            <div className="flex items-center gap-2.5 px-3 py-2.5">
                {glyph}
                <div className="min-w-0 flex-1">
                    <div className="truncate text-sm font-medium text-foreground">{data.name}</div>
                    <div className="truncate text-xs text-muted-foreground">{data.type}</div>
                </div>
                <NewBadge />
            </div>
            <Handle type="source" position={Position.Right} />
        </div>
    );
}

export const ghostNodeClass = (highlighted: boolean) => clsx("ghost-node", highlighted && "ghost-highlight");
