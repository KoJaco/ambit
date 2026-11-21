import { describe, expect, it } from "vitest";
import { floatingToggleMode } from "./right-rail-state";

describe("floatingToggleMode", () => {
    it("hides, shifts or leaves the floating toggle", () => {
        expect(floatingToggleMode({ panels: 0, toggles: 0 })).toBe("normal");
        expect(floatingToggleMode({ panels: 1, toggles: 0 })).toBe("shifted");
        expect(floatingToggleMode({ panels: 2, toggles: 1 })).toBe("hidden");
    });
});
