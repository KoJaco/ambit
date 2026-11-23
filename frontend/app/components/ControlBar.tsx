import clsx from "clsx";
import { useEffect, useRef, useState, useSyncExternalStore, type ReactNode, type Ref } from "react";
import { Tooltip } from "./tooltip";
import type { ControlBarTool } from "./types";
import {
    BookOpenIcon,
    HandGrabIcon,
    KeyboardIcon,
    MinusIcon,
    MousePointer2Icon,
    PlusIcon,
    RedoIcon,
    ScanSearchIcon,
    UndoIcon,
} from "lucide-react";
import { StructurePanel } from "./structure-panel";
import { ShortcutsDialog } from "./shortcuts-dialog";
import { DocsSheet } from "./docs-sheet";
import { groupButtonLabel } from "../canvas-structure";
import { barStore, escapeAction, focusAfterTransition, resolveBarMode } from "../bottom-bar";
import { useShortcuts } from "../hooks/use-shortcuts";
import {
    detectPlatform,
    withShortcut,
    type Platform,
    type ShortcutId,
} from "../shortcuts";
import styles from "./overlay.module.css";

type ControlButtonConfig = {
    tool: ControlBarTool;
    label: string;
    displayTitle?: string;
    shortcut?: ShortcutId;
};

type ControlButtonGroupKeys = "zoom" | "interaction" | "history";

const CONTROL_BUTTON_GROUPS: Record<
    ControlButtonGroupKeys,
    ControlButtonConfig[]
> = {
    interaction: [
        { tool: "grab", label: "Grab tool", displayTitle: "Grab", shortcut: "tool-grab" },
        { tool: "pointer", label: "Pointer tool", displayTitle: "Pointer", shortcut: "tool-pointer" },
    ],
    history: [
        { tool: "undo", label: "Undo", displayTitle: "Undo", shortcut: "undo" },
        { tool: "redo", label: "Redo", displayTitle: "Redo", shortcut: "redo" },
    ],
    zoom: [
        { tool: "zoom-in", label: "Zoom in", displayTitle: "Zoom in", shortcut: "zoom-in" },
        { tool: "zoom-out", label: "Zoom out", displayTitle: "Zoom out", shortcut: "zoom-out" },
        { tool: "recenter", label: "Re-center", displayTitle: "Re-center", shortcut: "fit-view" },
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

const focusRing =
    "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary";

// Transparent at rest, subtle hover, primary fill only for the selected tool.
function BarButton({
    label,
    tooltip,
    selected,
    pressed,
    disabled,
    onClick,
    buttonRef,
    haspopup,
    children,
}: {
    label: string;
    tooltip: string;
    selected?: boolean;
    pressed?: boolean;
    disabled?: boolean;
    onClick: () => void;
    buttonRef?: Ref<HTMLButtonElement>;
    haspopup?: "dialog";
    children: ReactNode;
}) {
    return (
        <Tooltip label={tooltip}>
            <button
                ref={buttonRef}
                type="button"
                aria-label={label}
                aria-pressed={pressed}
                aria-haspopup={haspopup}
                disabled={disabled}
                onClick={onClick}
                className={clsx(
                    "inline-flex h-8 w-8 items-center justify-center rounded-xl",
                    selected
                        ? "bg-primary text-primary-foreground"
                        : "bg-transparent text-foreground hover:bg-accent hover:ring-1 hover:ring-inset hover:ring-border",
                    disabled && "cursor-not-allowed opacity-40 hover:bg-transparent hover:ring-0",
                    focusRing,
                )}
            >
                {children}
            </button>
        </Tooltip>
    );
}

function Divider() {
    return <span aria-hidden className="mx-1 h-5 w-px shrink-0 bg-foreground/25" />;
}

const pillClass =
    "pointer-events-auto flex h-12 items-center rounded-full border border-foreground/25 bg-background/90 backdrop-blur px-2";
const barButtonClass =
    "inline-flex h-7 min-w-6 items-center justify-center rounded-full px-3 text-[11px] font-medium disabled:opacity-50 " +
    focusRing;

export function ControlBar({
    selectedTool,
    onSelectTool,
    onRecenter,
    onZoomOut,
    onZoomIn,
    onUndo,
    onRedo,
    onDelete,
    undoLabel,
    redoLabel,
}: {
    selectedTool: ControlBarTool;
    onSelectTool: (tool: ControlBarTool) => void;
    onRecenter?: () => void;
    onZoomOut?: () => void;
    onZoomIn?: () => void;
    onUndo?: () => void;
    onRedo?: () => void;
    // Delete/Backspace on the selected edge or node; undefined when nothing is selected.
    onDelete?: () => void;
    // What the next undo/redo would change, for the tooltip.
    undoLabel?: string | null;
    redoLabel?: string | null;
}) {
    const session = useSyncExternalStore(barStore.subscribe, barStore.get, barStore.getServerSnapshot);
    const mode = resolveBarMode(session);
    const [panel, setPanel] = useState<null | "shortcuts" | "docs">(null);
    const [platform, setPlatform] = useState<Platform>("other");
    const shortcutsButtonRef = useRef<HTMLButtonElement>(null);
    const docsButtonRef = useRef<HTMLButtonElement>(null);
    const groupButtonRef = useRef<HTMLButtonElement>(null);
    const toolbarRef = useRef<HTMLDivElement>(null);
    const prevMode = useRef(mode);
    // Remember which button opened the dialog so focus goes back to it, even for the `?` key.
    const lastPanelOpener = useRef<HTMLButtonElement | null>(null);

    useEffect(() => {
        setPlatform(detectPlatform(navigator));
    }, []);

    // Restore focus to the bar's equivalent control when a form closes and the bar swaps back.
    useEffect(() => {
        const target = focusAfterTransition(prevMode.current, mode);
        prevMode.current = mode;
        if (target === "group-button") groupButtonRef.current?.focus();
        else if (target === "selected-tool")
            toolbarRef.current?.querySelector<HTMLElement>("[aria-pressed=true]")?.focus();
    }, [mode]);

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

    function openPanel(next: "shortcuts" | "docs", opener: HTMLButtonElement | null) {
        lastPanelOpener.current = opener;
        setPanel(next);
    }

    useShortcuts(
        {
            "tool-pointer": () => onSelectTool("pointer"),
            "tool-grab": () => onSelectTool("grab"),
            undo: onUndo,
            redo: onRedo,
            delete: onDelete,
            "zoom-in": onZoomIn,
            "zoom-out": onZoomOut,
            "fit-view": onRecenter,
            "open-shortcuts": () => openPanel("shortcuts", shortcutsButtonRef.current),
            close: () => {
                const action = escapeAction(mode);
                if (action === "clear-selection") session.onClearSelection();
                else if (action === "cancel-request") session.onCancel();
            },
        },
        { dialogOpen: panel !== null, platform },
    );

    const isDisabled = (tool: ControlBarTool) =>
        (tool === "undo" && !onUndo) || (tool === "redo" && !onRedo);

    let content: ReactNode;
    if (mode === "group" || mode === "ungroup") {
        content = session.request ? (
            <StructurePanel
                key={mode}
                request={session.request}
                busy={session.busy}
                onConfirm={session.onConfirm}
                onCancel={session.onCancel}
            />
        ) : null;
    } else if (mode === "selection") {
        content = (
            <div role="group" aria-label="Selection" className="flex items-center gap-2 px-2 text-[11px] text-foreground">
                <span className="font-medium" aria-live="polite">
                    {session.selectedCount} selected
                </span>
                <button
                    ref={groupButtonRef}
                    type="button"
                    aria-label={groupButtonLabel(session.selectedCount)}
                    onClick={(event) => session.onOpenGroup(event.currentTarget)}
                    className={`${barButtonClass} bg-primary text-primary-foreground`}
                >
                    Group
                </button>
                <button
                    type="button"
                    aria-label="Clear selection"
                    onClick={session.onClearSelection}
                    className={`${barButtonClass} border border-border hover:bg-accent`}
                >
                    Cancel
                </button>
            </div>
        );
    } else {
        content = (
            <div ref={toolbarRef} role="toolbar" aria-label="Canvas tools" className="flex items-center">
                {Object.entries(CONTROL_BUTTON_GROUPS).map(([groupKey, groupButtons]) => (
                    <div key={groupKey} className="flex items-center gap-0.5">
                        {groupButtons.map(({ tool, label, displayTitle, shortcut }) => {
                            const isTool = tool === "grab" || tool === "pointer";
                            const detail = tool === "undo" ? undoLabel : tool === "redo" ? redoLabel : null;
                            const title = detail ? `${displayTitle ?? label}: ${detail}` : (displayTitle ?? label);
                            return (
                                <BarButton
                                    key={tool}
                                    label={label}
                                    tooltip={shortcut ? withShortcut(title, shortcut, platform) : title}
                                    selected={isTool && selectedTool === tool}
                                    pressed={isTool ? selectedTool === tool : undefined}
                                    disabled={isDisabled(tool)}
                                    onClick={() => handleButtonClick(tool)}
                                >
                                    <GetControlBarIcon tool={tool} />
                                </BarButton>
                            );
                        })}
                        <Divider />
                    </div>
                ))}
                <div className="flex items-center gap-0.5">
                    <BarButton
                        label="Keyboard shortcuts"
                        tooltip={withShortcut("Keyboard shortcuts", "open-shortcuts", platform)}
                        buttonRef={shortcutsButtonRef}
                        haspopup="dialog"
                        onClick={() => openPanel("shortcuts", shortcutsButtonRef.current)}
                    >
                        <KeyboardIcon className="h-4 w-4" />
                    </BarButton>
                    <BarButton
                        label="Documentation"
                        tooltip="Documentation"
                        buttonRef={docsButtonRef}
                        haspopup="dialog"
                        onClick={() => openPanel("docs", docsButtonRef.current)}
                    >
                        <BookOpenIcon className="h-4 w-4" />
                    </BarButton>
                </div>
            </div>
        );
    }

    return (
        <div className="pointer-events-none flex justify-center">
            <div className={pillClass}>
                {/* Keyed by mode so each swap replays the short fade/slide (off for reduced motion). */}
                <div key={mode} className={styles.barSwap}>
                    {content}
                </div>
            </div>
            {panel === "shortcuts" ? (
                <ShortcutsDialog
                    platform={platform}
                    onClose={() => setPanel(null)}
                    returnFocusRef={lastPanelOpener}
                />
            ) : null}
            {panel === "docs" ? (
                <DocsSheet
                    onClose={() => setPanel(null)}
                    onOpenShortcuts={() => openPanel("shortcuts", shortcutsButtonRef.current)}
                    returnFocusRef={lastPanelOpener}
                />
            ) : null}
        </div>
    );
}

export default ControlBar;
