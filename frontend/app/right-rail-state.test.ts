import { describe, expect, it } from "vitest";
import {
    floatingToggleMode,
    frontAfterReviewChange,
    isEditableElement,
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
