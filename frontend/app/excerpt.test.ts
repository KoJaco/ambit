import { describe, expect, it } from "vitest";
import { specExcerpt } from "./excerpt";

describe("specExcerpt", () => {
    it("returns empty for missing or blank specs", () => {
        expect(specExcerpt(undefined)).toBe("");
        expect(specExcerpt("  \n ")).toBe("");
    });
    it("collapses whitespace and keeps short text whole", () => {
        expect(specExcerpt("a\n\n  b")).toBe("a b");
    });
    it("truncates with an ellipsis within the limit", () => {
        const out = specExcerpt("word ".repeat(200), 280);
        expect(out.endsWith("…")).toBe(true);
        expect(out.length).toBeLessThanOrEqual(281);
    });
});
