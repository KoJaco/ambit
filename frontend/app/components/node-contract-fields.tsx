import clsx from "clsx";
import { PlusIcon, XIcon } from "lucide-react";
import { useId, useState } from "react";
import type { BuildOrder, NodeContract } from "../api";
import {
    CONTRACT_LISTS,
    LIST_ITEM_MAX,
    LIST_MAX_ITEMS,
    PAYLOADS_MAX,
    buildBadge,
    counterState,
    dependsOptions,
    toggleDependency,
    validateContract,
} from "../node-contract";

export const fieldClass =
    "w-full rounded-[10px] border border-transparent bg-accent px-2.5 py-1.5 hover:border-border focus:border-primary focus:bg-card focus:outline-none focus-visible:ring-2 focus-visible:ring-ring/40";
export const labelClass = "text-xs font-medium uppercase tracking-wide text-muted-foreground";
const iconButtonClass =
    "inline-flex size-7 shrink-0 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring disabled:opacity-40";

// Shown only near or over the limit, and in words, so it is never colour-only.
export function Counter({ id, length, max }: { id: string; length: number; max: number }) {
    const state = counterState(length, max);
    if (!state.near) return <span id={id} className="sr-only" />;
    return (
        <span id={id} className={clsx("text-xs", state.over ? "font-medium text-destructive" : "text-muted-foreground")}>
            {state.over ? "Too long: " : ""}
            {state.text} characters
        </span>
    );
}

export function ContractFields({
    value,
    onChange,
    serverError,
}: {
    value: NodeContract | undefined;
    onChange: (next: NodeContract) => void;
    serverError?: string | null;
}) {
    const uid = useId();
    const contract = value ?? {};
    const problems = validateContract(contract);
    const payloads = contract.payloads ?? "";
    return (
        <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
                <label htmlFor={`${uid}-payloads`} className={labelClass}>
                    Payloads
                </label>
                <textarea
                    id={`${uid}-payloads`}
                    aria-describedby={`${uid}-payloads-count`}
                    rows={4}
                    className={clsx(fieldClass, "min-h-20 resize-y font-mono text-xs")}
                    placeholder="Request and response shapes this node exposes or consumes."
                    value={payloads}
                    onChange={(event) => onChange({ ...contract, payloads: event.target.value })}
                />
                <Counter id={`${uid}-payloads-count`} length={payloads.length} max={PAYLOADS_MAX} />
            </div>
            {CONTRACT_LISTS.map(({ key, label, addLabel, itemLabel }) => {
                const items = contract[key] ?? [];
                return (
                    <fieldset key={key} className="flex min-w-0 flex-col gap-1.5">
                        <legend className={clsx(labelClass, "mb-1.5")}>{label}</legend>
                        {items.length === 0 ? <p className="text-xs text-muted-foreground">None yet.</p> : null}
                        <ul className="flex flex-col gap-1.5">
                            {items.map((item, index) => (
                                <li key={index} className="flex flex-col gap-0.5">
                                    <div className="flex items-center gap-1.5">
                                        <input
                                            aria-label={`${itemLabel} ${index + 1}`}
                                            aria-describedby={`${uid}-${key}-${index}-count`}
                                            className={clsx(fieldClass, "min-w-0 flex-1 text-sm")}
                                            value={item}
                                            onChange={(event) =>
                                                onChange({ ...contract, [key]: items.map((v, i) => (i === index ? event.target.value : v)) })
                                            }
                                        />
                                        <button
                                            type="button"
                                            aria-label={`Remove ${itemLabel.toLowerCase()} ${index + 1}`}
                                            className={iconButtonClass}
                                            onClick={() => onChange({ ...contract, [key]: items.filter((_, i) => i !== index) })}
                                        >
                                            <XIcon className="size-4" aria-hidden />
                                        </button>
                                    </div>
                                    <Counter id={`${uid}-${key}-${index}-count`} length={item.trim().length} max={LIST_ITEM_MAX} />
                                </li>
                            ))}
                        </ul>
                        <button
                            type="button"
                            disabled={items.length >= LIST_MAX_ITEMS}
                            className="inline-flex min-h-7 w-fit items-center gap-1 rounded-full bg-accent px-3 py-1 text-xs font-medium hover:bg-accent/70 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-50"
                            onClick={() => onChange({ ...contract, [key]: [...items, ""] })}
                        >
                            <PlusIcon className="size-3.5" aria-hidden />
                            {addLabel}
                        </button>
                        {items.length >= LIST_MAX_ITEMS ? (
                            <p className="text-xs text-muted-foreground">Limit of {LIST_MAX_ITEMS} reached.</p>
                        ) : null}
                    </fieldset>
                );
            })}
            {problems.length > 0 ? (
                <ul className="text-xs text-destructive" role="alert">
                    {problems.map((p) => (
                        <li key={p}>{p}</li>
                    ))}
                </ul>
            ) : null}
            {serverError ? (
                <p role="alert" className="text-xs text-destructive">
                    {serverError}
                </p>
            ) : null}
        </div>
    );
}

export function DependsPicker({
    selfId,
    value,
    order,
    onChange,
    serverError,
}: {
    selfId: string;
    value: string[] | undefined;
    order: BuildOrder | null;
    onChange: (next: string[]) => void;
    serverError?: string | null;
}) {
    const uid = useId();
    const [filter, setFilter] = useState("");
    const selected = value ?? [];
    const options = dependsOptions(order, selfId, selected);
    const needle = filter.trim().toLowerCase();
    // Selected rows stay visible while filtering so a dependency is never hidden.
    const shown = needle
        ? options.filter((o) => selected.includes(o.id) || o.name.toLowerCase().includes(needle) || o.id.includes(needle))
        : options;
    return (
        <fieldset className="flex min-w-0 flex-col gap-2">
            <legend className="sr-only">Depends on</legend>
            <p id={`${uid}-hint`} className="text-xs text-muted-foreground">
                Nodes that must be built first. A node cannot depend on itself.
            </p>
            {options.length > 8 ? (
                <input
                    aria-label="Filter nodes"
                    aria-describedby={`${uid}-hint`}
                    type="search"
                    className={clsx(fieldClass, "text-sm")}
                    placeholder="Filter nodes"
                    value={filter}
                    onChange={(event) => setFilter(event.target.value)}
                />
            ) : null}
            {options.length === 0 ? (
                <p className="text-xs text-muted-foreground">{order ? "No other nodes." : "Loading nodes."}</p>
            ) : (
                <ul className="flex max-h-48 flex-col overflow-y-auto rounded-[10px] border border-border" aria-label="Nodes">
                    {shown.map((option) => {
                        const checkboxId = `${uid}-${option.id}`;
                        return (
                            <li key={option.id}>
                                <label
                                    htmlFor={checkboxId}
                                    className="flex min-h-7 cursor-pointer items-center gap-2 px-2.5 py-1 text-sm hover:bg-accent"
                                >
                                    <input
                                        id={checkboxId}
                                        type="checkbox"
                                        className="size-4 shrink-0 accent-[var(--primary)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
                                        checked={selected.includes(option.id)}
                                        onChange={() => onChange(toggleDependency(selected, option.id, selfId))}
                                    />
                                    <span className="min-w-0 truncate" title={option.id}>
                                        {option.name}
                                    </span>
                                </label>
                            </li>
                        );
                    })}
                    {shown.length === 0 ? <li className="px-2.5 py-1 text-xs text-muted-foreground">No match.</li> : null}
                </ul>
            )}
            {serverError ? (
                <p role="alert" className="text-xs text-destructive">
                    {serverError}
                </p>
            ) : null}
        </fieldset>
    );
}

// Advisory only: wave and unfinished dependencies from GET /build-order.
export function BuildBadges({ order, id }: { order: BuildOrder | null; id: string }) {
    const badge = buildBadge(order, id);
    if (!badge) return null;
    const pill = "rounded-full px-2 py-0.5 text-[11px]";
    return (
        <ul className="flex flex-wrap gap-1.5" aria-label="Build order (advisory)">
            {badge.wave !== null ? <li className={clsx(pill, "bg-primary/10 font-medium text-primary")}>wave {badge.wave}</li> : null}
            {badge.done ? <li className={clsx(pill, "bg-muted text-muted-foreground")}>done</li> : null}
            {badge.blockedBy.length > 0 ? (
                <li className={clsx(pill, "bg-muted text-foreground")}>blocked by {badge.blockedBy.map((b) => b.name).join(", ")}</li>
            ) : null}
            {badge.inCycle ? <li className={clsx(pill, "bg-destructive/10 font-medium text-destructive")}>dependency cycle</li> : null}
        </ul>
    );
}
