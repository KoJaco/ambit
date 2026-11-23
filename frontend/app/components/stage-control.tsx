import { useEffect, useId, useRef, useState } from "react";
import { getCheckModel, getStage, putStage, type StageState } from "../api";
import { decideStageChange, nextStage, stageChipLabel, stageMove } from "../method-ui";
import { Overlay } from "./overlay";
import { CloseButton } from "./shortcuts-dialog";
import { sheetButtonClass } from "./method-sheet";

// "Stage: Design" chip in the control bar. Renders nothing for a stage-less method.
// Human-only: advancing never blocks on enforced findings, it only asks first.
export function StageControl({ version, onOpenCheck, onChanged }: { version: number; onOpenCheck: () => void; onChanged: () => void }) {
    const [state, setState] = useState<StageState | null>(null);
    const [open, setOpen] = useState(false);
    const chipRef = useRef<HTMLButtonElement>(null);

    useEffect(() => {
        const ac = new AbortController();
        getStage(ac.signal)
            .then(setState)
            .catch(() => {
                if (!ac.signal.aborted) setState(null);
            });
        return () => ac.abort();
    }, [version]);

    if (!state || state.stages.length === 0) return null;
    return (
        <>
            <button
                ref={chipRef}
                type="button"
                aria-haspopup="dialog"
                aria-label={`${stageChipLabel(state.stage)}. Change stage`}
                onClick={() => setOpen(true)}
                className="mr-1 inline-flex h-7 min-w-6 items-center rounded-full border border-border px-3 text-[11px] font-medium hover:bg-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
            >
                {stageChipLabel(state.stage)}
            </button>
            {open ? (
                <StageDialog
                    state={state}
                    returnFocusRef={chipRef}
                    onClose={() => setOpen(false)}
                    onOpenCheck={onOpenCheck}
                    onApplied={(next) => {
                        setState(next);
                        onChanged();
                    }}
                />
            ) : null}
        </>
    );
}

function StageDialog({
    state,
    returnFocusRef,
    onClose,
    onOpenCheck,
    onApplied,
}: {
    state: StageState;
    returnFocusRef: React.RefObject<HTMLElement | null>;
    onClose: () => void;
    onOpenCheck: () => void;
    onApplied: (next: StageState) => void;
}) {
    const titleId = useId();
    const selectId = useId();
    const closeRef = useRef<HTMLButtonElement>(null);
    const current = state.stage?.id ?? null;
    const [choice, setChoice] = useState(current ?? state.stages[0].id);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [status, setStatus] = useState("");
    const [confirm, setConfirm] = useState<{ target: string; message: string } | null>(null);

    async function apply(target: string) {
        setBusy(true);
        setError(null);
        setConfirm(null);
        try {
            await putStage(target);
            const fresh = await getStage();
            onApplied(fresh);
            setStatus(`Stage is now ${fresh.stage?.name ?? target}.`);
            setChoice(fresh.stage?.id ?? target);
        } catch (err) {
            setError(err instanceof Error ? err.message : "could not change stage");
        } finally {
            setBusy(false);
        }
    }

    async function request(target: string) {
        setStatus("");
        const move = stageMove(state.stages, current, target);
        if (move === "same" || move === "unknown") return;
        if (move !== "later") return void apply(target);
        setBusy(true);
        setError(null);
        try {
            const result = await getCheckModel();
            const decision = decideStageChange(move, result.counts.enforced);
            if (decision.action === "confirm") {
                setBusy(false);
                setConfirm({ target, message: decision.message });
                return;
            }
        } catch (err) {
            // The check is advisory; if it fails, say so and let the architect proceed deliberately.
            setBusy(false);
            setError(`Could not check the model first: ${err instanceof Error ? err.message : "request failed"}. Choose Set stage again to continue.`);
            setConfirm({ target, message: "The model check could not run. Advance anyway?" });
            return;
        }
        void apply(target);
    }

    const next = nextStage(state.stages, current);
    const targetName = (id: string) => state.stages.find((s) => s.id === id)?.name ?? id;

    return (
        <Overlay variant="dialog" labelledBy={titleId} onClose={onClose} returnFocusRef={returnFocusRef} initialFocusRef={closeRef}>
            <div className="flex items-center justify-between gap-2 border-b border-border px-5 py-3">
                <h2 id={titleId} className="text-base font-semibold">
                    Stage
                </h2>
                <CloseButton label="Close stage" onClick={onClose} buttonRef={closeRef} />
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
                <p className="text-sm">
                    Method {state.method.name}. Current stage: <span className="font-medium">{state.stage?.name ?? "not set"}</span>.
                </p>
                <p className="mt-1 text-xs text-muted-foreground">Only you change the stage. Agents cannot.</p>

                {confirm ? (
                    <div role="alertdialog" aria-label="Confirm stage change" className="mt-4 rounded-[14px] border border-border p-3">
                        <p className="text-sm font-medium">{confirm.message}</p>
                        <p className="mt-1 text-xs text-muted-foreground">Enforced issues never block a stage change.</p>
                        <div className="mt-3 flex flex-wrap gap-2">
                            <button
                                type="button"
                                autoFocus
                                disabled={busy}
                                className="inline-flex min-h-7 items-center rounded-full bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-60"
                                onClick={() => void apply(confirm.target)}
                            >
                                Advance anyway
                            </button>
                            <button
                                type="button"
                                className={sheetButtonClass}
                                onClick={() => {
                                    onClose();
                                    onOpenCheck();
                                }}
                            >
                                View issues
                            </button>
                            <button type="button" className={sheetButtonClass} onClick={() => setConfirm(null)}>
                                Cancel
                            </button>
                        </div>
                    </div>
                ) : (
                    <>
                        <div className="mt-4 flex flex-wrap items-end gap-2">
                            <div className="flex flex-col gap-1">
                                <label htmlFor={selectId} className="text-xs font-medium">
                                    Set stage
                                </label>
                                <select
                                    id={selectId}
                                    value={choice}
                                    disabled={busy}
                                    onChange={(e) => {
                                        setChoice(e.target.value);
                                        setStatus("");
                                    }}
                                    className="min-h-7 rounded-lg border border-border bg-background px-2 py-1 text-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
                                >
                                    {state.stages.map((s) => (
                                        <option key={s.id} value={s.id}>
                                            {s.name}
                                        </option>
                                    ))}
                                </select>
                            </div>
                            <button
                                type="button"
                                disabled={busy || choice === current}
                                className={sheetButtonClass}
                                onClick={() => void request(choice)}
                            >
                                Set stage
                            </button>
                            {next ? (
                                <button
                                    type="button"
                                    disabled={busy}
                                    className="inline-flex min-h-7 items-center rounded-full bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-60"
                                    onClick={() => void request(next.id)}
                                >
                                    Advance to {targetName(next.id)}
                                </button>
                            ) : null}
                        </div>
                        <div className="mt-3">
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
                {error ? (
                    <p role="alert" className="mt-3 text-sm text-destructive">
                        {error}
                    </p>
                ) : null}
                <p role="status" className="mt-3 text-sm text-muted-foreground">
                    {status}
                </p>
            </div>
        </Overlay>
    );
}
