import { describe, expect, it } from "vitest";
import { isBackdropClick, nextFocusIndex } from "./overlay-logic";

describe("nextFocusIndex", () => {
    it("wraps forwards and backwards", () => {
        expect(nextFocusIndex(3, 2, false)).toBe(0);
        expect(nextFocusIndex(3, 0, true)).toBe(2);
        expect(nextFocusIndex(3, 0, false)).toBe(1);
    });
    it("enters the list from outside", () => {
        expect(nextFocusIndex(3, -1, false)).toBe(0);
        expect(nextFocusIndex(3, -1, true)).toBe(2);
    });
    it("handles an empty list", () => {
        expect(nextFocusIndex(0, -1, false)).toBe(-1);
    });
});

describe("isBackdropClick", () => {
    it("requires press and release on the backdrop", () => {
        const backdrop = {};
        expect(isBackdropClick(backdrop, backdrop, backdrop)).toBe(true);
        expect(isBackdropClick({}, backdrop, backdrop)).toBe(false);
        expect(isBackdropClick(backdrop, {}, backdrop)).toBe(false);
    });
});
