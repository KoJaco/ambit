import { useEffect, useId, useRef, useState, type RefObject } from "react";
import { getMethod, getMethods, putMethod, type MethodDetail, type MethodSummary } from "../api";
import { gatesSummary, orderRules, severityLabel, switchNotice } from "../method-ui";
import { Overlay } from "./overlay";
import { CloseButton } from "./shortcuts-dialog";

export const sheetButtonClass =
    "inline-flex min-h-7 min-w-6 items-center justify-center rounded-full border border-border px-3 text-xs font-medium hover:bg-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-50";

// The active workflow method: switch it, read its guidance, see its lint rules and gates.
// Switching changes policy only. There is no SSE for config, so everything is refetched after the PUT.
export function MethodSheet({
    onClose,
    onChanged,
    onOpenCheck,
    returnFocusRef,
}: {
    onClose: () => void;
    // Called after a successful switch so the stage chip can refetch.
    onChanged: () => void;
    onOpenCheck: () => void;
    returnFocusRef?: RefObject<HTMLElement | null>;
}) {
    const titleId = useId();
    const selectId = useId();
    const closeRef = useRef<HTMLButtonElement>(null);
    const [method, setMethod] = useState<MethodDetail | null>(null);
    const [methods, setMethods] = useState<MethodSummary[]>([]);
    const [choice, setChoice] = useState("");
    const [loadError, setLoadError] = useState<string | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [busy, setBusy] = useState(false);
    const [notice, setNotice] = useState("");

    async function load(signal?: AbortSignal) {
        const [m, list] = await Promise.all([getMethod(signal), getMethods(signal)]);
        setMethod(m);
        setMethods(list.methods);
        setChoice(m.id);
    }

    useEffect(() => {
        const ac = new AbortController();
        load(ac.signal).catch((err) => {
            if (ac.signal.aborted) return;
            setLoadError(err instanceof Error ? err.message : "could not load the method");
        });
        return () => ac.abort();
    }, []);

    async function apply() {
        if (!method || choice === method.id) return;
        setBusy(true);
        setError(null);
        try {
            const res = await putMethod(choice);
            await load();
            const name = methods.find((m) => m.id === choice)?.name ?? res.method.name;
            setNotice(switchNotice(name, res));
            onChanged();
        } catch (err) {
            setError(err instanceof Error ? err.message : "could not switch method");
        } finally {
            setBusy(false);
        }
    }

    const gates = method ? gatesSummary(method.gates, method.stages) : [];

    return (
        <Overlay variant="sheet" labelledBy={titleId} onClose={onClose} returnFocusRef={returnFocusRef} initialFocusRef={closeRef}>
            <div className="flex items-center justify-between gap-2 border-b border-border px-5 py-3">
                <h2 id={titleId} className="text-base font-semibold">
                    Method
                </h2>
                <CloseButton label="Close method" onClick={onClose} buttonRef={closeRef} />
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
                {loadError ? (
                    <p role="alert" className="text-sm text-destructive">
                        {loadError}
                    </p>
                ) : method === null ? (
                    <p className="text-sm text-muted-foreground">Loading method.</p>
                ) : (
                    <>
                        <p className="text-sm font-semibold">{method.name}</p>
                        <p className="mt-1 text-sm text-muted-foreground">{method.description}</p>

                        <div className="mt-4 flex flex-wrap items-end gap-2">
                            <div className="flex flex-col gap-1">
                                <label htmlFor={selectId} className="text-xs font-medium">
                                    Switch method
                                </label>
                                <select
                                    id={selectId}
                                    aria-describedby={`${selectId}-note`}
                                    value={choice}
                                    disabled={busy}
                                    onChange={(event) => {
                                        setChoice(event.target.value);
                                        setNotice("");
                                    }}
                                    className="min-h-7 rounded-lg border border-border bg-background px-2 py-1 text-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
                                >
                                    {methods.map((m) => (
                                        <option key={m.id} value={m.id}>
                                            {m.name}
                                            {m.active ? " (current)" : ""}
                                        </option>
                                    ))}
                                </select>
                            </div>
                            <button
                                type="button"
                                disabled={busy || choice === method.id}
                                onClick={() => void apply()}
                                className="inline-flex min-h-7 items-center rounded-full bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-60"
                            >
                                {busy ? "Switching" : "Switch"}
                            </button>
                        </div>
                        <p id={`${selectId}-note`} className="mt-1 text-xs text-muted-foreground">
                            Switching changes how ambit guides and checks your work. It never changes your model data.
                        </p>
                        {choice !== method.id ? (
                            <p className="mt-1 text-xs text-muted-foreground">
                                {methods.find((m) => m.id === choice)?.description}
                            </p>
                        ) : null}
                        {error ? (
                            <p role="alert" className="mt-2 text-sm text-destructive">
                                {error}
                            </p>
                        ) : null}
                        <p role="status" className="mt-2 text-sm text-foreground">
                            {notice}
                        </p>

                        <details className="mt-4 rounded-[14px] border border-border px-3 py-2">
                            <summary className="min-h-6 cursor-pointer text-sm font-medium focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring">
                                Guidance
                            </summary>
                            <p className="mt-2 whitespace-pre-wrap text-sm text-muted-foreground">{method.guidance}</p>
                        </details>

                        <section aria-labelledby={`${selectId}-rules`} className="mt-5">
                            <h3 id={`${selectId}-rules`} className="mb-2 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
                                Lint rules
                            </h3>
                            <table className="w-full text-sm">
                                <caption className="sr-only">Model lint rules and their severity in this method</caption>
                                <thead>
                                    <tr className="text-left text-xs text-muted-foreground">
                                        <th scope="col" className="py-1 pr-3 font-medium">
                                            Rule
                                        </th>
                                        <th scope="col" className="py-1 font-medium">
                                            Severity
                                        </th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {orderRules(method.rules).map((rule) => (
                                        <tr key={rule.id} className="border-t border-border">
                                            <th scope="row" className="py-1.5 pr-3 text-left font-mono text-xs font-normal">
                                                {rule.id}
                                            </th>
                                            <td className="py-1.5 text-xs">{severityLabel[rule.severity]}</td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                            <p className="mt-2 text-xs text-muted-foreground">
                                Enforced issues are reported prominently but never block you. Advisory issues are suggestions. Off rules are skipped.
                            </p>
                        </section>

                        <section aria-labelledby={`${selectId}-gates`} className="mt-5">
                            <h3 id={`${selectId}-gates`} className="mb-2 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
                                Stage gates
                            </h3>
                            {gates.length === 0 ? (
                                <p className="text-sm text-muted-foreground">No stage gates.</p>
                            ) : (
                                <ul className="flex flex-col gap-1 text-sm">
                                    {gates.map((g) => (
                                        <li key={g.stage}>
                                            <span className="font-medium">{g.stage}:</span> agents cannot set status {g.statuses.join(", ")}.
                                        </li>
                                    ))}
                                </ul>
                            )}
                        </section>

                        <div className="mt-5 border-t border-border pt-3">
                            <button
                                type="button"
                                className={sheetButtonClass}
                                onClick={() => {
                                    onClose();
                                    onOpenCheck();
                                }}
                            >
                                Open model check
                            </button>
                        </div>
                    </>
                )}
            </div>
        </Overlay>
    );
}
