import clsx from "clsx";
import type { ReactNode } from "react";

export const rightRailPanelClass =
    "pointer-events-auto flex min-h-[75vh] max-h-full w-full flex-col gap-3 overflow-y-auto rounded-xl border border-foreground/25 bg-background/95 p-4 text-sm shadow-sm backdrop-blur";

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
