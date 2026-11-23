import { useSyncExternalStore } from "react";

// Tracks which right-rail panels are mounted so page chrome can get out of their way:
// `panels` counts open RightRailPanels, `toggles` counts panels that host their own theme toggle.
type RailState = { panels: number; toggles: number };

let state: RailState = { panels: 0, toggles: 0 };
const listeners = new Set<() => void>();

function update(patch: Partial<Record<keyof RailState, number>>) {
    state = {
        panels: state.panels + (patch.panels ?? 0),
        toggles: state.toggles + (patch.toggles ?? 0),
    };
    listeners.forEach((listener) => listener());
}

export function registerRailPanel(kind: keyof RailState): () => void {
    update({ [kind]: 1 });
    return () => update({ [kind]: -1 });
}

function subscribe(listener: () => void) {
    listeners.add(listener);
    return () => {
        listeners.delete(listener);
    };
}

export function useRailState(): RailState {
    return useSyncExternalStore(subscribe, () => state, () => state);
}

// Where the page-level theme toggle belongs: hidden when a panel hosts one, shifted left of the
// rail when a panel without one is open, otherwise in its normal top-right place.
export function floatingToggleMode(s: RailState): "hidden" | "shifted" | "normal" {
    if (s.toggles > 0) return "hidden";
    return s.panels > 0 ? "shifted" : "normal";
}

// ---- review auto-open ---------------------------------------------------------------------

// Whether the open inspector holds edits that are not saved. Read at decision time, not rendered.
let inspectorDirty = false;
export function setInspectorDirty(value: boolean) {
    inspectorDirty = value;
}
export function isInspectorDirty(): boolean {
    return inspectorDirty;
}

type ElementLike = { tagName?: string; isContentEditable?: boolean } | null | undefined;

// True when the focused element takes typed text, so a panel change would pull the rug out.
export function isEditableElement(el: ElementLike): boolean {
    if (!el) return false;
    if (el.isContentEditable) return true;
    const tag = (el.tagName ?? "").toUpperCase();
    return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT";
}

// Bring Review forward only for a proposal not seen before and only when the user is not mid-edit.
// Ids already seen never re-open it, so closing or leaving the panel sticks for that proposal.
export function shouldAutoOpenReview(input: {
    pendingIds: readonly string[];
    seenIds: ReadonlySet<string>;
    editingFocus: boolean;
    inspectorDirty: boolean;
}): boolean {
    if (input.editingFocus || input.inspectorDirty) return false;
    return input.pendingIds.some((id) => !input.seenIds.has(id));
}
