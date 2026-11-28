import { useEffect, useId, useRef, useState, type RefObject } from "react";
import { Select } from "./ui/select";
import { getMethod, getMethods, getProposalPolicy, putMethod, putProposalPolicy, type MethodDetail, type MethodSummary, type ProposalPolicyInfo } from "../api";
import { autoAcceptCanToggle, autoAcceptHint, autoAcceptStatus } from "../auto-accept";
import { formatRuleParams, gatesSummary, orderRules, severityLabel, switchNotice } from "../method-ui";
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
    onOpenAccepted,
    returnFocusRef,
    version = 0,
}: {
    onClose: () => void;
    // Called after a successful switch so the stage chip can refetch.
    onChanged: () => void;
    onOpenCheck: () => void;
    onOpenAccepted?: () => void;
    returnFocusRef?: RefObject<HTMLElement | null>;
    // Changes when the model or config changed elsewhere; the sheet refetches.
    version?: number;
}) {
    const titleId = useId();
    const selectId = useId();
    const closeRef = useRef<HTMLButtonElement>(null);
    const selectRef = useRef<HTMLButtonElement>(null);
    const [method, setMethod] = useState<MethodDetail | null>(null);
    const [methods, setMethods] = useState<MethodSummary[]>([]);
    const [choice, setChoice] = useState("");
    const [loadError, setLoadError] = useState<string | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [busy, setBusy] = useState(false);
    const [notice, setNotice] = useState("");
    const [policy, setPolicy] = useState<ProposalPolicyInfo | null>(null);
    const [policyError, setPolicyError] = useState<string | null>(null);
    const [policyBusy, setPolicyBusy] = useState(false);

    async function toggleAutoAccept(next: boolean) {
        setPolicyBusy(true);
        setPolicyError(null);
        try {
            setPolicy(await putProposalPolicy(next));
        } catch (err) {
            setPolicyError(err instanceof Error ? err.message : "could not change auto-accept");
        } finally {
            setPolicyBusy(false);
        }
    }

    async function load(signal?: AbortSignal) {
        const [m, list] = await Promise.all([getMethod(signal), getMethods(signal)]);
        setMethod(m);
        setMethods(list.methods);
        setChoice((prev) => (prev === "" ? m.id : prev));
        return m;
    }

    useEffect(() => {
        const ac = new AbortController();
        load(ac.signal).catch((err) => {
            if (ac.signal.aborted) return;
            setLoadError(err instanceof Error ? err.message : "could not load the method");
        });
        getProposalPolicy(ac.signal)
            .then(setPolicy)
            .catch(() => {
                if (!ac.signal.aborted) setPolicy(null);
            });
        return () => ac.abort();
    }, [version]);

    async function apply() {
        if (!method || choice === method.id) return;
        setBusy(true);
        setError(null);
        try {
            const res = await putMethod(choice);
            const fresh = await load();
            setChoice(fresh.id);
            const name = methods.find((m) => m.id === res.method)?.name ?? fresh.name;
            setNotice(switchNotice(name, res, (id) => fresh.stages.find((s) => s.id === id)?.name ?? id));
            onChanged();
        } catch (err) {
            setError(err instanceof Error ? err.message : "could not switch method");
        } finally {
            setBusy(false);
            // The Switch button is disabled now; keep focus in the sheet.
            window.requestAnimationFrame(() => selectRef.current?.focus());
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
                                <Select
                                    id={selectId}
                                    ref={selectRef}
                                    aria-describedby={`${selectId}-note`}
                                    value={choice}
                                    disabled={busy}
                                    onChange={(next) => {
                                        setChoice(next);
                                        setNotice("");
                                    }}
                                    className="min-h-7 min-w-40"
                                    options={methods.map((m) => ({ value: m.id, label: `${m.name}${m.active ? " (current)" : ""}` }))}
                                />
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
                            <h3 id={`${selectId}-rules`} className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
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
                                            <td className="py-1.5 text-xs">{severityLabel[rule.severity]}
                                                {formatRuleParams(rule.params) ? <span className="text-muted-foreground"> ({formatRuleParams(rule.params)})</span> : null}
                                            </td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                            <p className="mt-2 text-xs text-muted-foreground">
                                Enforced issues are reported prominently but never block you. Advisory issues are suggestions. Off rules are skipped.
                            </p>
                        </section>

                        <section aria-labelledby={`${selectId}-gates`} className="mt-5">
                            <h3 id={`${selectId}-gates`} className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
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

                        {policy ? (
                            <section aria-labelledby={`${selectId}-auto`} className="mt-5">
                                <h3 id={`${selectId}-auto`} className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                                    Architect auto-accept
                                </h3>
                                <div className="flex items-center gap-3">
                                    <button
                                        type="button"
                                        role="switch"
                                        id={`${selectId}-auto-switch`}
                                        aria-checked={policy.auto_accept}
                                        aria-describedby={`${selectId}-auto-desc`}
                                        disabled={policyBusy || !autoAcceptCanToggle(policy)}
                                        onClick={() => void toggleAutoAccept(!policy.auto_accept)}
                                        className={`relative inline-flex h-6 w-11 shrink-0 items-center rounded-full border border-border transition-colors motion-reduce:transition-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-50 ${policy.auto_accept ? "bg-primary" : "bg-accent"}`}
                                    >
                                        <span
                                            aria-hidden
                                            className={`inline-block size-4 rounded-full bg-card shadow transition-transform motion-reduce:transition-none ${policy.auto_accept ? "translate-x-6" : "translate-x-1"}`}
                                        />
                                    </button>
                                    <label htmlFor={`${selectId}-auto-switch`} className="text-sm font-medium">
                                        Let the architect agent accept safe proposals
                                    </label>
                                </div>
                                <p id={`${selectId}-auto-desc`} className="mt-1 text-xs text-muted-foreground">
                                    {autoAcceptHint(policy) || "Only low-risk changes qualify. Protected nodes, new external nodes, and stage, risk or review changes still need you."}
                                </p>
                                <p role="status" aria-live="polite" className="mt-1 text-sm">
                                    {autoAcceptStatus(policy)}
                                </p>
                                {policyError ? (
                                    <p role="alert" className="mt-1 text-sm text-destructive">
                                        {policyError}
                                    </p>
                                ) : null}
                            </section>
                        ) : null}

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
                            {onOpenAccepted ? (
                                <button
                                    type="button"
                                    className={`${sheetButtonClass} ml-2`}
                                    onClick={() => {
                                        onClose();
                                        onOpenAccepted();
                                    }}
                                >
                                    Architect accepted
                                </button>
                            ) : null}
                        </div>
                    </>
                )}
            </div>
        </Overlay>
    );
}
