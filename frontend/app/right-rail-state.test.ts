import { describe, expect, it } from "vitest";
import { floatingToggleMode, isEditableElement, reviewEscapeAction, shouldAutoOpenReview } from "./right-rail-state";

describe("floatingToggleMode", () => {
    it("hides, shifts or leaves the floating toggle", () => {
        expect(floatingToggleMode({ panels: 0, toggles: 0 })).toBe("normal");
        expect(floatingToggleMode({ panels: 1, toggles: 0 })).toBe("shifted");
        expect(floatingToggleMode({ panels: 2, toggles: 1 })).toBe("hidden");
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
