import type { Route } from "./+types/home";
import { Sidebar } from "../components/Sidebar";
import FlowCanvas from "../components/FlowCanvas";
import { ControlBar } from "../components/ControlBar";
import type { ControlBarTool } from "../components/types";
import { useRef, useState } from "react";
import { ModeToggle } from "~/components/ui/mode-toggle";
import { ReactFlowProvider } from "@xyflow/react";
import { UIProvider } from "../components/ui-context";

export function meta({}: Route.MetaArgs) {
    return [
        { title: "New React Router App" },
        { name: "description", content: "Welcome to React Router!" },
    ];
}

export default function Home() {
    const [tool, setTool] = useState<ControlBarTool>("grab");
    const [showRecenter, _] = useState(false);
    const recenterRef = useRef<null | (() => void)>(null);
    const zoomRef = useRef<{ zoomIn: () => void; zoomOut: () => void } | null>(
        null
    );

    return (
        <div className="flex min-h-screen">
            <div className="relative flex-1 bg-background overflow-x-hidden">
                <div className="absolute z-50 top-4 right-4">
                    <ModeToggle />
                </div>
                <UIProvider>
                    <Sidebar />
                    <ReactFlowProvider>
                        <FlowCanvas
                            tool={tool === "grab" ? "grab" : "pointer"}
                            onRequestRecenterRef={(fn) =>
                                (recenterRef.current = fn)
                            }
                            onRequestZoomRef={(api) => (zoomRef.current = api)}
                        />
                    </ReactFlowProvider>
                </UIProvider>
                <ControlBar
                    selectedTool={tool}
                    onSelectTool={setTool}
                    showRecenter={showRecenter}
                    onRecenter={() => recenterRef.current?.()}
                    onZoomIn={() => zoomRef.current?.zoomIn()}
                    onZoomOut={() => zoomRef.current?.zoomOut()}
                />
            </div>
        </div>
    );
}
