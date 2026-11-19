import { Handle, Position, type NodeProps } from "@xyflow/react";
import clsx from "clsx";
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
