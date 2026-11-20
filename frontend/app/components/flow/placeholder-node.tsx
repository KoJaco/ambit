import { Handle, Position, type NodeProps } from "@xyflow/react";
import clsx from "clsx";
import type { ArchNode } from "../NodeCard";

export function PlaceholderFrame({ data, selected }: NodeProps<ArchNode>) {
    return (
        <div
            className={clsx(
                "flex h-16 w-36 items-center justify-center rounded-lg border border-dashed bg-card/80 text-lg text-foreground/50 select-none",
                selected ? "border-foreground" : "border-foreground/40",
            )}
            title={data.name}
        >
            <Handle type="target" position={Position.Left} />
            +
            <Handle type="source" position={Position.Right} />
        </div>
    );
}
