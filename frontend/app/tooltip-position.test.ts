import { describe, expect, it } from "vitest";
import { placeTooltip } from "./tooltip-position";

const viewport = { width: 1000, height: 800 };
const tip = { width: 80, height: 24 };

describe("placeTooltip", () => {
    it("centres above the anchor by default", () => {
        const out = placeTooltip({ left: 480, top: 400, width: 32, height: 32 }, tip, viewport);
        expect(out).toEqual({ left: 456, top: 400 - 6 - 24, placement: "top" });
    });

    it("flips below when there is no room above", () => {
        const out = placeTooltip({ left: 480, top: 10, width: 32, height: 32 }, tip, viewport);
        expect(out.placement).toBe("bottom");
        expect(out.top).toBe(10 + 32 + 6);
    });

    it("flips above when preferring below near the bottom edge", () => {
        const out = placeTooltip({ left: 480, top: 770, width: 32, height: 24 }, tip, viewport, { prefer: "bottom" });
        expect(out.placement).toBe("top");
    });

    it("keeps its preferred side when neither fits", () => {
        const out = placeTooltip({ left: 0, top: 20, width: 10, height: 10 }, { width: 80, height: 900 }, viewport);
        expect(out.placement).toBe("top");
    });

    it("clamps horizontally inside the viewport", () => {
        expect(placeTooltip({ left: 0, top: 400, width: 10, height: 10 }, tip, viewport).left).toBe(8);
        expect(placeTooltip({ left: 995, top: 400, width: 10, height: 10 }, tip, viewport).left).toBe(1000 - 8 - 80);
    });
});
