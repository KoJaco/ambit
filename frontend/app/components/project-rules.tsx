import clsx from "clsx";
import { ArrowDownIcon, ArrowUpIcon, PlusIcon, XIcon } from "lucide-react";
import { useEffect, useId, useRef, useState, type RefObject } from "react";
import { getConstraints, putConstraints, type Constraint } from "../api";
import {
    CONSTRAINTS_MAX,
    CONSTRAINT_TEXT_MAX,
    addConstraint,
    constraintsDirty,
    moveConstraint,
    normalizeConstraints,
    removeConstraint,
    setConstraintText,
    toggleSeverity,
    validateConstraints,
} from "../node-contract";
import { Overlay } from "./overlay";
import { CloseButton } from "./shortcuts-dialog";
import { Counter, fieldClass } from "./node-contract-fields";

const iconButtonClass =
    "inline-flex size-7 shrink-0 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring disabled:opacity-40";

// The project's rules (constraints), edited as an ordered list and replaced as a whole on Save.
// There is no SSE for config, so the list is refetched after the architect's own save.
export function ProjectRulesSheet({
    onClose,
    returnFocusRef,
}: {
    onClose: () => void;
    returnFocusRef?: RefObject<HTMLElement | null>;
}) {
    const titleId = useId();
    const closeRef = useRef<HTMLButtonElement>(null);
    const [saved, setSaved] = useState<Constraint[] | null>(null);
    const [draft, setDraft] = useState<Constraint[]>([]);
    const [loadError, setLoadError] = useState<string | null>(null);
    const [saveError, setSaveError] = useState<string | null>(null);
    const [busy, setBusy] = useState(false);
    const [status, setStatus] = useState("");
    const [confirmDiscard, setConfirmDiscard] = useState(false);
    // Row to focus after a move or add, so keyboard users keep their place.
    const focusTarget = useRef<{ index: number; control: "text" | "up" | "down" | "add" } | null>(null);
    const rowRefs = useRef<Record<string, HTMLElement | null>>({});
    const addRef = useRef<HTMLButtonElement>(null);

    useEffect(() => {
        const ac = new AbortController();
        getConstraints(ac.signal)
            .then((res) => {
                setSaved(res.constraints);
                setDraft(res.constraints);
            })
            .catch((err: unknown) => {
                if (err instanceof DOMException && err.name === "AbortError") return;
                setLoadError(err instanceof Error ? err.message : "could not load project rules");
            });
        return () => ac.abort();
    }, []);

    useEffect(() => {
        const target = focusTarget.current;
        if (!target) return;
        focusTarget.current = null;
        if (target.control === "add") addRef.current?.focus();
        else rowRefs.current[`${target.control}-${target.index}`]?.focus();
    });

    const check = validateConstraints(draft);
    const dirty = saved !== null && constraintsDirty(saved, draft);

    function edit(next: Constraint[]) {
        setDraft(next);
        setStatus("");
        setConfirmDiscard(false);
    }

    function requestClose() {
        if (dirty && !confirmDiscard) {
            setConfirmDiscard(true);
            return;
        }
        onClose();
    }

    async function save() {
        if (!check.ok) return;
        setBusy(true);
        setSaveError(null);
        try {
            await putConstraints(normalizeConstraints(draft));
            // Refetch rather than trust the draft: the server owns normalisation.
            const fresh = await getConstraints();
            setSaved(fresh.constraints);
            setDraft(fresh.constraints);
            setStatus("Project rules saved.");
        } catch (err) {
            setSaveError(err instanceof Error ? err.message : "save failed");
        } finally {
            setBusy(false);
        }
    }

    return (
        <Overlay variant="sheet" labelledBy={titleId} onClose={requestClose} returnFocusRef={returnFocusRef} initialFocusRef={closeRef}>
            <div className="flex items-center justify-between gap-2 border-b border-border px-5 py-3">
                <h2 id={titleId} className="text-base font-semibold">
                    Project rules
                </h2>
                <CloseButton label="Close project rules" onClick={requestClose} buttonRef={closeRef} />
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
                <p className="text-sm text-muted-foreground">
                    Rules every node and agent brief must follow. "Must" is required; "Should" is a preference. The order is kept.
                </p>
                {loadError ? (
                    <p role="alert" className="mt-3 text-sm text-destructive">
                        {loadError}
                    </p>
                ) : saved === null ? (
                    <p className="mt-3 text-sm text-muted-foreground">Loading project rules.</p>
                ) : (
                    <>
                        {draft.length === 0 ? <p className="mt-3 text-sm text-muted-foreground">No rules yet.</p> : null}
                        <ol className="mt-3 flex flex-col gap-3" aria-label="Rules">
                            {draft.map((rule, index) => {
                                const message = check.rows[index];
                                const textId = `${titleId}-text-${rule.id}`;
                                return (
                                    <li key={rule.id} className="flex flex-col gap-1 rounded-[14px] border border-border p-2.5">
                                        <div className="flex items-start gap-1.5">
                                            <span className="mt-2 w-5 shrink-0 text-right text-xs text-muted-foreground" aria-hidden>
                                                {index + 1}.
                                            </span>
                                            <textarea
                                                ref={(el) => {
                                                    rowRefs.current[`text-${index}`] = el;
                                                }}
                                                aria-label={`Rule ${index + 1}`}
                                                aria-invalid={message ? true : undefined}
                                                aria-describedby={`${textId}-note`}
                                                rows={2}
                                                className={clsx(fieldClass, "min-w-0 flex-1 resize-y text-sm")}
                                                value={rule.text}
                                                onChange={(event) => edit(setConstraintText(draft, index, event.target.value))}
                                            />
                                        </div>
                                        <div className="flex flex-wrap items-center gap-1.5 pl-6">
                                            <button
                                                type="button"
                                                aria-label={`Rule ${index + 1} severity: ${rule.severity}. Change to ${rule.severity === "must" ? "should" : "must"}`}
                                                className="inline-flex min-h-7 items-center rounded-full border border-border px-3 text-xs font-medium hover:bg-accent focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring"
                                                onClick={() => edit(toggleSeverity(draft, index))}
                                            >
                                                {rule.severity === "must" ? "Must" : "Should"}
                                            </button>
                                            <button
                                                type="button"
                                                ref={(el) => {
                                                    rowRefs.current[`up-${index}`] = el;
                                                }}
                                                aria-label={`Move rule ${index + 1} up`}
                                                disabled={index === 0}
                                                className={iconButtonClass}
                                                onClick={() => {
                                                    focusTarget.current = { index: index - 1, control: "up" };
                                                    edit(moveConstraint(draft, index, -1));
                                                }}
                                            >
                                                <ArrowUpIcon className="size-4" aria-hidden />
                                            </button>
                                            <button
                                                type="button"
                                                ref={(el) => {
                                                    rowRefs.current[`down-${index}`] = el;
                                                }}
                                                aria-label={`Move rule ${index + 1} down`}
                                                disabled={index === draft.length - 1}
                                                className={iconButtonClass}
                                                onClick={() => {
                                                    focusTarget.current = { index: index + 1, control: "down" };
                                                    edit(moveConstraint(draft, index, 1));
                                                }}
                                            >
                                                <ArrowDownIcon className="size-4" aria-hidden />
                                            </button>
                                            <button
                                                type="button"
                                                aria-label={`Remove rule ${index + 1}`}
                                                className={iconButtonClass}
                                                onClick={() => {
                                                    focusTarget.current = draft.length > 1 ? { index: Math.max(0, index - 1), control: "text" } : { index: 0, control: "add" };
                                                    edit(removeConstraint(draft, index));
                                                }}
                                            >
                                                <XIcon className="size-4" aria-hidden />
                                            </button>
                                            <span id={`${textId}-note`} className="ml-auto text-xs">
                                                {message ? (
                                                    <span className="text-destructive">{message}</span>
                                                ) : (
                                                    <Counter id={`${textId}-count`} length={rule.text.trim().length} max={CONSTRAINT_TEXT_MAX} />
                                                )}
                                            </span>
                                        </div>
                                    </li>
                                );
                            })}
                        </ol>
                        <div className="mt-3 flex flex-wrap items-center gap-3">
                            <button
                                ref={addRef}
                                type="button"
                                disabled={draft.length >= CONSTRAINTS_MAX}
                                className="inline-flex min-h-7 items-center gap-1 rounded-full bg-accent px-3 py-1 text-xs font-medium hover:bg-accent/70 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-50"
                                onClick={() => {
                                    focusTarget.current = { index: draft.length, control: "text" };
                                    edit(addConstraint(draft));
                                }}
                            >
                                <PlusIcon className="size-3.5" aria-hidden />
                                Add rule
                            </button>
                            <span className="text-xs text-muted-foreground">
                                {draft.length} of {CONSTRAINTS_MAX} rules
                            </span>
                        </div>
                        {check.list ? (
                            <p role="alert" className="mt-2 text-xs text-destructive">
                                {check.list}
                            </p>
                        ) : null}
                        {saveError ? (
                            <p role="alert" className="mt-2 text-sm text-destructive">
                                {saveError}
                            </p>
                        ) : null}
                        {confirmDiscard ? (
                            <p role="alert" className="mt-2 text-sm text-foreground">
                                You have unsaved changes. Close again to discard them.
                            </p>
                        ) : null}
                        <div className="mt-4 flex items-center gap-3 border-t border-border pt-3">
                            <button
                                type="button"
                                disabled={busy || !dirty || !check.ok}
                                className="inline-flex min-h-7 items-center rounded-full bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-60"
                                onClick={() => void save()}
                            >
                                {busy ? "Saving" : "Save"}
                            </button>
                            {dirty ? <span className="text-xs text-muted-foreground">Unsaved changes</span> : null}
                        </div>
                    </>
                )}
                <p role="status" className="mt-2 text-sm text-muted-foreground">
                    {status}
                </p>
            </div>
        </Overlay>
    );
}
