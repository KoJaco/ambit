import clsx from "clsx";
import { PlusIcon, XIcon } from "lucide-react";
import { useId, useLayoutEffect, useRef, useState } from "react";
import type { BuildOrder, NodeContract } from "../api";
import type { DependsOption } from "../node-contract";
import {
    CONTRACT_LISTS,
    LIST_ITEM_MAX,
    LIST_MAX_ITEMS,
    PAYLOADS_MAX,
    buildBadge,
    contractFieldProblems,
    counterState,
    dependsOptions,
    toggleDependency,
} from "../node-contract";

export const fieldClass =
    "w-full rounded-[10px] border border-transparent bg-accent px-2.5 py-1.5 hover:border-border focus:border-primary focus:bg-card focus:outline-none focus-visible:ring-2 focus-visible:ring-ring/40";
export const labelClass = "text-xs font-medium uppercase tracking-wide text-muted-foreground";
const iconButtonClass =
    "inline-flex size-7 shrink-0 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring disabled:opacity-40";

// Shown only near or over the limit, and in words, so it is never colour-only. The visible count
// is not live (it would announce on every keystroke); a separate hidden polite region changes
// only when the state crosses into "near" or "over".
export function Counter({ id, length, max }: { id: string; length: number; max: number }) {
    const state = counterState(length, max);
    const announcement = state.over ? "Over the limit" : state.near ? "Near the limit" : "";
    return (
        <>
            <span aria-live="polite" className="sr-only">
                {announcement}
            </span>
            {state.near ? (
                <span id={id} className={clsx("text-xs", state.over ? "font-medium text-destructive" : "text-muted-foreground")}>
                    {state.over ? "Too long: " : ""}
                    {state.text} characters
                </span>
            ) : (
                <span id={id} className="sr-only" />
            )}
        </>
    );
}

// Single-line list item that wraps and grows with its content. Enter is swallowed because items
// are single-line strings.
function AutoGrowItem({
    value,
    onValueChange,
    className,
    ...rest
}: {
    value: string;
    onValueChange: (next: string) => void;
    className?: string;
} & Omit<React.TextareaHTMLAttributes<HTMLTextAreaElement>, "value" | "onChange" | "className">) {
    const ref = useRef<HTMLTextAreaElement>(null);
    useLayoutEffect(() => {
        const el = ref.current;
        if (!el) return;
        el.style.height = "auto";
        el.style.height = `${el.scrollHeight}px`;
    }, [value]);
    return (
        <textarea
            {...rest}
            ref={ref}
            rows={1}
            className={clsx(className, "resize-none overflow-hidden transition-[height] duration-150 motion-reduce:transition-none")}
            value={value}
            onChange={(event) => onValueChange(event.target.value.replace(/[\r\n]+/g, " "))}
            onKeyDown={(event) => {
                if (event.key === "Enter" && !event.nativeEvent.isComposing) event.preventDefault();
                rest.onKeyDown?.(event);
            }}
        />
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
    const fieldProblems = contractFieldProblems(contract);
    const payloads = contract.payloads ?? "";
    return (
        <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
                <label htmlFor={`${uid}-payloads`} className={labelClass}>
                    Payloads
                </label>
                <textarea
                    id={`${uid}-payloads`}
                    aria-describedby={`${uid}-payloads-count${fieldProblems.payloads ? ` ${uid}-payloads-error` : ""}`}
                    aria-invalid={fieldProblems.payloads ? true : undefined}
                    rows={4}
                    className={clsx(fieldClass, "min-h-20 resize-y font-mono text-xs")}
                    placeholder="Request and response shapes this node exposes or consumes."
                    value={payloads}
                    onChange={(event) => onChange({ ...contract, payloads: event.target.value })}
                />
                {fieldProblems.payloads ? (
                    <p id={`${uid}-payloads-error`} role="alert" className="text-xs text-destructive">
                        {fieldProblems.payloads}
                    </p>
                ) : null}
                <Counter id={`${uid}-payloads-count`} length={payloads.trim().length} max={PAYLOADS_MAX} />
            </div>
            {CONTRACT_LISTS.map(({ key, label, addLabel, itemLabel }) => {
                const items = contract[key] ?? [];
                const listProblems = fieldProblems.lists[key];
                return (
                    <fieldset key={key} className="flex min-w-0 flex-col gap-1.5">
                        <legend className={clsx(labelClass, "mb-1.5")}>{label}</legend>
                        {items.length === 0 ? <p className="text-xs text-muted-foreground">None yet.</p> : null}
                        {listProblems.list ? (
                            <p role="alert" className="text-xs text-destructive">
                                {listProblems.list}
                            </p>
                        ) : null}
                        <ul className="flex flex-col gap-1.5">
                            {items.map((item, index) => (
                                <li key={index} className="flex flex-col gap-0.5">
                                    <div className="flex items-start gap-1.5">
                                        <AutoGrowItem
                                            aria-label={`${itemLabel} ${index + 1}`}
                                            aria-describedby={`${uid}-${key}-${index}-count${listProblems.items[index] ? ` ${uid}-${key}-${index}-error` : ""}`}
                                            aria-invalid={listProblems.items[index] ? true : undefined}
                                            className={clsx(fieldClass, "min-w-0 flex-1 text-sm")}
                                            value={item}
                                            onValueChange={(next) =>
                                                onChange({ ...contract, [key]: items.map((v, i) => (i === index ? next : v)) })
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
                                    {listProblems.items[index] ? (
                                        <p id={`${uid}-${key}-${index}-error`} role="alert" className="text-xs text-destructive">
                                            {listProblems.items[index]}
                                        </p>
                                    ) : null}
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
    nodes,
    onChange,
    serverError,
}: {
    selfId: string;
    value: string[] | undefined;
    nodes: DependsOption[] | null;
    onChange: (next: string[]) => void;
    serverError?: string | null;
}) {
    const uid = useId();
    const [filter, setFilter] = useState("");
    const selected = value ?? [];
    const options = dependsOptions(nodes, selfId, selected);
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
            {selected.length > 0 ? (
                <ul className="flex flex-wrap gap-1.5" aria-label="Selected dependencies">
                    {selected.map((id) => {
                        const name = options.find((o) => o.id === id)?.name ?? id;
                        return (
                            <li
                                key={id}
                                className="inline-flex max-w-full items-center gap-1 rounded-full bg-primary/10 py-0.5 pl-2.5 pr-1 text-xs font-medium text-primary"
                            >
                                <span className="min-w-0 break-words" title={id}>
                                    {name}
                                </span>
                                <button
                                    type="button"
                                    aria-label={`Remove dependency ${name}`}
                                    className="inline-flex size-5 shrink-0 items-center justify-center rounded-full hover:bg-primary/20 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring"
                                    onClick={() => onChange(toggleDependency(selected, id, selfId))}
                                >
                                    <XIcon className="size-3.5" aria-hidden />
                                </button>
                            </li>
                        );
                    })}
                </ul>
            ) : null}
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
                <p className="text-xs text-muted-foreground">{nodes ? "No other nodes." : "Loading nodes."}</p>
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
export function BuildBadges({
    order,
    id,
    nameOf,
}: {
    order: BuildOrder | null;
    id: string;
    nameOf?: (id: string) => string | undefined;
}) {
    const badge = buildBadge(order, id, nameOf);
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
