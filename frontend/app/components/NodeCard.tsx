import React from "react";
import clsx from "clsx";

type NodeCardData = {
    id: string;
    displayData: {
        kind: string;
        title: string;
        bgColor?: string;
        textColor?: string;
        icon?: React.ReactNode;
    };
    x: number;
    y: number;
    width: number;
    height: number;
};

export function NodeCard({
    node,
    onPointerDown,
    isSelected = false,
}: {
    node: NodeCardData;
    onPointerDown: (e: React.PointerEvent) => void;
    isSelected?: boolean;
}) {
    return (
        <div
            data-node="true"
            className={clsx(
                "absolute z-100 rounded-xl border bg-card shadow-sm select-none",
                isSelected
                    ? "border-none shadow-xl ring-2 ring-offset-2"
                    : "border-foreground/25"
            )}
            style={{
                left: node.x,
                top: node.y,
                width: node.width,
                height: node.height,
            }}
            onPointerDown={onPointerDown}
        >
            <div className="flex h-full flex-col">
                <div className="flex items-center gap-2 px-3 py-2 border-b border-foreground/50">
                    {node.displayData.icon ?? (
                        <div
                            className={clsx(
                                "h-2 w-2 rounded-full",
                                node.displayData.bgColor
                            )}
                        />
                    )}
                    <div className="text-xs font-semibold uppercase tracking-wide text-foreground">
                        {node.displayData.title}
                    </div>
                </div>

                {/* Node Content / Func area */}
            </div>
        </div>
    );
}

export default NodeCard;
