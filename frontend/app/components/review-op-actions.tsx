import clsx from "clsx";
import { Check, X } from "lucide-react";
import { Tooltip } from "./tooltip";

export function ReviewOpActions({
    canAccept,
    needsConfirm,
    busy,
    onAccept,
    onReject,
    label,
    className,
}: {
    canAccept: boolean;
    needsConfirm: boolean;
    busy: boolean;
    onAccept: () => void;
    onReject: () => void;
    // What the change is, so each button has a unique accessible name.
    label: string;
    className?: string;
}) {
    return (
        <div className={clsx("flex gap-1", className)}>
            {canAccept ? (
                <Tooltip label={needsConfirm ? "Apply over newer edit" : "Accept"}>
                <button
                    type="button"
                    className={clsx(
                        "inline-flex size-7 items-center justify-center rounded-md hover:bg-foreground/10",
                        needsConfirm
                            ? "text-amber-700 dark:text-amber-400"
                            : "text-emerald-600",
                    )}
                    disabled={busy}
                    aria-label={`${needsConfirm ? "Apply over newer edit" : "Accept"}: ${label}`}
                    onClick={(event) => {
                        event.stopPropagation();
                        onAccept();
                    }}
                >
                    <Check className="size-4" aria-hidden />
                </button>
                </Tooltip>
            ) : null}
            <Tooltip label="Reject">
            <button
                type="button"
                className="inline-flex size-7 items-center justify-center rounded-md text-red-600 hover:bg-foreground/10"
                disabled={busy}
                aria-label={`Reject: ${label}`}
                onClick={(event) => {
                    event.stopPropagation();
                    onReject();
                }}
            >
                <X className="size-4" aria-hidden />
            </button>
            </Tooltip>
        </div>
    );
}
