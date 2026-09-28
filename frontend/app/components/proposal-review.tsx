import { Fragment, useEffect, useState } from "react";
import clsx from "clsx";
import { ChevronLeft, ListChecks, XCircle } from "lucide-react";
import {
    acceptAll,
    acceptOperation,
    deleteProposal,
    getProposal,
    listProposals,
    rejectAll,
    rejectOperation,
    type AcceptAllResult,
    type OpDiff,
    type ProposalDiff,
    type ProposalSummary,
} from "../api";
import { detailCanAccept, detailNeedsConfirm, proposalDetailBlocks } from "../proposal-detail";
import { proposalRows } from "../proposal-rows";
import { RightRailPanel } from "./right-rail-panel";
import { ReviewOpActions } from "./review-op-actions";

type DetailSelection = { proposalId: string; index: number };

export function ProposalReview({
    refreshKey,
    onFocus,
}: {
    refreshKey: number;
    onFocus?: () => void;
}) {
    const [summaries, setSummaries] = useState<ProposalSummary[]>([]);
    const [diffs, setDiffs] = useState<Record<string, ProposalDiff>>({});
    const [skipped, setSkipped] = useState<Record<string, AcceptAllResult["skipped"]>>({});
    const [detail, setDetail] = useState<DetailSelection | null>(null);
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

    useEffect(() => {
        setDetail(null);
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

    const detailOp = detail ? findOp(diffs, detail) : null;

    return (
        <RightRailPanel
            onPointerDown={() => {
                onFocus?.();
            }}
        >
            {detail && detailOp ? (
                <div className="animate-in slide-in-from-right-4 flex min-h-0 flex-col gap-3 duration-200">
                    <div className="flex items-center gap-2">
                        <button
                            type="button"
                            className="inline-flex items-center gap-1 text-xs text-foreground/80"
                            onClick={() => setDetail(null)}
                        >
                            <ChevronLeft className="size-4" aria-hidden />
                            Review
                        </button>
                    </div>
                    <div className="flex min-h-0 flex-col gap-2 overflow-y-auto">
                        {proposalDetailBlocks(detailOp).map((block, blockIndex) => (
                            <DetailBlockView key={blockIndex} block={block} />
                        ))}
                    </div>
                    {detailOp.status === "pending" ? (
                        <ReviewOpActions
                            canAccept={detailCanAccept(detailOp)}
                            needsConfirm={detailNeedsConfirm(detailOp)}
                            busy={busy}
                            onAccept={() =>
                                void run(async () => {
                                    await acceptOperation(detail.proposalId, detail.index, detailNeedsConfirm(detailOp));
                                    setDetail(null);
                                    await reloadOne(detail.proposalId, setSummaries, setDiffs);
                                })
                            }
                            onReject={() =>
                                void run(async () => {
                                    await rejectOperation(detail.proposalId, detail.index);
                                    setDetail(null);
                                    await reloadOne(detail.proposalId, setSummaries, setDiffs);
                                })
                            }
                        />
                    ) : null}
                </div>
            ) : (
                <>
                    <div className="flex items-center justify-between" onPointerDown={() => onFocus?.()}>
                        <h2 className="font-semibold">Review</h2>
                        <span className="text-xs text-foreground/60">{summaries.length}</span>
                    </div>
                    {error ? <p className="text-red-600">{error}</p> : null}
                    <div className="flex min-h-0 flex-col gap-3">
                        {summaries.map((proposal, index) => (
                            <Fragment key={proposal.proposal_id}>
                                {index > 0 ? <hr className="my-2 border-foreground/15" /> : null}
                                <section className="flex flex-col gap-2">
                                    <header className="flex flex-wrap items-baseline justify-between gap-2">
                                        <div className="min-w-0 flex-1">
                                            <p className="font-medium">
                                                {proposal.unreadable
                                                    ? `Unreadable ${proposal.proposal_id}`
                                                    : proposal.summary || `${proposal.source} ${proposal.proposal_id}`}
                                            </p>
                                            {proposal.error ? (
                                                <p className="text-xs text-red-600">{proposal.error}</p>
                                            ) : null}
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
                                                            current.filter(
                                                                (item) => item.proposal_id !== proposal.proposal_id,
                                                            ),
                                                        );
                                                    })
                                                }
                                            >
                                                Delete
                                            </button>
                                        ) : (
                                            <div className="flex gap-1">
                                                <button
                                                    type="button"
                                                    className="inline-flex size-7 items-center justify-center rounded-md text-emerald-600 hover:bg-foreground/10"
                                                    disabled={busy}
                                                    aria-label="Accept all"
                                                    title="Accept all"
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
                                                    <ListChecks className="size-4" aria-hidden />
                                                </button>
                                                <button
                                                    type="button"
                                                    className="inline-flex size-7 items-center justify-center rounded-md text-red-600 hover:bg-foreground/10"
                                                    disabled={busy}
                                                    aria-label="Reject all"
                                                    title="Reject all"
                                                    onClick={() =>
                                                        void run(async () => {
                                                            await rejectAll(proposal.proposal_id);
                                                            await reloadOne(proposal.proposal_id, setSummaries, setDiffs);
                                                        })
                                                    }
                                                >
                                                    <XCircle className="size-4" aria-hidden />
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
                                        {proposalRows(diffs[proposal.proposal_id] ?? emptyDiff(proposal)).map(
                                            (row) => (
                                                <li
                                                    key={row.key}
                                                    className={clsx(
                                                        "relative rounded-md border px-2 py-1 pr-16 transition-colors",
                                                        row.stale
                                                            ? "border-amber-600/70 bg-amber-500/10"
                                                            : "border-foreground/15",
                                                    )}
                                                >
                                                    <button
                                                        type="button"
                                                        className="w-full cursor-pointer text-left hover:opacity-90"
                                                        onClick={() => {
                                                            onFocus?.();
                                                            setDetail({
                                                                proposalId: proposal.proposal_id,
                                                                index: row.index,
                                                            });
                                                        }}
                                                    >
                                                        <p>
                                                            {row.summary}
                                                            {row.stale ? (
                                                                <span
                                                                    className="ml-2 text-xs uppercase tracking-wide text-amber-700 dark:text-amber-400"
                                                                >
                                                                    Stale
                                                                </span>
                                                            ) : null}
                                                            {row.status !== "pending" ? (
                                                                <span className="ml-2 text-xs text-foreground/60">
                                                                    {row.status}
                                                                </span>
                                                            ) : null}
                                                        </p>
                                                        {row.detail ? (
                                                            <p className="truncate text-xs text-foreground/70">
                                                                {row.detail}
                                                            </p>
                                                        ) : null}
                                                    </button>
                                                    {row.status === "pending" ? (
                                                        <ReviewOpActions
                                                            className="absolute top-1 right-1"
                                                            canAccept={row.canAccept}
                                                            needsConfirm={row.needsConfirm}
                                                            busy={busy}
                                                            onAccept={() =>
                                                                void run(async () => {
                                                                    await acceptOperation(
                                                                        proposal.proposal_id,
                                                                        row.index,
                                                                        row.needsConfirm,
                                                                    );
                                                                    await reloadOne(
                                                                        proposal.proposal_id,
                                                                        setSummaries,
                                                                        setDiffs,
                                                                    );
                                                                })
                                                            }
                                                            onReject={() =>
                                                                void run(async () => {
                                                                    await rejectOperation(
                                                                        proposal.proposal_id,
                                                                        row.index,
                                                                    );
                                                                    await reloadOne(
                                                                        proposal.proposal_id,
                                                                        setSummaries,
                                                                        setDiffs,
                                                                    );
                                                                })
                                                            }
                                                        />
                                                    ) : null}
                                                </li>
                                            ),
                                        )}
                                    </ul>
                                </section>
                            </Fragment>
                        ))}
                    </div>
                </>
            )}
        </RightRailPanel>
    );
}

function DetailBlockView({ block }: { block: ReturnType<typeof proposalDetailBlocks>[number] }) {
    if (block.kind === "heading") {
        return <h3 className="text-sm font-semibold">{block.text}</h3>;
    }
    if (block.kind === "line") {
        return <p className="text-xs text-foreground/80">{block.text}</p>;
    }
    if (block.kind === "pair") {
        return (
            <div className="text-xs">
                <p className="font-medium text-foreground/70">{block.label}</p>
                <p className="text-foreground/60">{block.before}</p>
                <p className="text-foreground">→ {block.after}</p>
            </div>
        );
    }
    return (
        <div className="text-xs">
            <p className="font-medium text-foreground/70">{block.label}</p>
            {block.before ? (
                <pre className="mt-1 max-h-32 overflow-auto rounded border border-foreground/15 bg-foreground/5 p-2 font-mono text-[11px] whitespace-pre-wrap">
                    {block.before}
                </pre>
            ) : null}
            <pre className="mt-1 max-h-48 overflow-auto rounded border border-foreground/15 bg-foreground/5 p-2 font-mono text-[11px] whitespace-pre-wrap">
                {block.after}
            </pre>
        </div>
    );
}

function findOp(diffs: Record<string, ProposalDiff>, detail: DetailSelection): OpDiff | null {
    const proposal = diffs[detail.proposalId];
    if (!proposal) return null;
    return proposal.operations.find((op) => op.index === detail.index) ?? null;
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
