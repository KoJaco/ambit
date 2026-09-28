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
import { useParams } from "react-router";
import { subscribeEvents } from "../api";
import { Breadcrumbs } from "../components/Breadcrumbs";
import ScreenSizeAlert from "../components/ScreenSizeAlert";
import { ProposalReview } from "../components/proposal-review";
import { RightRailStack, type RightRailFront } from "../components/right-rail-stack";
import { useDesktopViewport } from "../hooks/use-desktop-viewport";

export function meta({}: Route.MetaArgs) {
    return [
        { title: "ambit" },
        { name: "description", content: "Architecture canvas" },
    ];
}

export default function Home() {
    const desktopViewport = useDesktopViewport();
    const { nodeId } = useParams();
    const [tool, setTool] = useState<ControlBarTool>("grab");
    const [refreshKey, setRefreshKey] = useState(0);
    const [proposalRefreshKey, setProposalRefreshKey] = useState(0);
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
    }, [nodeId]);

    if (!desktopViewport) {
        return <ScreenSizeAlert />;
    }

    return (
        <div className="fixed top-0 left-0 h-dvh w-full overflow-hidden bg-background">
            <UIProvider>
                <ReactFlowProvider>
                    <FlowCanvas
                        nodeId={nodeId}
                        refreshKey={refreshKey}
                        tool={tool === "grab" ? "grab" : "pointer"}
                        onSelectNode={(id) => {
                            setSelectedId(id);
                            setFrontPanel("inspector");
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
                            <Breadcrumbs nodeId={nodeId} refreshKey={refreshKey} />
                        </div>
                        <div className="pointer-events-auto">
                            <ModeToggle />
                        </div>
                    </header>
                    <div className="relative min-h-0 flex-1">
                        <Sidebar
                            nodeId={nodeId}
                            refreshKey={refreshKey}
                            onMutated={() => setRefreshKey((value) => value + 1)}
                        />
                        <RightRailStack
                            front={frontPanel}
                            onFocusInspector={() => setFrontPanel("inspector")}
                            onFocusReview={() => setFrontPanel("review")}
                            inspector={
                                selectedId ? (
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
