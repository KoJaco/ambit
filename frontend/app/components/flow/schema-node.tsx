import { Handle, Position, type NodeProps } from "@xyflow/react";
import clsx from "clsx";
import type { ArchNode } from "../NodeCard";

export function SchemaNode({ data, selected }: NodeProps<ArchNode>) {
    const rows = data.schema ?? [];
    return (
        <div
            className={clsx(
                "min-w-56 overflow-hidden rounded-lg border bg-card shadow-sm select-none",
                selected ? "border-foreground border-2" : "border-foreground/25",
            )}
        >
            <Handle type="target" position={Position.Left} />
            <div className="border-b border-foreground/20 bg-foreground/5 px-3 py-2 text-xs font-semibold">
                {data.name}
            </div>
            <div className="flex flex-col">
                {rows.length === 0 ? (
                    <div className="px-3 py-2 text-[10px] text-foreground/50">No columns</div>
                ) : (
                    rows.map((row) => (
                        <div
                            key={`${row.title}-${row.type}`}
                            className="flex items-center justify-between gap-4 border-t border-foreground/10 px-3 py-1 text-[11px]"
                        >
                            <span className="font-medium">{row.title}</span>
                            <span className="text-foreground/60">{row.type}</span>
                        </div>
                    ))
                )}
            </div>
            <Handle type="source" position={Position.Right} />
        </div>
    );
}
