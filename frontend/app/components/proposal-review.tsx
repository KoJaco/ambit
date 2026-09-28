import { useEffect, useState } from "react";
import clsx from "clsx";
import {
    acceptAll,
    acceptOperation,
    deleteProposal,
    getProposal,
    listProposals,
    rejectAll,
    rejectOperation,
    type AcceptAllResult,
    type ProposalDiff,
    type ProposalSummary,
} from "../api";
import { proposalRows } from "../proposal-rows";

export function ProposalReview({ refreshKey, clearRight }: { refreshKey: number; clearRight: boolean }) {
    const [summaries, setSummaries] = useState<ProposalSummary[]>([]);
    const [diffs, setDiffs] = useState<Record<string, ProposalDiff>>({});
    const [skipped, setSkipped] = useState<Record<string, AcceptAllResult["skipped"]>>({});
    const [error, setError] = useState<string | null>(null);
    const [busy, setBusy] = useState(false);

    useEffect(() => {
        const ac = new AbortController();
        setError(null);
        listProposals()
            .then(async ({ proposals }) => {
                if (ac.signal.aborted) return;
                const next: Record<string, ProposalDiff> = {};
                await Promise.all(
                    proposals
                        .filter((proposal) => !proposal.unreadable)
                        .map(async (proposal) => {
                            next[proposal.proposal_id] = await getProposal(proposal.proposal_id);
                        }),
                );
                if (ac.signal.aborted) return;
                setSummaries(proposals);
                setDiffs(next);
            })
            .catch((err: unknown) => {
                if (ac.signal.aborted) return;
                setError(err instanceof Error ? err.message : "proposal request failed");
            });
        return () => ac.abort();
    }, [refreshKey]);

    async function run(action: () => Promise<void>) {
        setBusy(true);
        setError(null);
        try {
            await action();
        } catch (err: unknown) {
            setError(err instanceof Error ? err.message : "proposal request failed");
        } finally {
            setBusy(false);
        }
    }

    if (summaries.length === 0 && !error) return null;

    return (
        <aside
            className={clsx(
                "pointer-events-auto absolute bottom-0 left-72 z-10 flex max-h-[45%] min-w-0 flex-col overflow-hidden rounded-xl border border-foreground/25 bg-background/95 text-sm shadow-sm backdrop-blur",
                clearRight ? "right-80" : "right-0",
            )}
        >
            <div className="flex items-center justify-between border-b border-foreground/15 px-3 py-2">
                <h2 className="text-xs uppercase tracking-wide text-foreground/60">Review</h2>
                <span className="text-xs text-foreground/60">{summaries.length}</span>
            </div>
            {error ? <p className="px-3 py-2 text-red-600">{error}</p> : null}
            <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto p-3">
                {summaries.map((proposal) => (
                    <section key={proposal.proposal_id} className="flex flex-col gap-2">
                        <header className="flex flex-wrap items-baseline justify-between gap-2">
                            <div>
                                <p className="font-medium">
                                    {proposal.unreadable
                                        ? `Unreadable ${proposal.proposal_id}`
                                        : proposal.summary || `${proposal.source} ${proposal.proposal_id}`}
                                </p>
                                {proposal.error ? <p className="text-xs text-red-600">{proposal.error}</p> : null}
                            </div>
                            {proposal.unreadable ? (
                                <button
                                    type="button"
                                    className="rounded-md border border-foreground/20 px-2 py-1 text-xs"
                                    disabled={busy}
                                    onClick={() =>
                                        void run(async () => {
                                            await deleteProposal(proposal.proposal_id);
                                            setSummaries((current) =>
                                                current.filter((item) => item.proposal_id !== proposal.proposal_id),
                                            );
                                        })
                                    }
                                >
                                    Delete
                                </button>
                            ) : (
                                <div className="flex gap-2">
                                    <button
                                        type="button"
                                        className="rounded-md border border-foreground/20 px-2 py-1 text-xs"
                                        disabled={busy}
                                        onClick={() =>
                                            void run(async () => {
                                                const result = await acceptAll(proposal.proposal_id);
                                                setSkipped((current) => ({
                                                    ...current,
                                                    [proposal.proposal_id]: result.skipped,
                                                }));
                                                await reloadOne(proposal.proposal_id, setSummaries, setDiffs);
                                            })
                                        }
                                    >
                                        Accept all
                                    </button>
                                    <button
                                        type="button"
                                        className="rounded-md border border-foreground/20 px-2 py-1 text-xs"
                                        disabled={busy}
                                        onClick={() =>
                                            void run(async () => {
                                                await rejectAll(proposal.proposal_id);
                                                await reloadOne(proposal.proposal_id, setSummaries, setDiffs);
                                            })
                                        }
                                    >
                                        Reject all
                                    </button>
                                </div>
                            )}
                        </header>
                        {skipped[proposal.proposal_id]?.length ? (
                            <p className="text-xs text-amber-700 dark:text-amber-400">
                                Skipped{" "}
                                {skipped[proposal.proposal_id]
                                    .map((op) => `${op.node_id} (${op.reason})`)
                                    .join(", ")}
                            </p>
                        ) : null}
                        <ul className="flex flex-col gap-1">
                            {proposalRows(diffs[proposal.proposal_id] ?? emptyDiff(proposal)).map((row) => (
                                <li
                                    key={row.key}
                                    className={clsx(
                                        "flex flex-wrap items-center justify-between gap-2 rounded-md border px-2 py-1",
                                        row.stale
                                            ? "border-amber-600/70 bg-amber-500/10"
                                            : "border-foreground/15",
                                    )}
                                >
                                    <div className="min-w-0">
                                        <p>
                                            {row.summary}
                                            {row.stale ? (
                                                <span className="ml-2 text-xs uppercase tracking-wide text-amber-700 dark:text-amber-400">
                                                    Stale
                                                </span>
                                            ) : null}
                                            {row.status !== "pending" ? (
                                                <span className="ml-2 text-xs text-foreground/60">{row.status}</span>
                                            ) : null}
                                        </p>
                                        {row.detail ? (
                                            <p className="truncate text-xs text-foreground/70">{row.detail}</p>
                                        ) : null}
                                    </div>
                                    {row.status === "pending" ? (
                                        <div className="flex gap-2">
                                            {row.canAccept ? (
                                                <button
                                                    type="button"
                                                    className="rounded-md border border-foreground/20 px-2 py-1 text-xs"
                                                    disabled={busy}
                                                    onClick={() =>
                                                        void run(async () => {
                                                            await acceptOperation(
                                                                proposal.proposal_id,
                                                                row.index,
                                                                row.needsConfirm,
                                                            );
                                                            await reloadOne(proposal.proposal_id, setSummaries, setDiffs);
                                                        })
                                                    }
                                                >
                                                    {row.needsConfirm ? "Apply over newer edit" : "Accept"}
                                                </button>
                                            ) : null}
                                            <button
                                                type="button"
                                                className="rounded-md border border-foreground/20 px-2 py-1 text-xs"
                                                disabled={busy}
                                                onClick={() =>
                                                    void run(async () => {
                                                        await rejectOperation(proposal.proposal_id, row.index);
                                                        await reloadOne(proposal.proposal_id, setSummaries, setDiffs);
                                                    })
                                                }
                                            >
                                                Reject
                                            </button>
                                        </div>
                                    ) : null}
                                </li>
                            ))}
                        </ul>
                    </section>
                ))}
            </div>
        </aside>
    );
}

function emptyDiff(proposal: ProposalSummary): ProposalDiff {
    return {
        proposal_id: proposal.proposal_id,
        created_at: proposal.created_at ?? "",
        source: proposal.source ?? "",
        summary: proposal.summary,
        operations: [],
    };
}

async function reloadOne(
    id: string,
    setSummaries: (value: ProposalSummary[] | ((current: ProposalSummary[]) => ProposalSummary[])) => void,
    setDiffs: (value: Record<string, ProposalDiff> | ((current: Record<string, ProposalDiff>) => Record<string, ProposalDiff>)) => void,
) {
    const { proposals } = await listProposals();
    setSummaries(proposals);
    const still = proposals.find((proposal) => proposal.proposal_id === id && !proposal.unreadable);
    if (!still) {
        setDiffs((current) => {
            const next = { ...current };
            delete next[id];
            return next;
        });
        return;
    }
    const diff = await getProposal(id);
    setDiffs((current) => ({ ...current, [id]: diff }));
}
