import clsx from "clsx";
import type { ReactNode } from "react";

export const rightRailPanelClass =
    "pointer-events-auto flex min-h-[75vh] max-h-full w-full flex-col gap-6 overflow-y-auto rounded-[20px] border border-border bg-card p-6 text-sm shadow-md";

export function RightRailPanel({
    children,
    className,
    onPointerDown,
}: {
    children: ReactNode;
    className?: string;
    onPointerDown?: () => void;
}) {
    return (
        <aside className={clsx(rightRailPanelClass, className)} onPointerDown={onPointerDown}>
            {children}
        </aside>
    );
}
