import type { Route } from "./+types/home";
import { Sidebar } from "../components/Sidebar";
import FlowCanvas from "../components/FlowCanvas";
import { ControlBar } from "../components/ControlBar";
import type { ControlBarTool } from "../components/types";
import { useEffect, useRef, useState } from "react";
import { ModeToggle } from "~/components/ui/mode-toggle";
import { ReactFlowProvider } from "@xyflow/react";
import { UIProvider } from "../components/ui-context";
import { useParams } from "react-router";
import { subscribeEvents } from "../api";

export function meta({}: Route.MetaArgs) {
    return [
        { title: "ambit" },
        { name: "description", content: "Architecture canvas" },
    ];
}

export default function Home() {
    const { nodeId } = useParams();
    const [tool, setTool] = useState<ControlBarTool>("grab");
    const [refreshKey, setRefreshKey] = useState(0);
    useEffect(
        () =>
            subscribeEvents({
                onModelChanged: () => setRefreshKey((value) => value + 1),
                onIntegrityChanged: () => setRefreshKey((value) => value + 1),
            }),
        [],
    );
    const recenterRef = useRef<null | (() => void)>(null);
    const zoomRef = useRef<{ zoomIn: () => void; zoomOut: () => void } | null>(
        null
    );

    return (
        <div className="flex min-h-screen">
            <div className="relative flex-1 overflow-x-hidden bg-background">
                <div className="absolute top-4 right-4 z-50">
                    <ModeToggle />
                </div>
                <UIProvider>
                    <Sidebar
                        nodeId={nodeId}
                        refreshKey={refreshKey}
                        onMutated={() => setRefreshKey((value) => value + 1)}
                    />
                    <ReactFlowProvider>
                        <FlowCanvas
                            nodeId={nodeId}
                            refreshKey={refreshKey}
                            tool={tool === "grab" ? "grab" : "pointer"}
                            onRequestRecenterRef={(fn) => {
                                recenterRef.current = fn;
                            }}
                            onRequestZoomRef={(api) => {
                                zoomRef.current = api;
                            }}
                        />
                    </ReactFlowProvider>
                </UIProvider>
                <ControlBar
                    selectedTool={tool}
                    onSelectTool={setTool}
                    showRecenter
                    onRecenter={() => recenterRef.current?.()}
                    onZoomIn={() => zoomRef.current?.zoomIn()}
                    onZoomOut={() => zoomRef.current?.zoomOut()}
                />
            </div>
        </div>
    );
}
