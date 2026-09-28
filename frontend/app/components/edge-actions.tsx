import { useState } from "react";
import { deleteRelationship, type Relationship } from "../api";
import { RightRailPanel } from "./right-rail-panel";

export function EdgeActions({
    edge,
    onClose,
    onMutated,
    onDeleted,
    onFocus,
}: {
    edge: Relationship;
    onClose: () => void;
    onMutated: () => void;
    onDeleted?: () => void;
    onFocus?: () => void;
}) {
    const [confirmDelete, setConfirmDelete] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [busy, setBusy] = useState(false);

    async function run(action: () => Promise<void>) {
        setBusy(true);
        setError(null);
        try {
            await action();
            onMutated();
        } catch (err: unknown) {
            setError(err instanceof Error ? err.message : "request failed");
        } finally {
            setBusy(false);
        }
    }

    return (
        <RightRailPanel onPointerDown={onFocus}>
            <div className="flex items-center justify-between gap-2">
                <h2 className="text-xs font-semibold uppercase tracking-wide">Relationship</h2>
                <button type="button" className="text-xs text-foreground/60" onClick={onClose}>
                    Close
                </button>
            </div>
            <p className="text-sm">
                {edge.from} → {edge.to}
            </p>
            <p className="text-xs text-foreground/70">
                {edge.label || "(no label)"} · {edge.kind || "—"}
            </p>
            <p className="mt-1 text-[10px] text-foreground/50">id: {edge.id}</p>
            {!confirmDelete ? (
                <button
                    type="button"
                    disabled={busy}
                    className="mt-4 rounded border border-red-600/40 px-2 py-1 text-sm text-red-600"
                    onClick={() => setConfirmDelete(true)}
                >
                    Delete relationship…
                </button>
            ) : (
                <div className="mt-4 rounded border border-red-600/30 bg-red-600/5 p-2 text-sm">
                    <p className="text-red-700">
                        Deletes this edge and all interior members (including nested interiors).
                    </p>
                    <div className="mt-2 flex gap-2">
                        <button
                            type="button"
                            disabled={busy}
                            className="rounded bg-red-600 px-2 py-1 text-white"
                            onClick={() =>
                                void run(async () => {
                                    await deleteRelationship(edge.id);
                                    onClose();
                                    onDeleted?.();
                                })
                            }
                        >
                            Confirm delete
                        </button>
                        <button type="button" className="rounded border px-2 py-1" onClick={() => setConfirmDelete(false)}>
                            Cancel
                        </button>
                    </div>
                </div>
            )}
            {error ? <p className="mt-2 text-xs text-red-600">{error}</p> : null}
        </RightRailPanel>
    );
}
