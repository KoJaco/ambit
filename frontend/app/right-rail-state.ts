import { useSyncExternalStore } from "react";

// Tracks which right-rail panels are mounted so page chrome can get out of their way:
// `panels` counts open RightRailPanels, `toggles` counts panels that host their own theme toggle,
// `review` counts mounted Review panels (the Review component itself renders nothing when closed or empty).
export type RailState = { panels: number; toggles: number; review: number };

let state: RailState = { panels: 0, toggles: 0, review: 0 };
const listeners = new Set<() => void>();

function update(patch: Partial<Record<keyof RailState, number>>) {
    state = {
        panels: state.panels + (patch.panels ?? 0),
        toggles: state.toggles + (patch.toggles ?? 0),
        review: state.review + (patch.review ?? 0),
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

export type RightRailFront = "inspector" | "review";

// Whether the inspector and Review sheets stack. Review is only a sheet while it actually renders
// one, so an always-mounted-but-empty Review never pushes the inspector into the back.
export function railStackLayout(input: { hasInspector: boolean; reviewMounted: boolean; front: RightRailFront }): {
    stacked: boolean;
    inspectorFront: boolean;
    reviewFront: boolean;
} {
    const stacked = input.hasInspector && input.reviewMounted;
    return {
        stacked,
        inspectorFront: !stacked || input.front === "inspector",
        reviewFront: !stacked || input.front === "review",
    };
}

// Which sheet is in front once Review is gone: the inspector, so it is not left behind a stale choice.
export function frontAfterReviewChange(front: RightRailFront, reviewMounted: boolean): RightRailFront {
    return reviewMounted ? front : "inspector";
}

// Length of the sheet exit animation. None when the user prefers reduced motion (WCAG 2.2 SC 2.3.3).
export const PANEL_EXIT_MS = 150;
export function panelExitMs(prefersReducedMotion: boolean): number {
    return prefersReducedMotion ? 0 : PANEL_EXIT_MS;
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

// What a click on empty canvas closes: the inspector stays when it holds unsaved edits.
export function paneClickCloses(inspectorDirty: boolean): { inspector: boolean } {
    return { inspector: !inspectorDirty };
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

// What Escape does inside the Review panel: leave a detail view first, then close the panel.
export function reviewEscapeAction(input: { detailOpen: boolean; editingFocus: boolean }): "back" | "close" | "none" {
    if (input.editingFocus) return "none";
    return input.detailOpen ? "back" : "close";
}

// Presence of an exit-animating panel as pure state: `lingering` flips on the render where `show`
// turns false (so the panel never unmounts for a frame), unless reduced motion is on.
export function nextPanelPresence(
    prev: { show: boolean; lingering: boolean },
    show: boolean,
    prefersReducedMotion: boolean,
): { show: boolean; lingering: boolean } {
    if (show) return { show, lingering: false };
    if (prev.show) return { show, lingering: panelExitMs(prefersReducedMotion) > 0 };
    return { show, lingering: prev.lingering };
}
