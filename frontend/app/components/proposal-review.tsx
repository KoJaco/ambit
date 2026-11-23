import { useEffect, useLayoutEffect, useRef, useState, type ReactNode, type Ref } from "react";
import clsx from "clsx";
import { Check, ChevronLeft, ListChecks, TriangleAlert, XCircle } from "lucide-react";
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
import { detailCanAccept, detailNames, detailNeedsConfirm, proposalDetailBlocks, proposalDetailHead, proposalDetailView, unresolvedNodeIds } from "../proposal-detail";
import { isEditableElement, reviewEscapeAction } from "../right-rail-state";
import { useNodeSnapshots } from "../use-node-snapshots";
import { ProposalVisual } from "./proposal-visuals";
import { history } from "../history";
import { acceptedEntry } from "../proposal-history";
import {
    hierarchySections,
    proposalOrigin,
    proposalRows,
    summarizeProposal,
    unresolvedParentIds,
    unresolvedRowIds,
    type HierarchyItem,
    type ReviewRow,
    type RowVerb,
} from "../proposal-rows";
import { PanelHeaderActions, RightRailPanel } from "./right-rail-panel";
import { ReviewOpActions } from "./review-op-actions";

type DetailSelection = { proposalId: string; index: number };

export function ProposalReview({
    refreshKey,
    onFocus,
    open = true,
    onClose,
    onPendingChange,
}: {
    refreshKey: number;
    onFocus?: () => void;
    // When false the panel is hidden but proposals keep loading so the pending count stays current.
    open?: boolean;
    onClose?: () => void;
    // Ids of proposals that still have pending operations; called after each load.
    onPendingChange?: (ids: string[]) => void;
}) {
    const [summaries, setSummaries] = useState<ProposalSummary[]>([]);
    const [diffs, setDiffs] = useState<Record<string, ProposalDiff>>({});
    const [skipped, setSkipped] = useState<Record<string, AcceptAllResult["skipped"]>>({});
    const [detail, setDetail] = useState<DetailSelection | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [busy, setBusy] = useState(false);
    // Proposals whose per-change accept/reject controls are shown; collapsed by default.
    const [individual, setIndividual] = useState<Record<string, boolean>>({});
    const [loaded, setLoaded] = useState(false);
    // Row that opened the detail view, so Back can return focus to it.
    const openedFrom = useRef<string | null>(null);
    const detailHeading = useRef<HTMLHeadingElement>(null);
    const listRef = useRef<HTMLDivElement>(null);
    const detailKey = detail ? `${detail.proposalId}:${detail.index}` : null;
    const wasDetail = useRef(false);
    const pendingCallback = useRef(onPendingChange);
    pendingCallback.current = onPendingChange;

    useEffect(() => {
        if (!loaded) return;
        const ids = summaries
            .filter((proposal) => !proposal.unreadable)
            .filter((proposal) => diffs[proposal.proposal_id]?.operations.some((op) => op.status === "pending"))
            .map((proposal) => proposal.proposal_id);
        pendingCallback.current?.(ids);
    }, [loaded, summaries, diffs]);

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
                setLoaded(true);
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

    // Accepting is what changes the model, so it is what undo has to reverse. An accept that
    // cannot be reversed exactly clears history rather than leave steps that no longer fit.
    function recordAcceptance(summary: string | undefined, ops: OpDiff[], applied: number[]) {
        const done = ops.filter((op) => applied.includes(op.index));
        if (done.length === 0) return;
        const entry = acceptedEntry(summary || "accepted proposal", done);
        if (entry) history.record(entry);
        else history.clear();
    }

    async function acceptRows(proposal: ProposalSummary, diff: ProposalDiff | undefined, rows: ReviewRow[], label: string) {
        await run(async () => {
            const done: number[] = [];
            for (const row of rows) {
                if (row.status !== "pending" || !row.canAccept) continue;
                await acceptOperation(proposal.proposal_id, row.index, row.needsConfirm);
                done.push(row.index);
            }
            recordAcceptance(label, diff?.operations ?? [], done);
            await reloadOne(proposal.proposal_id, setSummaries, setDiffs);
        });
    }

    async function rejectRows(proposalId: string, rows: ReviewRow[]) {
        await run(async () => {
            for (const row of rows) {
                if (row.status === "pending") await rejectOperation(proposalId, row.index);
            }
            await reloadOne(proposalId, setSummaries, setDiffs);
        });
    }

    function renderRow(proposalId: string, diff: ProposalDiff | undefined, { row, label }: HierarchyItem, showActions: boolean): ReactNode {
        const proposal = summaries.find((item) => item.proposal_id === proposalId);
        return (
            <li
                key={row.key}
                className="group relative -mx-2 border-b border-border/60 px-2 transition-colors last:border-b-0 hover:bg-accent/50 focus-within:bg-accent/50"
            >
                <button
                    type="button"
                    className={clsx("flex w-full cursor-pointer items-start gap-2.5 py-2 text-left focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring", showActions && row.status === "pending" && "pr-[4.5rem]")}
                    data-row-key={`${proposalId}:${row.key}`}
                    onClick={() => {
                        onFocus?.();
                        openedFrom.current = `${proposalId}:${row.key}`;
                        setDetail({ proposalId, index: row.index });
                    }}
                >
                    <VerbChip verb={row.verb} done={row.status !== "pending"} />
                    <span className="flex min-w-0 flex-1 flex-col">
                        <span className="flex min-w-0 items-center gap-2 text-sm font-medium">
                            <span className="truncate" title={label}>{label}</span>
                            {row.stale ? (
                                <span className="shrink-0 rounded-full bg-amber-500/20 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-amber-800 dark:text-amber-300">
                                    Stale
                                </span>
                            ) : null}
                            {row.status !== "pending" ? (
                                <span className="shrink-0 text-xs font-normal text-muted-foreground">{row.status}</span>
                            ) : null}
                        </span>
                        {row.change ? <span className="truncate text-xs text-muted-foreground" title={row.change}>{row.change}</span> : null}
                    </span>
                </button>
                {showActions && row.status === "pending" ? (
                    <ReviewOpActions
                        className="absolute top-1.5 right-2"
                        canAccept={row.canAccept}
                        needsConfirm={row.needsConfirm}
                        busy={busy}
                        label={label}
                        onAccept={() => void acceptRows(proposal ?? emptySummary(proposalId), diff, [row], row.title)}
                        onReject={() => void rejectRows(proposalId, [row])}
                    />
                ) : null}
            </li>
        );
    }

    // Hooks must run on every render, so this sits above the early return.
    const detailOp = detail ? findOp(diffs, detail) : null;
    const fetchedNodes = useNodeSnapshots((known) =>
        detail && detailOp && diffs[detail.proposalId]
            ? unresolvedNodeIds(diffs[detail.proposalId], detailOp, known)
            : Object.values(diffs).flatMap((d) => [...unresolvedParentIds(d, known), ...unresolvedRowIds(d, known)]),
    );

    const fetchedNodeNames = new Map([...fetchedNodes].map(([id, node]) => [id, node.name]));

    // Focus follows the view: heading on entering detail, the opening row (or list heading) on leaving it.
    useLayoutEffect(() => {
        if (detailKey) {
            wasDetail.current = true;
            detailHeading.current?.focus();
            return;
        }
        if (!wasDetail.current) return;
        wasDetail.current = false;
        const key = openedFrom.current;
        const row = key ? Array.from(listRef.current?.querySelectorAll<HTMLElement>("[data-row-key]") ?? []).find((el) => el.dataset.rowKey === key) : null;
        (row ?? document.getElementById("review-heading"))?.focus();
    }, [detailKey]);

    if (!open || (summaries.length === 0 && !error)) return null;

    return (
        <RightRailPanel
            label="Review"
            onPointerDown={() => {
                onFocus?.();
            }}
            onKeyDown={(event) => {
                if (event.key !== "Escape" || event.defaultPrevented) return;
                const action = reviewEscapeAction({ detailOpen: !!detail, editingFocus: isEditableElement(event.target as HTMLElement) });
                if (action === "none") return;
                event.stopPropagation();
                if (action === "back") setDetail(null);
                else onClose?.();
            }}
        >
            {detail && detailOp ? (
                <div className="animate-in slide-in-from-right-4 flex min-h-0 flex-col gap-3 duration-200">
                    <div className="flex items-center gap-2">
                        <button
                            type="button"
                            className="inline-flex min-h-6 items-center gap-1 rounded text-xs text-foreground/80 focus-visible:outline-2 focus-visible:outline-ring"
                            onClick={() => setDetail(null)}
                        >
                            <ChevronLeft className="size-4" aria-hidden />
                            Review
                        </button>
                        <span className="ml-auto">
                            <PanelHeaderActions />
                        </span>
                    </div>
                    <div className="flex min-h-0 flex-col gap-2 overflow-y-auto">
                        {(() => {
                            const names = detailNames(diffs[detail.proposalId], fetchedNodes);
                            const head = proposalDetailHead(detailOp, names);
                            const rest = proposalDetailBlocks(detailOp, names).slice(head.length);
                            const view = proposalDetailView(diffs[detail.proposalId], detailOp, fetchedNodes);
                            return (
                                <>
                                    {head.map((block, i) => (
                                        <DetailBlockView key={i} block={block} headingRef={i === 0 ? detailHeading : undefined} />
                                    ))}
                                    <ProposalVisual view={view} />
                                    <details className="text-xs">
                                        <summary className="cursor-pointer rounded text-foreground/80 focus-visible:outline-2 focus-visible:outline-ring">
                                            Full details
                                        </summary>
                                        <div className="mt-2 flex flex-col gap-2">
                                            {rest.map((block, i) => (
                                                <DetailBlockView key={i} block={block} />
                                            ))}
                                        </div>
                                    </details>
                                </>
                            );
                        })()}
                    </div>
                    {detailOp.status === "pending" ? (
                        <ReviewOpActions
                            label={proposalRows({ ...diffs[detail.proposalId], operations: [detailOp] }, fetchedNodeNames)[0]?.title ?? detailOp.node_id}
                            canAccept={detailCanAccept(detailOp)}
                            needsConfirm={detailNeedsConfirm(detailOp)}
                            busy={busy}
                            onAccept={() =>
                                void run(async () => {
                                    await acceptOperation(detail.proposalId, detail.index, detailNeedsConfirm(detailOp));
                                    recordAcceptance(diffs[detail.proposalId]?.summary, [detailOp], [detail.index]);
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
                    <div className="flex shrink-0 items-center justify-between" onPointerDown={() => onFocus?.()}>
                        <h2 id="review-heading" tabIndex={-1} className="flex items-center gap-2 font-semibold outline-none focus-visible:outline-2 focus-visible:outline-ring">
                            Review
                            <span className={clsx("rounded-full bg-primary/10 px-2 py-0.5 text-[11px] font-medium", accentText)}>
                                {summaries.length} {summaries.length === 1 ? "proposal" : "proposals"}
                            </span>
                        </h2>
                        <PanelHeaderActions onClose={onClose} closeLabel="Close Review" />
                    </div>
                    {error ? <p className="shrink-0 text-destructive">{error}</p> : null}
                    <div ref={listRef} className="-mx-2 flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-2 pb-1">
                        {summaries.map((proposal) => {
                            const diff = diffs[proposal.proposal_id];
                            const rows = proposalRows(diff ?? emptyDiff(proposal), fetchedNodeNames);
                            const pending = rows.filter((row) => row.status === "pending").length;
                            const origin = proposalOrigin(proposal.source);
                            const line = summarizeProposal(diff ?? emptyDiff(proposal));
                            const sections = hierarchySections(diff ?? emptyDiff(proposal), fetchedNodeNames);
                            const showIndividual = !!individual[proposal.proposal_id];
                            return (
                                <section
                                    key={proposal.proposal_id}
                                    aria-labelledby={`proposal-title-${proposal.proposal_id}`}
                                    className="rounded-2xl border border-border bg-background/40"
                                >
                                    <header className="sticky top-0 z-10 flex flex-col gap-2.5 rounded-t-2xl border-b border-border bg-card/95 p-3 backdrop-blur">
                                        <div className="flex flex-col gap-1">
                                            <h3 id={`proposal-title-${proposal.proposal_id}`} className="text-sm leading-snug font-semibold">
                                                {proposal.unreadable
                                                    ? `Unreadable ${proposal.proposal_id}`
                                                    : proposal.summary || `${proposal.source} ${proposal.proposal_id}`}
                                            </h3>
                                            {proposal.error ? (
                                                <p className="text-xs text-destructive">{proposal.error}</p>
                                            ) : (
                                                <div className="flex flex-col gap-1">
                                                    <p className="text-sm font-medium text-foreground">{line.headline}</p>
                                                    <p className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                                                        <span
                                                            className={clsx(
                                                                "rounded-full px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide",
                                                                origin === "You" ? "bg-muted" : clsx("bg-primary/10", accentText),
                                                            )}
                                                        >
                                                            {origin}
                                                        </span>
                                                        <span>{line.source}</span>
                                                    </p>
                                                    {line.staleCount > 0 ? (
                                                        <p className="flex items-center gap-1.5 text-xs font-medium text-amber-800 dark:text-amber-300">
                                                            <TriangleAlert className="size-3.5 shrink-0" aria-hidden />
                                                            {line.staleCount} out of date: the model changed since this was proposed
                                                        </p>
                                                    ) : null}
                                                </div>
                                            )}
                                        </div>
                                        {proposal.unreadable ? (
                                            <button
                                                type="button"
                                                className={outlineButton}
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
                                                    className={primaryButton}
                                                    disabled={busy || pending === 0}
                                                    onClick={() =>
                                                        void run(async () => {
                                                            const ops = diff?.operations.filter((op) => op.status === "pending") ?? [];
                                                            const result = await acceptAll(proposal.proposal_id);
                                                            setSkipped((current) => ({
                                                                ...current,
                                                                [proposal.proposal_id]: result.skipped,
                                                            }));
                                                            recordAcceptance(proposal.summary, ops, result.applied);
                                                            await reloadOne(proposal.proposal_id, setSummaries, setDiffs);
                                                        })
                                                    }
                                                >
                                                    <ListChecks className="size-4" aria-hidden />
                                                    Accept all
                                                </button>
                                                <button
                                                    type="button"
                                                    className={outlineButton}
                                                    aria-label={`Reject all changes in ${proposal.summary || proposal.proposal_id}`}
                                                    disabled={busy || pending === 0}
                                                    onClick={() =>
                                                        void run(async () => {
                                                            await rejectAll(proposal.proposal_id);
                                                            await reloadOne(proposal.proposal_id, setSummaries, setDiffs);
                                                        })
                                                    }
                                                >
                                                    <XCircle className="size-4" aria-hidden />
                                                    Reject all
                                                </button>
                                            </div>
                                        )}
                                    </header>
                                    {skipped[proposal.proposal_id]?.length ? (
                                        <p className="px-3 pt-2 text-xs text-amber-700 dark:text-amber-400">
                                            Skipped{" "}
                                            {skipped[proposal.proposal_id].map((op) => `${op.node_id} (${op.reason})`).join(", ")}
                                        </p>
                                    ) : null}
                                    {proposal.unreadable ? null : (
                                        <div className="flex flex-col gap-2 p-2">
                                            <button
                                                type="button"
                                                aria-pressed={showIndividual}
                                                className={clsx(
                                                    "inline-flex w-fit items-center gap-1 rounded-md border px-1.5 py-1 text-xs font-medium hover:bg-accent focus-visible:outline-2 focus-visible:outline-ring",
                                                    showIndividual ? "border-foreground/70 bg-accent text-foreground" : "border-transparent text-foreground/80",
                                                )}
                                                onClick={() =>
                                                    setIndividual((current) => ({
                                                        ...current,
                                                        [proposal.proposal_id]: !current[proposal.proposal_id],
                                                    }))
                                                }
                                            >
                                                {showIndividual ? <Check className="size-4" aria-hidden /> : <ListChecks className="size-4" aria-hidden />}
                                                Review individually
                                            </button>
                                            <div className="flex flex-col gap-3">
                                                {sections.map((section) => (
                                                    <section key={section.key} aria-labelledby={`sec-${proposal.proposal_id}-${section.key}`} className="flex flex-col gap-1.5">
                                                        <h4 id={`sec-${proposal.proposal_id}-${section.key}`} className="text-xs font-semibold text-muted-foreground">{section.title}</h4>
                                                        <ul className="flex flex-col">
                                                            {section.items.map((item) =>
                                                                renderRow(proposal.proposal_id, diff, item, showIndividual),
                                                            )}
                                                        </ul>
                                                    </section>
                                                ))}
                                            </div>
                                        </div>
                                    )}
                                </section>
                            );
                        })}
                    </div>
                </>
            )}
        </RightRailPanel>
    );
}

// Accent text that holds 4.5:1 on the primary tint in light mode; the dark primary already does.
const accentText = "text-[color-mix(in_oklab,var(--primary)_75%,black)] dark:text-primary";

const buttonBase =
    "inline-flex min-h-8 flex-1 items-center justify-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-50";
const primaryButton = clsx(buttonBase, "bg-primary text-primary-foreground hover:opacity-90");
const outlineButton = clsx(buttonBase, "border border-border bg-card hover:bg-accent");

const VERB_STYLE: Record<RowVerb, string> = {
    Create: "bg-emerald-500/15 text-emerald-800 dark:text-emerald-300",
    Update: "bg-sky-500/15 text-sky-800 dark:text-sky-300",
    Delete: "bg-red-500/15 text-red-800 dark:text-red-300",
    Connect: "bg-violet-500/15 text-violet-800 dark:text-violet-300",
    "Remove connection": "bg-red-500/15 text-red-800 dark:text-red-300",
    Other: "bg-muted text-muted-foreground",
};

function VerbChip({ verb, done }: { verb: RowVerb; done?: boolean }) {
    return (
        <span
            className={clsx(
                "mt-0.5 inline-flex min-w-[3.75rem] shrink-0 text-center justify-center rounded-full px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide",
                VERB_STYLE[verb],
                done && "opacity-50",
            )}
        >
            {verb}
        </span>
    );
}

function emptySummary(id: string): ProposalSummary {
    return { proposal_id: id, unreadable: false };
}

function DetailBlockView({
    block,
    headingRef,
}: {
    block: ReturnType<typeof proposalDetailBlocks>[number];
    headingRef?: Ref<HTMLHeadingElement>;
}) {
    if (block.kind === "heading") {
        return (
            <h3 ref={headingRef} tabIndex={headingRef ? -1 : undefined} className="text-sm font-semibold outline-none focus-visible:outline-2 focus-visible:outline-ring">
                {block.text}
            </h3>
        );
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
