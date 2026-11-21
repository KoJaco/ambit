import clsx from "clsx";
import type { ReactNode } from "react";

// Animated height/opacity region. Collapsed content is `invisible` (not focusable, not read) once
// the transition ends. Pair with a trigger that sets aria-expanded and aria-controls={id}.
export function Collapsible({
    open,
    id,
    children,
    className,
}: {
    open: boolean;
    id: string;
    children: ReactNode;
    className?: string;
}) {
    return (
        <div
            id={id}
            className={clsx(
                "grid transition-[grid-template-rows,opacity,visibility] duration-200 ease-out motion-reduce:transition-none",
                open ? "visible grid-rows-[1fr] opacity-100" : "invisible grid-rows-[0fr] opacity-0",
                className,
            )}
        >
            <div className="min-h-0 overflow-hidden">{children}</div>
        </div>
    );
}
