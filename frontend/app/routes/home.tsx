import type { Route } from "./+types/home";
import { Sidebar } from "../components/Sidebar";
import FlowCanvas from "../components/FlowCanvas";
import NodeInspector from "../components/node-inspector";
import { ControlBar } from "../components/ControlBar";
import type { ControlBarTool } from "../components/types";
import { useEffect, useRef, useState } from "react";
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
import type { Relationship } from "../api";
import { returnNodeIdAfterDelete } from "../relationship-breadcrumb";
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
    useEffect(
        () =>
            subscribeEvents({
                onModelChanged: () => setRefreshKey((value) => value + 1),
                onIntegrityChanged: () => setRefreshKey((value) => value + 1),
                onProposalsChanged: () => setProposalRefreshKey((value) => value + 1),
            }),
        [],
    );
    const [selectedId, setSelectedId] = useState<string | null>(null);
    const [frontPanel, setFrontPanel] = useState<RightRailFront>("review");
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
                        tool={tool === "grab" ? "grab" : "pointer"}
                        onSelectNode={(id) => {
                            setSelectedId(id);
                            setSelectedEdge(null);
                            setFrontPanel("inspector");
                        }}
                        onSelectEdge={(edge) => {
                            setSelectedEdge(edge);
                            setSelectedId(null);
                            if (edge) setFrontPanel("inspector");
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
                        <div className="pointer-events-auto">
                            <ModeToggle />
                        </div>
                    </header>
                    <div className="relative min-h-0 flex-1">
                        <Sidebar
                            nodeId={nodeId}
                            relationshipId={relationshipId}
                            refreshKey={refreshKey}
                            onMutated={() => setRefreshKey((value) => value + 1)}
                        />
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
                                    />
                                ) : selectedId ? (
                                    <NodeInspector
                                        nodeId={selectedId}
                                        onClose={() => setSelectedId(null)}
                                        onSaved={() => setRefreshKey((value) => value + 1)}
                                        onFocus={() => setFrontPanel("inspector")}
                                    />
                                ) : null
                            }
                            review={
                                <ProposalReview
                                    refreshKey={refreshKey + proposalRefreshKey}
                                    onFocus={() => setFrontPanel("review")}
                                />
                            }
                        />
                    </div>
                    <ControlBar
                        selectedTool={tool}
                        onSelectTool={setTool}
                        onRecenter={() => recenterRef.current?.()}
                        onZoomIn={() => zoomRef.current?.zoomIn()}
                        onZoomOut={() => zoomRef.current?.zoomOut()}
                    />
                </div>
            </UIProvider>
        </div>
    );
}
