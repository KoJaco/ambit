import { useId, useRef, useState } from "react";
import clsx from "clsx";
import { LayoutGridIcon, Pencil, Plus, Trash2 } from "lucide-react";
import { deletePart, putParts, type ModelScope, type Part, type ScopeInfo, type Summary } from "../api";
import { partSelection } from "../part-selection";
import styles from "./Sidebar.module.css";
import { parseGlobs, partSummary, validatePartForm, type PartForm } from "../parts-scope";

// Visible 1px border (muted-foreground, >=3:1 against the card) and a solid 2px focus ring; the outline is kept.
const fieldClass =
    "w-full rounded-[10px] border border-muted-foreground bg-accent px-2.5 py-1.5 text-sm focus-visible:border-primary focus-visible:bg-card focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring";
const smallButton =
    "inline-flex size-8 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring";
const pillButton =
    "rounded-full border border-border px-3 py-1.5 text-sm hover:bg-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-60";

// Parts of a partial model. Edits are the human path (PUT/DELETE /parts), not proposals.
export function PartsPanel({
    info,
    roots,
    selectedId,
    onInfo,
}: {
    info: ScopeInfo;
    // All root-level nodes, unfiltered; parts pick their roots from these.
    roots: Summary[];
    selectedId: string | null;
    onInfo: (next: ScopeInfo) => void;
}) {
    const uid = useId();
    const [form, setForm] = useState<PartForm | null>(null);
    const [errors, setErrors] = useState<{ name?: string; include?: string }>({});
    const [error, setError] = useState("");
    const [status, setStatus] = useState("");
    const [confirmId, setConfirmId] = useState<string | null>(null);
    const [busy, setBusy] = useState(false);
    const newRef = useRef<HTMLButtonElement>(null);
    const nameRef = useRef<HTMLInputElement>(null);
    const allRef = useRef<HTMLButtonElement>(null);
    const deleteRefs = useRef(new Map<string, HTMLButtonElement>());
    const keepRef = useRef<HTMLButtonElement>(null);

    function apply(next: ScopeInfo) {
        onInfo(next);
        partSelection.sync(next.parts, next.scope === "partial");
    }

    function open(next: PartForm) {
        setForm(next);
        setErrors({});
        setError("");
        window.setTimeout(() => nameRef.current?.focus(), 0);
    }

    function close() {
        setForm(null);
        setErrors({});
        setError("");
        window.setTimeout(() => newRef.current?.focus(), 0);
    }

    async function save() {
        if (!form) return;
        const result = validatePartForm(form, info.parts);
        if (!result.ok) {
            setErrors(result.errors);
            return;
        }
        setBusy(true);
        setError("");
        try {
            apply(await putParts({ part: result.part }));
            setStatus(`Saved part ${result.part.name}.`);
            close();
        } catch (err) {
            setError(err instanceof Error ? err.message : "could not save the part");
        } finally {
            setBusy(false);
        }
    }

    async function remove(part: Part) {
        setBusy(true);
        setError("");
        try {
            const next = await deletePart(part.id);
            if (selectedId === part.id) partSelection.select(null);
            apply(next);
            setStatus(`Deleted part ${part.name}.`);
            setConfirmId(null);
            // The deleted row is gone: land on the All row (always present), else the New part button.
            window.setTimeout(() => (allRef.current ?? newRef.current)?.focus(), 0);
        } catch (err) {
            setError(err instanceof Error ? err.message : "could not delete the part");
        } finally {
            setBusy(false);
        }
    }

    async function setScope(scope: ModelScope) {
        setBusy(true);
        setError("");
        try {
            apply(await putParts({ scope }));
            if (scope === "whole") partSelection.select(null);
        } catch (err) {
            setError(err instanceof Error ? err.message : "could not change the scope");
        } finally {
            setBusy(false);
        }
    }

    function askDelete(part: Part) {
        setConfirmId(part.id);
        window.setTimeout(() => keepRef.current?.focus(), 0);
    }

    function cancelDelete(part: Part) {
        setConfirmId(null);
        window.setTimeout(() => deleteRefs.current.get(part.id)?.focus(), 0);
    }

    const selectRow = (part: Part | null) => partSelection.select(part);

    return (
        <>
        <div className={clsx("flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-3 pt-3 pb-4", styles.sidebarScroll)}>
            <section aria-label="Part scope">
                <h2 className="mb-1 pr-8 text-xs uppercase tracking-wide text-foreground/60">Parts</h2>
                <ul className="flex flex-col gap-1">
                    <li>
                        <button
                            ref={allRef}
                            type="button"
                            aria-pressed={selectedId === null}
                            onClick={() => selectRow(null)}
                            className={clsx(
                                "flex min-h-8 w-full items-center rounded-[10px] border bg-card px-2 py-1.5 text-left text-sm hover:bg-accent focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring",
                                selectedId === null ? "border-primary bg-accent ring-1 ring-primary" : "border-border",
                            )}
                        >
                            All
                        </button>
                    </li>
                    {info.parts.map((part) => {
                        const selected = part.id === selectedId;
                        return (
                            <li key={part.id} className="flex items-center gap-1">
                                <button
                                    type="button"
                                    aria-pressed={selected}
                                    title={part.name}
                                    onClick={() => selectRow(part)}
                                    className={clsx(
                                        "flex min-h-8 min-w-0 flex-1 flex-col rounded-[10px] border bg-card px-2 py-1 text-left hover:bg-accent focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring",
                                        selected ? "border-primary bg-accent ring-1 ring-primary" : "border-border",
                                    )}
                                >
                                    <span className="truncate text-sm">{part.name}</span>
                                    <span className="text-[11px] text-muted-foreground">{partSummary(part)}</span>
                                </button>
                                {confirmId === part.id ? (
                                    <>
                                        <button type="button" disabled={busy} className={clsx(pillButton, "px-2 py-1 text-xs text-destructive")} onClick={() => void remove(part)}>
                                            Delete
                                        </button>
                                        <button ref={keepRef} type="button" className={clsx(pillButton, "px-2 py-1 text-xs")} onClick={() => cancelDelete(part)}>
                                            Keep
                                        </button>
                                    </>
                                ) : (
                                    <>
                                        <button
                                            type="button"
                                            aria-label={`Edit part ${part.name}`}
                                            className={smallButton}
                                            onClick={() => open({ id: part.id, name: part.name, includeText: part.include.join("\n"), rootIds: part.root_node_ids })}
                                        >
                                            <Pencil className="size-3.5" aria-hidden />
                                        </button>
                                        <button
                                            type="button"
                                            ref={(el) => {
                                                if (el) deleteRefs.current.set(part.id, el);
                                                else deleteRefs.current.delete(part.id);
                                            }}
                                            aria-label={`Delete part ${part.name}`}
                                            className={smallButton}
                                            onClick={() => askDelete(part)}
                                        >
                                            <Trash2 className="size-3.5" aria-hidden />
                                        </button>
                                    </>
                                )}
                            </li>
                        );
                    })}
                </ul>
                {info.parts.length === 0 ? <p className="mt-1 text-xs text-foreground/50">No parts yet.</p> : null}
            </section>

            {form ? (
                <form
                    aria-label={form.id ? `Edit part ${form.name}` : "New part"}
                    className="flex flex-col gap-2 rounded-[14px] border border-border p-2.5"
                    onSubmit={(event) => {
                        event.preventDefault();
                        void save();
                    }}
                    onKeyDown={(event) => {
                        if (event.key !== "Escape") return;
                        event.stopPropagation();
                        close();
                    }}
                >
                    <div>
                        <label htmlFor={`${uid}-name`} className="mb-1 block text-xs font-medium">
                            Name
                        </label>
                        <input
                            id={`${uid}-name`}
                            ref={nameRef}
                            className={fieldClass}
                            value={form.name}
                            aria-invalid={errors.name ? true : undefined}
                            aria-describedby={errors.name ? `${uid}-name-err` : undefined}
                            onChange={(event) => setForm({ ...form, name: event.target.value })}
                        />
                        {errors.name ? (
                            <p id={`${uid}-name-err`} role="alert" className="mt-1 text-xs text-destructive">
                                {errors.name}
                            </p>
                        ) : null}
                    </div>
                    <div>
                        <label htmlFor={`${uid}-include`} className="mb-1 block text-xs font-medium">
                            Include globs (one per line)
                        </label>
                        <textarea
                            id={`${uid}-include`}
                            rows={3}
                            className={clsx(fieldClass, "font-mono text-xs")}
                            value={form.includeText}
                            aria-describedby={errors.include ? `${uid}-inc-err` : `${uid}-inc-hint`}
                            aria-invalid={errors.include ? true : undefined}
                            onChange={(event) => setForm({ ...form, includeText: event.target.value })}
                        />
                        {errors.include ? (
                            <p id={`${uid}-inc-err`} role="alert" className="mt-1 text-xs text-destructive">
                                {errors.include}
                            </p>
                        ) : (
                            <p id={`${uid}-inc-hint`} className="mt-1 text-xs text-muted-foreground">
                                {parseGlobs(form.includeText).length} glob(s). Unowned files matching these are checked for drift.
                            </p>
                        )}
                    </div>
                    <fieldset className="min-w-0">
                        <legend className="mb-1 text-xs font-medium">Root nodes</legend>
                        {roots.length === 0 ? (
                            <p className="text-xs text-muted-foreground">No root-level nodes yet.</p>
                        ) : (
                            <ul className="flex max-h-36 flex-col gap-0.5 overflow-y-auto">
                                {roots.map((root) => {
                                    const checked = form.rootIds.includes(root.id);
                                    return (
                                        <li key={root.id}>
                                            <label className="flex min-h-6 cursor-pointer items-center gap-2 rounded px-1 text-sm hover:bg-accent">
                                                <input
                                                    type="checkbox"
                                                    checked={checked}
                                                    onChange={() =>
                                                        setForm({ ...form, rootIds: checked ? form.rootIds.filter((id) => id !== root.id) : [...form.rootIds, root.id] })
                                                    }
                                                />
                                                <span className="min-w-0 truncate">{root.name}</span>
                                            </label>
                                        </li>
                                    );
                                })}
                            </ul>
                        )}
                    </fieldset>
                    {error ? (
                        <p role="alert" className="text-xs text-destructive">
                            {error}
                        </p>
                    ) : null}
                    <div className="flex gap-2">
                        <button
                            type="submit"
                            disabled={busy}
                            className="flex-1 rounded-full bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-60"
                        >
                            {form.id ? "Save part" : "Create part"}
                        </button>
                        <button type="button" className={pillButton} onClick={close}>
                            Cancel
                        </button>
                    </div>
                </form>
            ) : null}
            {!form && error ? (
                <p role="alert" className="text-xs text-destructive">
                    {error}
                </p>
            ) : null}
            <p role="status" aria-live="polite" className="sr-only">
                {status}
            </p>
        </div>
        <div className="shrink-0 border-t border-border bg-muted px-3 py-2.5">
            <button
                ref={newRef}
                type="button"
                aria-expanded={form !== null}
                className="flex min-h-8 w-full items-center justify-center gap-1.5 rounded-full border border-border bg-card px-3 py-1.5 text-sm font-medium hover:bg-background focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
                onClick={() => (form && !form.id ? nameRef.current?.focus() : open({ name: "", includeText: "", rootIds: [] }))}
            >
                <Plus className="size-4" aria-hidden />
                New part
            </button>
            <div className="mt-1.5">
                <button
                    type="button"
                    disabled={busy}
                    className="flex min-h-8 w-full items-center justify-center gap-1.5 rounded-full px-3 py-1.5 text-sm text-muted-foreground hover:bg-background hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-60"
                    onClick={() => void setScope("whole")}
                >
                    <LayoutGridIcon className="size-4" aria-hidden />
                    Switch to whole model
                </button>
            </div>
        </div>
        </>
    );
}
