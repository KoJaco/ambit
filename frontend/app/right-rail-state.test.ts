import { describe, expect, it } from "vitest";
import {
    floatingToggleMode,
    frontAfterReviewChange,
    isEditableElement,
    nextPanelPresence,
    paneClickCloses,
    panelExitMs,
    railStackLayout,
    reviewEscapeAction,
    shouldAutoOpenReview,
} from "./right-rail-state";

describe("floatingToggleMode", () => {
    it("hides, shifts or leaves the floating toggle", () => {
        expect(floatingToggleMode({ panels: 0, toggles: 0, review: 0 })).toBe("normal");
        expect(floatingToggleMode({ panels: 1, toggles: 0, review: 0 })).toBe("shifted");
        expect(floatingToggleMode({ panels: 2, toggles: 1, review: 1 })).toBe("hidden");
    });
});

describe("shouldAutoOpenReview", () => {
    const base = { pendingIds: ["p1"], seenIds: new Set<string>(), editingFocus: false, inspectorDirty: false };
    it("opens for an unseen proposal when idle", () => {
        expect(shouldAutoOpenReview(base)).toBe(true);
    });
    it("does not open while editing or with unsaved inspector edits", () => {
        expect(shouldAutoOpenReview({ ...base, editingFocus: true })).toBe(false);
        expect(shouldAutoOpenReview({ ...base, inspectorDirty: true })).toBe(false);
    });
    it("does not reopen for a proposal already seen, or with none pending", () => {
        expect(shouldAutoOpenReview({ ...base, seenIds: new Set(["p1"]) })).toBe(false);
        expect(shouldAutoOpenReview({ ...base, pendingIds: [] })).toBe(false);
        expect(shouldAutoOpenReview({ ...base, pendingIds: ["p1", "p2"], seenIds: new Set(["p1"]) })).toBe(true);
    });
});

describe("isEditableElement", () => {
    it("detects text-entry elements", () => {
        expect(isEditableElement({ tagName: "INPUT" })).toBe(true);
        expect(isEditableElement({ tagName: "textarea" })).toBe(true);
        expect(isEditableElement({ tagName: "SELECT" })).toBe(true);
        expect(isEditableElement({ tagName: "DIV", isContentEditable: true })).toBe(true);
        expect(isEditableElement({ tagName: "BUTTON" })).toBe(false);
        expect(isEditableElement(null)).toBe(false);
    });
});

describe("reviewEscapeAction", () => {
    it("steps back from detail, then closes, and ignores typing", () => {
        expect(reviewEscapeAction({ detailOpen: true, editingFocus: false })).toBe("back");
        expect(reviewEscapeAction({ detailOpen: false, editingFocus: false })).toBe("close");
        expect(reviewEscapeAction({ detailOpen: true, editingFocus: true })).toBe("none");
    });
});

describe("railStackLayout", () => {
    it("does not stack the inspector behind a Review that renders nothing", () => {
        const l = railStackLayout({ hasInspector: true, reviewMounted: false, front: "review" });
        expect(l).toEqual({ stacked: false, inspectorFront: true, reviewFront: true });
    });
    it("stacks only when both sheets exist and honours the front choice", () => {
        expect(railStackLayout({ hasInspector: true, reviewMounted: true, front: "review" })).toEqual({
            stacked: true,
            inspectorFront: false,
            reviewFront: true,
        });
        expect(railStackLayout({ hasInspector: false, reviewMounted: true, front: "inspector" }).stacked).toBe(false);
    });
});

describe("frontAfterReviewChange", () => {
    it("falls back to the inspector when Review unmounts", () => {
        expect(frontAfterReviewChange("review", false)).toBe("inspector");
        expect(frontAfterReviewChange("review", true)).toBe("review");
    });
});

describe("panelExitMs", () => {
    it("skips the exit animation for reduced motion", () => {
        expect(panelExitMs(true)).toBe(0);
        expect(panelExitMs(false)).toBeGreaterThan(0);
    });
});

describe("paneClickCloses", () => {
    it("keeps an inspector with unsaved edits open", () => {
        expect(paneClickCloses(true).inspector).toBe(false);
        expect(paneClickCloses(false).inspector).toBe(true);
    });
});

describe("nextPanelPresence", () => {
    it("lingers on the render where show turns false", () => {
        expect(nextPanelPresence({ show: true, lingering: false }, false, false)).toEqual({ show: false, lingering: true });
    });
    it("unmounts at once with reduced motion", () => {
        expect(nextPanelPresence({ show: true, lingering: false }, false, true)).toEqual({ show: false, lingering: false });
    });
    it("stops lingering when shown again", () => {
        expect(nextPanelPresence({ show: false, lingering: true }, true, false)).toEqual({ show: true, lingering: false });
    });
});

import { inspectorCloseAction, restoreInspectorFocus } from "./right-rail-state";
describe("inspectorCloseAction", () => {
    it("closes a clean inspector at once", () => {
        expect(inspectorCloseAction({ dirty: false, confirming: false, request: "close" })).toBe("close");
        expect(inspectorCloseAction({ dirty: false, confirming: false, request: "escape" })).toBe("close");
    });
    it("asks before closing a dirty one", () => {
        expect(inspectorCloseAction({ dirty: true, confirming: false, request: "close" })).toBe("confirm");
        expect(inspectorCloseAction({ dirty: true, confirming: false, request: "escape" })).toBe("confirm");
    });
    it("keeps editing on Escape while asking, and keeps asking on a repeat close", () => {
        expect(inspectorCloseAction({ dirty: true, confirming: true, request: "escape" })).toBe("keep");
        expect(inspectorCloseAction({ dirty: true, confirming: true, request: "close" })).toBe("confirm");
    });
});

describe("restoreInspectorFocus", () => {
    it("restores focus when the discard prompt closes and focus fell to the page", () => {
        expect(restoreInspectorFocus(true, false, true)).toBe(true);
    });
    it("does nothing while prompting, when focus is elsewhere, or without a prompt", () => {
        expect(restoreInspectorFocus(true, true, true)).toBe(false);
        expect(restoreInspectorFocus(true, false, false)).toBe(false);
        expect(restoreInspectorFocus(false, false, true)).toBe(false);
    });
});
