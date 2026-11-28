import { useEffect, useId, useRef, useState, type RefObject } from "react";
import { getProposalAuditDigest, type AuditDigest } from "../api";
import { digestSummary, entryViews, ruleLabel, ruleRows } from "../audit-digest";
import { Overlay } from "./overlay";
import { CloseButton } from "./shortcuts-dialog";

// Proposals the architect agent accepted under the auto-accept policy, with the rule that allowed each op.
export function ArchitectAcceptedSheet({
    onClose,
    returnFocusRef,
    version = 0,
}: {
    onClose: () => void;
    returnFocusRef?: RefObject<HTMLElement | null>;
    // Bumped on model-changed events so the digest refetches.
    version?: number;
}) {
    const titleId = useId();
    const closeRef = useRef<HTMLButtonElement>(null);
    const [digest, setDigest] = useState<AuditDigest | null>(null);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        const ac = new AbortController();
        getProposalAuditDigest({}, ac.signal)
            .then((d) => {
                setDigest(d);
                setError(null);
            })
            .catch((err) => {
                if (ac.signal.aborted) return;
                setError(err instanceof Error ? err.message : "could not load the digest");
            });
        return () => ac.abort();
    }, [version]);

    const entries = digest ? entryViews(digest.entries) : [];
    const rules = digest ? ruleRows(digest.by_rule) : [];

    return (
        <Overlay variant="sheet" labelledBy={titleId} onClose={onClose} returnFocusRef={returnFocusRef} initialFocusRef={closeRef}>
            <div className="flex items-center justify-between gap-2 border-b border-border px-5 py-3">
                <h2 id={titleId} className="text-base font-semibold">
                    Architect accepted
                </h2>
                <CloseButton label="Close architect accepted" onClick={onClose} buttonRef={closeRef} />
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
                {error ? (
                    <p role="alert" className="text-sm text-destructive">
                        {error}
                    </p>
                ) : digest === null ? (
                    <p role="status" className="text-sm text-muted-foreground">
                        Loading the digest.
                    </p>
                ) : entries.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                        Nothing has been auto-accepted yet. When auto-accept is on in the build stage, proposals the architect agent accepts are listed here.
                    </p>
                ) : (
                    <>
                        <p role="status" className="text-sm font-medium">
                            {digestSummary(digest)}
                        </p>
                        {rules.length > 0 ? (
                            <section aria-labelledby={`${titleId}-rules`} className="mt-4">
                                <h3 id={`${titleId}-rules`} className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                                    By rule
                                </h3>
                                <table className="w-full text-sm">
                                    <caption className="sr-only">Auto-accepted changes by policy rule</caption>
                                    <thead>
                                        <tr className="text-left text-xs text-muted-foreground">
                                            <th scope="col" className="py-1 pr-3 font-medium">
                                                Rule
                                            </th>
                                            <th scope="col" className="py-1 text-right font-medium">
                                                Changes
                                            </th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        {rules.map((r) => (
                                            <tr key={r.rule} className="border-t border-border">
                                                <th scope="row" className="py-1.5 pr-3 text-left font-mono text-xs font-normal">
                                                    {ruleLabel(r.rule)}
                                                </th>
                                                <td className="py-1.5 text-right text-xs">{r.count}</td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            </section>
                        ) : null}
                        <section aria-labelledby={`${titleId}-recent`} className="mt-5">
                            <h3 id={`${titleId}-recent`} className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                                Recent
                            </h3>
                            <ul className="flex flex-col gap-2">
                                {entries.map((e) => (
                                    <li key={e.key} className="rounded-[14px] border border-border p-3">
                                        <p className="flex flex-wrap items-baseline gap-x-2 text-sm">
                                            <span className="font-mono text-xs font-medium">{e.shortId}</span>
                                            <time className="text-xs text-muted-foreground" title={e.timestamp}>
                                                {e.age}
                                            </time>
                                            <span className="text-xs text-muted-foreground">{e.actor}</span>
                                        </p>
                                        <ul className="mt-1.5 flex flex-col gap-1">
                                            {e.ops.map((o) => (
                                                <li key={o.key} className="text-xs break-words">
                                                    {o.text} <span className="font-mono text-muted-foreground">({ruleLabel(o.rule)})</span>
                                                </li>
                                            ))}
                                        </ul>
                                    </li>
                                ))}
                            </ul>
                        </section>
                    </>
                )}
            </div>
        </Overlay>
    );
}
