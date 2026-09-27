import clsx from "clsx";
import type { ControlBarTool } from "./types";
import {
    HandGrabIcon,
    MinusIcon,
    MousePointer2Icon,
    PlusIcon,
    PointerIcon,
    RedoIcon,
    ScanSearchIcon,
    UndoIcon,
} from "lucide-react";

type ControlButtonConfig = {
    tool: ControlBarTool;
    label: string;
    displayTitle?: string;
};

type ControlButtonGroupKeys = "zoom" | "interaction" | "history";

const CONTROL_BUTTON_GROUPS: Record<
    ControlButtonGroupKeys,
    ControlButtonConfig[]
> = {
    interaction: [
        { tool: "grab", label: "Grab tool", displayTitle: "Grab" },
        { tool: "pointer", label: "Pointer tool", displayTitle: "Pointer" },
    ],
    history: [
        { tool: "undo", label: "Undo", displayTitle: "Undo" },
        { tool: "redo", label: "Redo", displayTitle: "Redo" },
    ],
    zoom: [
        { tool: "zoom-in", label: "Zoom in", displayTitle: "Zoom in" },
        { tool: "zoom-out", label: "Zoom out", displayTitle: "Zoom out" },
        { tool: "recenter", label: "Re-center", displayTitle: "Re-center" },
    ],
};

function GetControlBarIcon({ tool }: { tool: ControlBarTool }) {
    switch (tool) {
        case "zoom-in":
            return <PlusIcon className="w-4 h-4" />;
        case "zoom-out":
            return <MinusIcon className="w-4 h-4" />;

        case "grab":
            return <HandGrabIcon className="w-4 h-4" />;
        case "pointer":
            return <MousePointer2Icon className="w-4 h-4" />;
        case "undo":
            return <UndoIcon className="w-4 h-4" />;
        case "redo":
            return <RedoIcon className="w-4 h-4" />;
        case "recenter":
            return <ScanSearchIcon className="w-4 h-4" />;

        default:
            return null;
    }
}

function ControlBarButton({
    tool,
    label,
    displayTitle,
    selectedTool,
    onClick,
}: {
    tool: ControlBarTool;
    label: string;
    displayTitle?: string;
    selectedTool: ControlBarTool;
    onClick: () => void;
}) {
    const isSelected = selectedTool === tool;
    const baseClasses =
        "h-8 w-8 inline-flex items-center justify-center rounded-xl";
    const variantClasses = isSelected
        ? "bg-primary text-primary-foreground"
        : "bg-primary-foreground text-primary";
    return (
        <button
            aria-label={label}
            title={displayTitle ?? label}
            onClick={onClick}
            className={clsx(baseClasses, variantClasses)}
        >
            <GetControlBarIcon tool={tool} />
        </button>
    );
}

export function ControlBar({
    selectedTool,
    onSelectTool,
    showRecenter = false,
    onRecenter,
    onZoomOut,
    onZoomIn,
    onUndo,
    onRedo,
}: {
    selectedTool: ControlBarTool;
    onSelectTool: (tool: ControlBarTool) => void;
    showRecenter?: boolean;
    onRecenter?: () => void;
    onZoomOut?: () => void;
    onZoomIn?: () => void;
    onUndo?: () => void;
    onRedo?: () => void;
}) {
    function handleButtonClick(tool: ControlBarTool) {
        switch (tool) {
            case "undo":
                onUndo?.();
                return;
            case "redo":
                onRedo?.();
                return;
            case "grab":
            case "pointer":
                onSelectTool(tool);
                return;
            case "zoom-in":
                onZoomIn?.();
                return;
            case "zoom-out":
                onZoomOut?.();
                return;
            case "recenter":
                onRecenter?.();
                return;
            default:
                return;
        }
    }
    return (
        <div className="pointer-events-none fixed bottom-4 left-1/2 -translate-x-1/2 z-50">
            <div className="pointer-events-auto flex items-center gap-2 rounded-full border border-foreground/25 bg-background/90 backdrop-blur px-2 py-1.5">
                {Object.entries(CONTROL_BUTTON_GROUPS).map(
                    ([groupKey, groupButtons], index) => {
                        return (
                            <div
                                key={groupKey}
                                className={clsx(
                                    "border-r border-primary/50 pr-2",
                                    index ===
                                        Object.entries(CONTROL_BUTTON_GROUPS)
                                            .length -
                                            1 && "border-r-0"
                                )}
                            >
                                {groupButtons.map(
                                    ({ tool, label, displayTitle }) => {
                                        return (
                                            <ControlBarButton
                                                key={tool}
                                                tool={tool}
                                                label={label}
                                                displayTitle={displayTitle}
                                                selectedTool={selectedTool}
                                                onClick={() =>
                                                    handleButtonClick(tool)
                                                }
                                            />
                                        );
                                    }
                                )}

                                {showRecenter && (
                                    <ControlBarButton
                                        tool="recenter"
                                        label="Re-center"
                                        displayTitle="Re-center"
                                        selectedTool={selectedTool}
                                        onClick={() => onRecenter?.()}
                                    />
                                )}
                            </div>
                        );
                    }
                )}
                {/*                 
                {CONTROL_BUTTONS.map(({ tool, label, displayTitle }) => {
                    return (
                        <ControlBarButton
                            key={tool}
                            tool={tool}
                            label={label}
                            displayTitle={displayTitle}
                            selectedTool={selectedTool}
                            onClick={() => handleButtonClick(tool)}
                        />
                    );
                })} */}
            </div>
        </div>
    );
}

export default ControlBar;
