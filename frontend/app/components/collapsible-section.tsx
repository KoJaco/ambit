import clsx from "clsx";
import { ChevronDown } from "lucide-react";
import type { ReactNode } from "react";
import { Collapsible } from "./collapsible";

// A bordered card with a header button that expands a scrollable body. Shared by the inspector's
// Appearance and Node settings and the new-node form. When open the header is rounded on top only,
// so its hover fill meets the body border cleanly.
export function CollapsibleSection({
    title,
    summary,
    open,
    onToggle,
    id,
    maxHeightClass = "max-h-[45vh]",
    children,
}: {
    title: string;
    // Shown under the title while collapsed.
    summary?: ReactNode;
    open: boolean;
    onToggle: () => void;
    id: string;
    maxHeightClass?: string;
    children: ReactNode;
}) {
    return (
        <div className="overflow-hidden rounded-[14px] border border-border">
            <button
                type="button"
                aria-expanded={open}
                aria-controls={id}
                className={clsx(
                    "flex w-full items-start justify-between gap-3 px-3 py-2.5 text-left hover:bg-accent focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring",
                    open ? "rounded-t-[14px] rounded-b-none" : "rounded-[14px]",
                )}
                onClick={onToggle}
            >
                <span className="flex min-w-0 flex-col gap-1.5">
                    <span className="text-sm font-medium">{title}</span>
                    {open ? null : summary}
                </span>
                <ChevronDown
                    className={clsx(
                        "mt-0.5 size-4 shrink-0 text-muted-foreground transition-transform duration-200 motion-reduce:transition-none",
                        open && "rotate-180",
                    )}
                    aria-hidden
                />
            </button>
            <Collapsible open={open} id={id}>
                <div className={clsx("overflow-y-auto border-t border-border p-3", maxHeightClass)}>{children}</div>
            </Collapsible>
        </div>
    );
}
