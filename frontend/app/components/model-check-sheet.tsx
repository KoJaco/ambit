import { useEffect, useId, useRef, useState, type RefObject } from "react";
import { useNavigate } from "react-router";
import { getCheckModel, type CheckModelResult } from "../api";
import { checkStatus, driftCount, findingTarget, groupFindings } from "../method-ui";
import { Overlay } from "./overlay";
import { CloseButton } from "./shortcuts-dialog";
import { sheetButtonClass } from "./method-sheet";

// Model lint findings, enforced first. Never blocks anything; the toggles only change what is reported.
export function ModelCheckSheet({
    onClose,
    returnFocusRef,
}: {
    onClose: () => void;
    returnFocusRef?: RefObject<HTMLElement | null>;
}) {
    const titleId = useId();
    const closeRef = useRef<HTMLButtonElement>(null);
    const navigate = useNavigate();
    const [includeDrift, setIncludeDrift] = useState(false);
    const [all, setAll] = useState(false);
    const [result, setResult] = useState<CheckModelResult | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [loading, setLoading] = useState(true);
    const [reload, setReload] = useState(0);

    useEffect(() => {
        const ac = new AbortController();
        setLoading(true);
        setError(null);
        getCheckModel({ includeDrift, all }, ac.signal)
            .then((r) => {
                setResult(r);
                setLoading(false);
            })
            .catch((err) => {
                if (ac.signal.aborted) return;
                setError(err instanceof Error ? err.message : "could not run the model check");
                setLoading(false);
            });
        return () => ac.abort();
    }, [includeDrift, all, reload]);

    const groups = result ? groupFindings(result.findings) : [];
    const drift = result?.drift;
    const checkboxClass = "size-4 accent-[var(--color-primary)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring";

    return (
        <Overlay variant="sheet" labelledBy={titleId} onClose={onClose} returnFocusRef={returnFocusRef} initialFocusRef={closeRef}>
            <div className="flex items-center justify-between gap-2 border-b border-border px-5 py-3">
                <h2 id={titleId} className="text-base font-semibold">
                    Model check
                </h2>
                <CloseButton label="Close model check" onClick={onClose} buttonRef={closeRef} />
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
                <p className="text-sm text-muted-foreground">
                    Checks the model against the active method's lint rules. It never blocks your work.
                </p>
                <div className="mt-3 flex flex-wrap items-center gap-x-5 gap-y-2">
                    <label className="inline-flex min-h-6 items-center gap-2 text-sm">
                        <input type="checkbox" className={checkboxClass} checked={includeDrift} onChange={(e) => setIncludeDrift(e.target.checked)} />
                        Include drift
                    </label>
                    <label className="inline-flex min-h-6 items-center gap-2 text-sm">
                        <input type="checkbox" className={checkboxClass} checked={all} onChange={(e) => setAll(e.target.checked)} />
                        All rules
                    </label>
                    <button type="button" className={sheetButtonClass} disabled={loading} onClick={() => setReload((n) => n + 1)}>
                        Run again
                    </button>
                </div>
                <p className="mt-1 text-xs text-muted-foreground">
                    Drift compares tracked files with node globs. All rules runs every rule as advisory, even ones the method turns off.
                </p>

                <div role="status" aria-live="polite" className="mt-4 text-sm font-medium">
                    {loading ? "Checking the model." : result ? checkStatus(result) : ""}
                </div>
                {error ? (
                    <p role="alert" className="mt-2 text-sm text-destructive">
                        {error}
                    </p>
                ) : null}

                {result ? (
                    <>
                        <p className="mt-1 text-xs text-muted-foreground">
                            Method {result.method}
                            {result.stage ? `, stage ${result.stage}` : ""}. Enforced {result.counts.enforced}, advisory {result.counts.advisory}, total {result.counts.total}.
                        </p>
                        {groups.map((group) => (
                            <section key={group.severity} aria-label={`${group.label} findings`} className="mt-5">
                                <h3 className="mb-2 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
                                    {group.label} ({group.findings.length})
                                </h3>
                                <ul className="flex flex-col gap-2">
                                    {group.findings.map((f, i) => {
                                        const target = findingTarget(f);
                                        return (
                                            <li key={`${f.rule}-${f.node_id ?? f.relationship_id ?? ""}-${i}`} className="flex items-start justify-between gap-3 rounded-[14px] border border-border p-2.5">
                                                <div className="min-w-0">
                                                    <p className="font-mono text-xs text-muted-foreground">{f.rule}</p>
                                                    <p className="text-sm break-words">{f.message}</p>
                                                </div>
                                                {target ? (
                                                    <button
                                                        type="button"
                                                        className={`${sheetButtonClass} shrink-0`}
                                                        aria-label={`Go to ${target.label}`}
                                                        onClick={() => {
                                                            onClose();
                                                            navigate(target.href);
                                                        }}
                                                    >
                                                        Go to
                                                    </button>
                                                ) : null}
                                            </li>
                                        );
                                    })}
                                </ul>
                            </section>
                        ))}

                        {result.drift_error ? (
                            <p role="alert" className="mt-4 text-sm text-destructive">
                                Drift could not run: {result.drift_error}
                            </p>
                        ) : null}
                        {drift ? (
                            <section aria-label="Drift" className="mt-5">
                                <h3 className="mb-2 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">Drift ({driftCount(drift)})</h3>
                                {driftCount(drift) === 0 ? <p className="text-sm text-muted-foreground">No drift found.</p> : null}
                                {drift.unowned_files?.length ? (
                                    <div className="mb-3">
                                        <h4 className="text-xs font-medium">Files no node owns ({drift.unowned_files.length})</h4>
                                        <ul className="mt-1 flex flex-col gap-0.5 font-mono text-xs break-all">
                                            {drift.unowned_files.map((p) => (
                                                <li key={p}>{p}</li>
                                            ))}
                                        </ul>
                                    </div>
                                ) : null}
                                {drift.unmatched_globs?.length ? (
                                    <div className="mb-3">
                                        <h4 className="text-xs font-medium">Globs that match no file ({drift.unmatched_globs.length})</h4>
                                        <ul className="mt-1 flex flex-col gap-0.5 text-xs">
                                            {drift.unmatched_globs.map((g) => (
                                                <li key={`${g.node}-${g.field}-${g.glob}`} className="break-all">
                                                    <span className="font-mono">{g.glob}</span> ({g.field} of {g.node})
                                                </li>
                                            ))}
                                        </ul>
                                    </div>
                                ) : null}
                                {drift.overlaps?.length ? (
                                    <div>
                                        <h4 className="text-xs font-medium">Files owned by several nodes ({drift.overlaps.length})</h4>
                                        <ul className="mt-1 flex flex-col gap-0.5 text-xs">
                                            {drift.overlaps.map((o) => (
                                                <li key={o.path} className="break-all">
                                                    <span className="font-mono">{o.path}</span>: {o.owners.join(", ")}
                                                </li>
                                            ))}
                                        </ul>
                                    </div>
                                ) : null}
                            </section>
                        ) : null}
                    </>
                ) : null}
            </div>
        </Overlay>
    );
}
