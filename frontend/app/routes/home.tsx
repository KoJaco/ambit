import type { Route } from "./+types/home";
import { Sidebar } from "../components/Sidebar";
import FlowCanvas from "../components/FlowCanvas";
import NodeInspector from "../components/node-inspector";
import { ControlBar } from "../components/ControlBar";
import type { ControlBarTool } from "../components/types";
import clsx from "clsx";
import { useEffect, useMemo, useRef, useState } from "react";
import { ModeToggle } from "~/components/ui/mode-toggle";
import { ReactFlowProvider } from "@xyflow/react";
import { UIProvider } from "../components/ui-context";
import { useParams, useLocation, useNavigate } from "react-router";
import { subscribeEvents } from "../api";
import { Breadcrumbs } from "../components/Breadcrumbs";
import ScreenSizeAlert from "../components/ScreenSizeAlert";
import { ProposalReview } from "../components/proposal-review";
import { RightRailStack, type RightRailFront } from "../components/right-rail-stack";
import { EdgeActions } from "../components/edge-actions";
import type { ProposalDiff, Relationship } from "../api";
import type { CanvasPreview } from "../ghost-preview";
import { returnNodeIdAfterDelete } from "../relationship-breadcrumb";
import {
    floatingToggleMode,
    isEditableElement,
    isInspectorDirty,
    shouldAutoOpenReview,
    useRailState,
} from "../right-rail-state";
import { history, isOutsideChange, useHistory } from "../history";
import { useDesktopViewport } from "../hooks/use-desktop-viewport";

export function meta({}: Route.MetaArgs) {
    return [
        { title: "ambit" },
        { name: "description", content: "Architecture canvas" },
    ];
}

export default function Home() {
    const desktopViewport = useDesktopViewport();
    const { nodeId, relationshipId } = useParams();
    const location = useLocation();
    const navigate = useNavigate();
    const [tool, setTool] = useState<ControlBarTool>("grab");
    const [refreshKey, setRefreshKey] = useState(0);
    const [proposalRefreshKey, setProposalRefreshKey] = useState(0);
    const [selectedEdge, setSelectedEdge] = useState<Relationship | null>(null);
    const toggleMode = floatingToggleMode(useRailState());
    useEffect(
        () =>
            subscribeEvents({
                onModelChanged: () => {
                    // A change we did not make invalidates every recorded undo step.
                    if (isOutsideChange()) history.clear();
                    setRefreshKey((value) => value + 1);
                },
                onIntegrityChanged: () => setRefreshKey((value) => value + 1),
                onProposalsChanged: () => setProposalRefreshKey((value) => value + 1),
            }),
        [],
    );
    const [selectedId, setSelectedId] = useState<string | null>(null);
    // Bumped by Delete/Backspace; the open inspector panel turns it into its confirmation.
    const [deleteRequest, setDeleteRequest] = useState(0);
    const historyView = useHistory();
    const [historyEpoch, setHistoryEpoch] = useState(0);
    const [historyNote, setHistoryNote] = useState<string | null>(null);
    useEffect(() => {
        if (!historyNote) return;
        const timer = window.setTimeout(() => setHistoryNote(null), 3000);
        return () => window.clearTimeout(timer);
    }, [historyNote]);
    async function stepHistory(dir: "undo" | "redo") {
        try {
            const label = await history[dir]();
            if (!label) return;
            setHistoryNote(`${dir === "undo" ? "Undid" : "Redid"}: ${label}`);
        } catch (err) {
            history.clear();
            setHistoryNote(`Could not ${dir}: ${err instanceof Error ? err.message : "request failed"}. History cleared.`);
        }
        // Reload what the steps touched; the inspector remounts so it shows the restored values.
        setHistoryEpoch((value) => value + 1);
        setRefreshKey((value) => value + 1);
    }
    const [frontPanel, setFrontPanel] = useState<RightRailFront>("review");
    // Whether the Review panel is shown. Closing it keeps the proposals; the toolbar badge reopens it.
    const [reviewOpen, setReviewOpen] = useState(true);
    function showReview() {
        setReviewOpen(true);
        setFrontPanel("review");
    }
    // Ghost preview on the canvas: proposals the Review panel has switched on, the element a row
    // or ghost is pointing at, and a request from a ghost to open an item in the panel.
    const [previewDiffs, setPreviewDiffs] = useState<ProposalDiff[]>([]);
    const [highlightKey, setHighlightKey] = useState<string | null>(null);
    const [openItem, setOpenItem] = useState<{ proposalId: string; index: number; nonce: number } | null>(null);
    const canvasPreview = useMemo<CanvasPreview>(
        () => ({
            diffs: previewDiffs,
            highlightKey,
            onHover: setHighlightKey,
            onActivate: (proposalId, index) => {
                setOpenItem((current) => ({ proposalId, index, nonce: (current?.nonce ?? 0) + 1 }));
                showReview();
            },
        }),
        [previewDiffs, highlightKey], // eslint-disable-line react-hooks/exhaustive-deps
    );
    const [pendingReviewIds, setPendingReviewIds] = useState<string[]>([]);
    // Proposals already offered to the user. null until the first load, so proposals that were
    // waiting when the page opened do not count as new.
    const seenProposals = useRef<Set<string> | null>(null);
    function onPendingReview(ids: string[]) {
        setPendingReviewIds(ids);
        if (seenProposals.current === null) {
            seenProposals.current = new Set(ids);
            return;
        }
        const open = shouldAutoOpenReview({
            pendingIds: ids,
            seenIds: seenProposals.current,
            editingFocus: isEditableElement(document.activeElement),
            inspectorDirty: isInspectorDirty(),
        });
        ids.forEach((id) => seenProposals.current?.add(id));
        if (open) showReview();
    }
    const recenterRef = useRef<null | (() => void)>(null);
    const zoomRef = useRef<{ zoomIn: () => void; zoomOut: () => void } | null>(
        null
    );

    useEffect(() => {
        setSelectedId(null);
        setSelectedEdge(null);
    }, [nodeId, relationshipId]);

    if (!desktopViewport) {
        return <ScreenSizeAlert />;
    }

    return (
        <div className="fixed top-0 left-0 h-dvh w-full overflow-hidden bg-background">
            <UIProvider>
                <ReactFlowProvider>
                    <FlowCanvas
                        nodeId={nodeId}
                        relationshipId={relationshipId}
                        refreshKey={refreshKey}
                        preview={canvasPreview}
                        tool={tool === "grab" ? "grab" : "pointer"}
                        onSelectNode={(id) => {
                            setSelectedId(id);
                            if (id === null) return;
                            setSelectedEdge(null);
                            setFrontPanel("inspector");
                        }}
                        onSelectEdge={(edge) => {
                            setSelectedEdge(edge);
                            setSelectedId(null);
                            if (edge) setFrontPanel("inspector");
                        }}
                        onProposalStaged={() => {
                            setProposalRefreshKey((value) => value + 1);
                            showReview();
                        }}
                        onRequestRecenterRef={(fn) => {
                            recenterRef.current = fn;
                        }}
                        onRequestZoomRef={(api) => {
                            zoomRef.current = api;
                        }}
                    />
                </ReactFlowProvider>
                <div className="pointer-events-none absolute top-0 left-0 z-40 flex h-full w-full flex-col gap-4 p-4">
                    <header className="flex h-8 shrink-0 items-center justify-between">
                        <div className="pointer-events-auto">
                            <Breadcrumbs
                                nodeId={nodeId}
                                relationshipId={relationshipId}
                                refreshKey={refreshKey}
                            />
                        </div>
                        {toggleMode === "hidden" ? null : (
                            <div className={clsx("pointer-events-auto", toggleMode === "shifted" && "mr-[min(440px,92vw)]")}>
                                <ModeToggle />
                            </div>
                        )}
                    </header>
                    <div className="relative min-h-0 flex-1">
                        <Sidebar
                            nodeId={nodeId}
                            relationshipId={relationshipId}
                            refreshKey={refreshKey}
                            onMutated={() => setRefreshKey((value) => value + 1)}
                        />
                    </div>
                    <ControlBar
                        selectedTool={tool}
                        onSelectTool={setTool}
                        onRecenter={() => recenterRef.current?.()}
                        onZoomIn={() => zoomRef.current?.zoomIn()}
                        onZoomOut={() => zoomRef.current?.zoomOut()}
                        onUndo={historyView.canUndo && !historyView.busy ? () => void stepHistory("undo") : undefined}
                        onDelete={
                            selectedEdge || selectedId
                                ? () => {
                                      setFrontPanel("inspector");
                                      setDeleteRequest((value) => value + 1);
                                  }
                                : undefined
                        }
                        onRedo={historyView.canRedo && !historyView.busy ? () => void stepHistory("redo") : undefined}
                        undoLabel={historyView.undoLabel}
                        redoLabel={historyView.redoLabel}
                        reviewCount={pendingReviewIds.length}
                        onOpenReview={() => {
                            showReview();
                            window.requestAnimationFrame(() => document.getElementById("review-heading")?.focus());
                        }}
                    />
                </div>
                <div
                    role="status"
                    aria-live="polite"
                    className="pointer-events-none absolute bottom-20 left-1/2 z-50 -translate-x-1/2"
                >
                    {historyNote ? (
                        <p className="rounded-lg border border-border bg-card px-3 py-1.5 text-xs text-foreground shadow-md">
                            {historyNote}
                        </p>
                    ) : null}
                </div>
                <RightRailStack
                    front={frontPanel}
                    onFocusInspector={() => setFrontPanel("inspector")}
                    onFocusReview={() => setFrontPanel("review")}
                    inspector={
                        selectedEdge ? (
                            <EdgeActions
                                edge={selectedEdge}
                                onClose={() => setSelectedEdge(null)}
                                onMutated={() => setRefreshKey((value) => value + 1)}
                                onDeleted={() => {
                                    const deleted = selectedEdge;
                                    setSelectedEdge(null);
                                    if (relationshipId && deleted?.id === relationshipId) {
                                        const fromNodeId = (location.state as { fromNodeId?: string } | null)
                                            ?.fromNodeId;
                                        const target = returnNodeIdAfterDelete(deleted, fromNodeId);
                                        if (target) navigate(`/node/${target}`);
                                        else navigate("/");
                                    }
                                }}
                                onFocus={() => setFrontPanel("inspector")}
                                deleteRequest={deleteRequest}
                            />
                        ) : selectedId ? (
                            <NodeInspector
                                key={`${selectedId}:${historyEpoch}`}
                                nodeId={selectedId}
                                refreshKey={refreshKey}
                                onClose={() => setSelectedId(null)}
                                onSaved={() => setRefreshKey((value) => value + 1)}
                                onDeleted={() => {
                                    setSelectedId(null);
                                    setRefreshKey((value) => value + 1);
                                }}
                                deleteRequest={deleteRequest}
                                onFocus={() => setFrontPanel("inspector")}
                            />
                        ) : null
                    }
                    review={
                        <ProposalReview
                            refreshKey={refreshKey + proposalRefreshKey}
                            onFocus={() => setFrontPanel("review")}
                            open={reviewOpen}
                            onClose={() => {
                                setReviewOpen(false);
                                setFrontPanel("inspector");
                                window.requestAnimationFrame(() =>
                                    document.querySelector<HTMLElement>('[aria-label^="Review, "]')?.focus(),
                                );
                            }}
                            onPendingChange={onPendingReview}
                            onPreviewChange={setPreviewDiffs}
                            onHighlight={setHighlightKey}
                            highlightKey={highlightKey}
                            openItem={openItem}
                        />
                    }
                />
            </UIProvider>
        </div>
    );
}
