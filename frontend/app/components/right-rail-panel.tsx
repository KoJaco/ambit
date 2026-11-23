import clsx from "clsx";
import { X } from "lucide-react";
import { useEffect, type KeyboardEvent, type ReactNode } from "react";
import { registerRailPanel } from "../right-rail-state";
import { Tooltip } from "./tooltip";
import { ModeToggle } from "./ui/mode-toggle";

// Right-edge sheet: full height of its rail, rounded on the left only, flush to the screen edge.
export const rightRailPanelClass =
    "pointer-events-auto relative flex h-full max-h-full w-full flex-col gap-6 overflow-y-auto rounded-l-[20px] border-y border-l border-border bg-card p-6 text-sm shadow-lg";

export function RightRailPanel({
    children,
    className,
    onPointerDown,
    onKeyDown,
    label,
}: {
    children: ReactNode;
    className?: string;
    onPointerDown?: () => void;
    onKeyDown?: (event: KeyboardEvent<HTMLElement>) => void;
    // Accessible name of the complementary landmark.
    label?: string;
}) {
    useEffect(() => registerRailPanel("panels"), []);
    return (
        <aside aria-label={label} className={clsx(rightRailPanelClass, className)} onPointerDown={onPointerDown} onKeyDown={onKeyDown}>
            {children}
        </aside>
    );
}

// Theme toggle plus close button for a panel header. Place it in a `relative` header, or let it
// sit in a flex row (`absolute` is opt-in via the `floating` prop).
export function PanelHeaderActions({
    onClose,
    closeLabel = "Close panel",
    floating = false,
    hideClose = false,
}: {
    onClose?: () => void;
    closeLabel?: string;
    floating?: boolean;
    hideClose?: boolean;
}) {
    useEffect(() => registerRailPanel("toggles"), []);
    return (
        <div className={clsx("flex shrink-0 items-center gap-1.5", floating && "absolute top-4 right-4 z-10")}>
            <ModeToggle />
            {onClose && !hideClose ? (
                <Tooltip label={closeLabel}>
                    <button
                        type="button"
                        aria-label={closeLabel}
                        className="inline-flex size-8 shrink-0 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring"
                        onClick={onClose}
                    >
                        <X className="size-4" aria-hidden />
                    </button>
                </Tooltip>
            ) : null}
        </div>
    );
}
