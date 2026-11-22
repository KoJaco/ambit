import { describe, expect, it } from "vitest";
import { fitInsets } from "./components/FlowCanvas";

describe("fitInsets", () => {
    it("reserves only the sidebar when no right sheet is open", () => {
        expect(fitInsets(1440, false)).toEqual({ left: 290, right: 0 });
    });
    it("reserves the right sheet width plus a gutter when one is open", () => {
        expect(fitInsets(1440, true)).toEqual({ left: 290, right: 464 });
    });
    it("caps the sheet at 92% of a narrow viewport", () => {
        expect(fitInsets(1000, true).right).toBe(440 + 24);
        expect(fitInsets(400, true)).toEqual({ left: 120, right: Math.round(400 * 0.92) + 24 });
    });
});
