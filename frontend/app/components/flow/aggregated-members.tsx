import { useEffect, useRef, useState } from "react";
import { Link } from "react-router";
import { getRelationshipLevel, type Relationship } from "../../api";
import { edgePillClass, edgePillDrillableClass } from "./edge-shared";

const maxProbed = 20;

// Pill for a rolled-up edge that stands for several relationships. Click or Enter opens a small list
// of member ids. The level payload does not say which members have an interior, so on open each
// member is probed once (404 means no interior) and only drillable ones get a link.
export function AggregatedPill({
    rel,
    label,
    fromNodeId,
    onOpenChange,
}: {
    rel: Relationship;
    label: string;
    fromNodeId?: string;
    // Lets the label wrapper raise itself above neighbouring pills while the popover is open.
    onOpenChange?: (open: boolean) => void;
}) {
    const [open, setOpen] = useState(false);
    useEffect(() => {
        onOpenChange?.(open);
        return () => onOpenChange?.(false);
    }, [open, onOpenChange]);
    const [drillable, setDrillable] = useState<Record<string, boolean>>({});
    const rootRef = useRef<HTMLDivElement>(null);
    const members = rel.members ?? [];
    const kinds = (rel.kinds ?? []).join(", ");

    useEffect(() => {
        if (!open) return;
        const ac = new AbortController();
        for (const id of members.slice(0, maxProbed)) {
            getRelationshipLevel(id, ac.signal)
                .then(() => setDrillable((current) => ({ ...current, [id]: true })))
                .catch(() => undefined);
        }
        return () => ac.abort();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [open, members.join("\0")]);

    useEffect(() => {
        if (!open) return;
        const onPointerDown = (event: PointerEvent) => {
            if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
        };
        document.addEventListener("pointerdown", onPointerDown);
        return () => document.removeEventListener("pointerdown", onPointerDown);
    }, [open]);

    return (
        <div
            ref={rootRef}
            className="nodrag nopan pointer-events-auto relative"
            onKeyDown={(event) => {
                if (event.key === "Escape") setOpen(false);
            }}
        >
            <button
                type="button"
                aria-expanded={open}
                aria-haspopup="true"
                title={kinds ? `Kinds: ${kinds}` : undefined}
                className={`block min-h-6 max-w-[11rem] cursor-pointer break-words text-center leading-normal focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary ${edgePillClass} ${edgePillDrillableClass}`}
                onClick={(event) => {
                    event.stopPropagation();
                    setOpen((value) => !value);
                }}
            >
                {label}
            </button>
            {open ? (
                <div
                    role="group"
                    aria-label={label}
                    className="absolute top-full left-1/2 z-50 mt-1 w-56 -translate-x-1/2 rounded-xl border border-border bg-card p-1.5 text-left shadow-md"
                >
                    {kinds ? <p className="px-2 pb-1 text-[10px] text-muted-foreground">Kinds: {kinds}</p> : null}
                    <ul className="max-h-48 overflow-auto text-xs">
                        {members.map((id) => (
                            <li key={id} className="flex min-h-6 items-center justify-between gap-2 px-2">
                                <span className="truncate" title={id}>
                                    {id}
                                </span>
                                {drillable[id] ? (
                                    <Link
                                        to={`/relationship/${id}`}
                                        state={{ fromNodeId }}
                                        aria-label={`Open relationship ${id}`}
                                        className="shrink-0 font-medium text-primary underline focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary"
                                    >
                                        Open
                                    </Link>
                                ) : null}
                            </li>
                        ))}
                    </ul>
                </div>
            ) : null}
        </div>
    );
}
