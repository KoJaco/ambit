import { useEffect, useRef, useState } from "react";
import { groupButtonLabel, resolveGroupName } from "../canvas-structure";

export type StructureRequest =
    | { kind: "group"; ids: string[]; suggestion: string }
    | { kind: "ungroup"; id: string; name: string };

const buttonClass =
    "inline-flex h-7 min-w-6 items-center justify-center rounded-full px-3 text-[11px] font-medium focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:opacity-50";

// Small inline form for group (asks for a name) and ungroup (asks about the container).
// Esc cancels, Enter confirms. Focus moves in on open; the caller restores it on close.
export function StructurePanel({
    request,
    busy,
    onConfirm,
    onCancel,
}: {
    request: StructureRequest;
    busy: boolean;
    onConfirm: (value: { name: string; deleteContainer: boolean }) => void;
    onCancel: () => void;
}) {
    const [name, setName] = useState(request.kind === "group" ? request.suggestion : "");
    const [deleteContainer, setDeleteContainer] = useState(false);
    const firstRef = useRef<HTMLInputElement | HTMLButtonElement | null>(null);

    useEffect(() => {
        firstRef.current?.focus();
        if (firstRef.current instanceof HTMLInputElement) firstRef.current.select();
    }, []);

    function confirm() {
        if (busy) return;
        onConfirm({
            name: request.kind === "group" ? resolveGroupName(name, request.suggestion) : "",
            deleteContainer,
        });
    }

    const title =
        request.kind === "group" ? groupButtonLabel(request.ids.length) : `Ungroup ${request.name}`;

    return (
        <form
            role="group"
            aria-label={title}
            className="pointer-events-auto flex flex-wrap items-center gap-2 rounded-full border border-border bg-card px-3 py-1.5 text-[11px] text-foreground shadow-sm"
            onSubmit={(event) => {
                event.preventDefault();
                confirm();
            }}
            onKeyDown={(event) => {
                if (event.key === "Escape") {
                    event.preventDefault();
                    event.stopPropagation();
                    onCancel();
                }
            }}
        >
            {request.kind === "group" ? (
                <>
                    <label htmlFor="structure-group-name" className="font-medium">
                        Container name
                    </label>
                    <input
                        id="structure-group-name"
                        ref={(el) => {
                            firstRef.current = el;
                        }}
                        value={name}
                        onChange={(event) => setName(event.target.value)}
                        aria-label="Container name"
                        autoComplete="off"
                        className="h-7 w-40 rounded-full border border-border bg-background px-3 text-[11px] focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary"
                    />
                </>
            ) : (
                <>
                    <span className="font-medium">Move children out of {request.name}?</span>
                    <label className="flex min-h-6 items-center gap-1.5">
                        <input
                            type="checkbox"
                            checked={deleteContainer}
                            onChange={(event) => setDeleteContainer(event.target.checked)}
                            aria-label="Also delete the container"
                            className="size-4 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary"
                        />
                        Also delete the container
                    </label>
                </>
            )}
            <button
                type="submit"
                disabled={busy}
                ref={(el) => {
                    if (request.kind === "ungroup") firstRef.current = el;
                }}
                aria-label={request.kind === "group" ? "Stage group proposal" : "Stage ungroup proposal"}
                className={`${buttonClass} bg-primary text-primary-foreground`}
            >
                {request.kind === "group" ? "Group" : "Ungroup"}
            </button>
            <button
                type="button"
                onClick={onCancel}
                aria-label="Cancel"
                className={`${buttonClass} border border-border hover:bg-muted`}
            >
                Cancel
            </button>
        </form>
    );
}
