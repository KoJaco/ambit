// The bottom bar is a single slot. It shows the toolbar, or one action bar that replaces it.
// FlowCanvas owns the group/ungroup requests and the selection; ControlBar renders the slot.
// They meet through a tiny external store so neither needs the other's props.

import type { StructureRequest } from "./components/structure-panel";

export type BarMode = "toolbar" | "selection" | "group" | "ungroup";

// An open request always wins (it is what the user just asked for). Otherwise two or more
// groupable nodes swap the toolbar for the compact "N selected" bar. Anything else is the toolbar.
export function resolveBarMode(input: { request: StructureRequest | null; selectedCount: number }): BarMode {
    if (input.request) return input.request.kind;
    return input.selectedCount >= 2 ? "selection" : "toolbar";
}

export type EscapeAction = "cancel-request" | "clear-selection" | "none";

// What Esc does for each mode. group/ungroup forms handle Esc themselves; this is the fallback
// for when focus is elsewhere.
export function escapeAction(mode: BarMode): EscapeAction {
    if (mode === "group" || mode === "ungroup") return "cancel-request";
    if (mode === "selection") return "clear-selection";
    return "none";
}

// Where focus should land when a form closes. The Group button is replaced when the bar swaps,
// so the bar restores focus to its new equivalent. Ungroup opens from a node on the canvas,
// which FlowCanvas restores itself.
export type FocusTarget = "group-button" | "selected-tool" | "none";

export function focusAfterTransition(prev: BarMode, next: BarMode): FocusTarget {
    if (prev === "group") return next === "selection" ? "group-button" : next === "toolbar" ? "selected-tool" : "none";
    if (prev === "selection" && next === "toolbar") return "none";
    return "none";
}

export type BarSession = {
    request: StructureRequest | null;
    busy: boolean;
    selectedCount: number;
    onOpenGroup: (opener: HTMLElement | null) => void;
    onConfirm: (value: { name: string; deleteContainer: boolean }) => void;
    onCancel: () => void;
    onClearSelection: () => void;
};

const idle: BarSession = {
    request: null,
    busy: false,
    selectedCount: 0,
    onOpenGroup: () => {},
    onConfirm: () => {},
    onCancel: () => {},
    onClearSelection: () => {},
};

type Listener = () => void;

export function createBarStore() {
    let session = idle;
    const listeners = new Set<Listener>();
    return {
        get: () => session,
        set(next: BarSession) {
            session = next;
            listeners.forEach((listener) => listener());
        },
        reset() {
            if (session === idle) return;
            session = idle;
            listeners.forEach((listener) => listener());
        },
        subscribe(listener: Listener) {
            listeners.add(listener);
            return () => {
                listeners.delete(listener);
            };
        },
        getServerSnapshot: () => idle,
    };
}

export const barStore = createBarStore();
