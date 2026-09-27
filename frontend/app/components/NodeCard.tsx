import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";
import clsx from "clsx";

export type CrossingMark = {
    direction: string;
    label: string;
    otherId: string;
};

export type ArchNodeData = {
    name: string;
    type: string;
    status: string;
    protected: boolean;
    crossings: CrossingMark[];
};

export type ArchNode = Node<ArchNodeData, "arch">;

export function NodeCard({ data, selected }: NodeProps<ArchNode>) {
    return (
        <div
            className={clsx(
                "min-w-48 rounded-xl border bg-card shadow-sm select-none",
                selected
                    ? "border-none shadow-xl ring-2 ring-offset-2"
                    : "border-foreground/25"
            )}
        >
            <Handle type="target" position={Position.Left} />
            <div className="flex flex-col">
                <div className="flex items-center gap-2 border-b border-foreground/50 px-3 py-2">
                    <div className="h-2 w-2 rounded-full bg-foreground/60" />
                    <div className="text-xs font-semibold uppercase tracking-wide text-foreground">
                        {data.name}
                    </div>
                    {data.protected ? (
                        <span className="ml-auto text-[10px] uppercase tracking-wide text-foreground/70">
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
                                {crossing.direction === "in" ? "←" : "→"} {crossing.label || "relationship"}{" "}
                                {crossing.otherId}
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
