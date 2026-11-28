import { useCallback, useEffect, useId, useRef, useState, type RefObject } from "react";
import { BotIcon, CheckIcon, LockIcon, UserIcon, type LucideIcon } from "lucide-react";
import { getBuildOrder, putGate, type BuildClass, type BuildItem, type BuildOrder } from "../api";
import { CLASS_LABEL, chipDescription, gateRowLabel, groupGates, remainingGates, summaryText, waveColumns, withGateCleared } from "../run-plan";
import { Overlay } from "./overlay";
import { CloseButton } from "./shortcuts-dialog";
import { sheetButtonClass } from "./method-sheet";

const CLASS_ICON: Record<BuildClass, LucideIcon> = { "agent-parallel": BotIcon, "needs-human": UserIcon, blocked: LockIcon };
// Class is carried by icon + label text; the border style repeats it but is never the only cue.
const CLASS_BORDER: Record<BuildClass, string> = {
    "agent-parallel": "border-primary",
    "needs-human": "border-dashed border-foreground",
    blocked: "border-destructive",
};

const headingClass = "mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground";

function Chip({ item, ready, nameOf }: { item: BuildItem; ready: boolean; nameOf: (id: string) => string }) {
    const Icon = item.done ? CheckIcon : CLASS_ICON[item.class];
    return (
        <li
            aria-label={chipDescription(item, nameOf)}
            className={`rounded-[14px] border bg-card p-2 text-sm ${CLASS_BORDER[item.class]} ${item.done ? "opacity-60" : ""} ${ready ? "ring-2 ring-primary" : ""}`}
        >
            <p className={`break-words font-medium ${item.done ? "line-through" : ""}`}>{item.name}</p>
            <p className="mt-0.5 inline-flex items-center gap-1 text-xs text-muted-foreground">
                <Icon aria-hidden className="size-3.5" />
                {item.done ? "Done" : CLASS_LABEL[item.class]}
                {ready && !item.done ? " (ready now)" : ""}
            </p>
            {item.blocked_by.length > 0 && !item.done ? (
                <p className="mt-0.5 text-xs break-words">Blocked by: {item.blocked_by.map(nameOf).join(", ")}</p>
            ) : null}
        </li>
    );
}

// The build plan: what can start now, the waves, and every human gate in one checklist.
// Advisory only; clearing a gate is stored per user and never blocks anything by itself.
export function RunPlanSheet({
    onClose,
    returnFocusRef,
    version = 0,
}: {
    onClose: () => void;
    returnFocusRef?: RefObject<HTMLElement | null>;
    // Bumped on model-changed events so the plan refetches after outside changes.
    version?: number;
}) {
    const titleId = useId();
    const closeRef = useRef<HTMLButtonElement>(null);
    const [plan, setPlan] = useState<BuildOrder | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [gateError, setGateError] = useState("");
    const [loading, setLoading] = useState(true);
    const [reload, setReload] = useState(0);
    // Gate writes in flight; a refetch that lands meanwhile must not undo the optimistic state.
    const pending = useRef(0);

    const load = useCallback((signal: AbortSignal) => {
        return getBuildOrder(undefined, signal)
            .then((r) => {
                if (pending.current === 0) setPlan(r);
                setError(null);
                setLoading(false);
            })
            .catch((err) => {
                if (signal.aborted) return;
                setError(err instanceof Error ? err.message : "could not load the run plan");
                setLoading(false);
            });
    }, []);

    useEffect(() => {
        const ac = new AbortController();
        void load(ac.signal);
        return () => ac.abort();
    }, [load, version, reload]);

    async function toggle(nodeId: string, index: number, cleared: boolean) {
        if (!plan) return;
        setGateError("");
        setPlan((p) => (p ? withGateCleared(p, nodeId, index, cleared) : p));
        pending.current++;
        try {
            await putGate(nodeId, index, cleared);
            pending.current--;
            // Clearing a gate can move a node from needs-human to ready; re-derive the plan.
            setReload((n) => n + 1);
        } catch (err) {
            pending.current--;
            setPlan((p) => (p ? withGateCleared(p, nodeId, index, !cleared) : p));
            setGateError(err instanceof Error ? err.message : "could not save that gate");
        }
    }

    const nameMap = new Map<string, string>();
    for (const i of plan?.order ?? []) nameMap.set(i.id, i.name);
    for (const e of plan?.externals ?? []) nameMap.set(e.id, e.name);
    const nameOf = (id: string) => nameMap.get(id) ?? id;

    const columns = plan ? waveColumns(plan) : [];
    const groups = plan ? groupGates(plan) : [];
    const ready = new Set(plan?.ready_now ?? []);
    const parallel = new Set(plan?.parallel_set ?? []);
    const left = remainingGates(groups);
    const checkboxClass = "mt-0.5 size-4 shrink-0 accent-[var(--color-primary)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring";

    return (
        <Overlay variant="sheet" labelledBy={titleId} onClose={onClose} returnFocusRef={returnFocusRef} initialFocusRef={closeRef}>
            <div className="flex items-center justify-between gap-2 border-b border-border px-5 py-3">
                <h2 id={titleId} className="text-base font-semibold">
                    Run plan
                </h2>
                <CloseButton label="Close run plan" onClick={onClose} buttonRef={closeRef} />
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
                <p className="text-sm text-muted-foreground">The advisory build order from dependencies and gates. It never blocks your work.</p>
                <div className="mt-3 flex flex-wrap items-center gap-3">
                    <p role={error ? "alert" : "status"} aria-live={error ? "assertive" : "polite"} className={`text-sm font-medium ${error ? "text-destructive" : ""}`}>
                        {error ? error : plan ? summaryText(plan) : loading ? "Loading run plan" : ""}
                    </p>
                    <button type="button" className={sheetButtonClass} disabled={loading && !plan} onClick={() => setReload((n) => n + 1)}>
                        Refresh
                    </button>
                </div>

                {plan ? (
                    <>
                        <section aria-label="Ready now" className="mt-5">
                            <h3 className={headingClass}>Ready now</h3>
                            {plan.ready_now.length === 0 ? (
                                <p className="text-sm text-muted-foreground">Nothing is ready to build.</p>
                            ) : (
                                <>
                                    <ul className="flex flex-wrap gap-2 p-1.5">
                                        {plan.ready_now.map((id) => (
                                            <li
                                                key={id}
                                                className={`inline-flex items-center gap-1 rounded-full border px-3 py-1 text-sm ${parallel.has(id) ? "border-primary bg-primary text-primary-foreground" : "border-border"}`}
                                            >
                                                {parallel.has(id) ? <BotIcon aria-hidden className="size-3.5" /> : null}
                                                {nameOf(id)}
                                                {parallel.has(id) ? <span className="text-xs">(in parallel set)</span> : null}
                                            </li>
                                        ))}
                                    </ul>
                                    <p className="mt-1 text-xs text-muted-foreground">
                                        The parallel set are ready nodes whose files do not overlap, so they can be built at the same time.
                                    </p>
                                </>
                            )}
                        </section>

                        <section aria-label="Waves" className="mt-5">
                            <h3 className={headingClass}>Waves</h3>
                            {columns.length === 0 ? <p className="text-sm text-muted-foreground">No buildable nodes.</p> : null}
                            <div className="flex gap-4 overflow-x-auto p-2" tabIndex={0} role="group" aria-label="Waves, scroll horizontally">
                                {columns.map((col) => (
                                    <div key={col.wave} className="w-44 shrink-0">
                                        <h4 className="mb-1 text-xs font-medium">
                                            Wave {col.wave + 1}{" "}
                                            <span className="font-normal text-muted-foreground">
                                                ({col.doneCount}/{col.items.length} done)
                                            </span>
                                        </h4>
                                        <ul className="flex flex-col gap-2">
                                            {col.items.map((item) => (
                                                <Chip key={item.id} item={item} ready={ready.has(item.id)} nameOf={nameOf} />
                                            ))}
                                        </ul>
                                    </div>
                                ))}
                            </div>
                            {plan.cycles.length > 0 ? (
                                <p role="alert" className="mt-2 text-sm text-destructive">
                                    Dependency cycles: {plan.cycles.map((c) => c.map(nameOf).join(" to ")).join("; ")}
                                </p>
                            ) : null}
                            {plan.unordered.length > 0 ? (
                                <p className="mt-2 text-xs text-muted-foreground">Not ordered: {plan.unordered.map(nameOf).join(", ")}</p>
                            ) : null}
                        </section>

                        <section aria-label="Human gates" className="mt-5">
                            <h3 className={headingClass}>Human gates {groups.length ? `(${left} left)` : ""}</h3>
                            {groups.length === 0 ? <p className="text-sm text-muted-foreground">No human gates in this model.</p> : null}
                            <p role="status" aria-live="polite" className="sr-only">
                                {groups.length ? `${left} human gates left` : ""}
                            </p>
                            {gateError ? (
                                <p role="alert" className="mb-2 text-sm text-destructive">
                                    {gateError}
                                </p>
                            ) : null}
                            {groups.map((group) => (
                                <fieldset key={group.kind} className="mb-3 min-w-0">
                                    <legend className="mb-1 text-sm font-medium">
                                        {group.label} <span className="font-normal text-muted-foreground">({group.remaining} left)</span>
                                    </legend>
                                    <ul className="flex flex-col gap-1.5 px-1.5 py-1">
                                        {group.rows.map((row) => (
                                            <li key={row.key}>
                                                <label className="flex min-h-6 cursor-pointer items-start gap-2 text-sm">
                                                    <input
                                                        type="checkbox"
                                                        className={checkboxClass}
                                                        checked={row.cleared}
                                                        onChange={(e) => void toggle(row.nodeId, row.index, e.target.checked)}
                                                    />
                                                    <span className={`break-words ${row.cleared ? "text-muted-foreground line-through" : ""}`}>
                                                        {gateRowLabel(row)}
                                                        {row.external ? " (external)" : ""}
                                                    </span>
                                                </label>
                                            </li>
                                        ))}
                                    </ul>
                                </fieldset>
                            ))}
                        </section>

                        <section aria-label="Externals" className="mt-5">
                            <h3 className={headingClass}>Externals ({plan.externals.length})</h3>
                            {plan.externals.length === 0 ? (
                                <p className="text-sm text-muted-foreground">No external systems.</p>
                            ) : (
                                <>
                                    <p className="mb-1 text-xs text-muted-foreground">Not built here and never blocking. Listed so you know what you depend on.</p>
                                    <ul className="flex flex-col gap-1 px-1.5 py-1 text-sm">
                                        {plan.externals.map((e) => (
                                            <li key={e.id} className="break-words">
                                                {e.name} <span className="text-xs text-muted-foreground">({e.status}, not buildable)</span>
                                            </li>
                                        ))}
                                    </ul>
                                </>
                            )}
                        </section>
                    </>
                ) : null}
            </div>
        </Overlay>
    );
}
