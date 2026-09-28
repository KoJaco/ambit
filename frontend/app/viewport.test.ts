import { describe, expect, it } from "vitest";
import { isDesktopViewport } from "./viewport";

describe("isDesktopViewport", () => {
    it("allows desktop-sized viewports", () => {
        expect(isDesktopViewport(1280, 800)).toBe(true);
        expect(isDesktopViewport(900, 600)).toBe(true);
    });

    it("blocks typical phone sizes", () => {
        expect(isDesktopViewport(390, 844)).toBe(false);
        expect(isDesktopViewport(800, 500)).toBe(false);
    });
});
