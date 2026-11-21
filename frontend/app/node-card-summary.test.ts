import { describe, expect, it } from "vitest";
import { fullSummaryText, typeMixText } from "./node-card-summary";

describe("node card summary", () => {
    it("lists every type in the mix without truncating", () => {
        const mix = [
            { type: "service", count: 8 },
            { type: "module", count: 2 },
            { type: "data store", count: 1 },
            { type: "queue", count: 1 },
            { type: "", count: 1 },
        ];
        expect(typeMixText(mix)).toBe("8 service, 2 module, 1 data store, 1 queue, 1 untyped");
        expect(fullSummaryText(13, mix)).toBe("13 · 8 service, 2 module, 1 data store, 1 queue, 1 untyped");
    });

    it("falls back to the bare count", () => {
        expect(fullSummaryText(3)).toBe("3");
        expect(fullSummaryText(3, [])).toBe("3");
    });
});
