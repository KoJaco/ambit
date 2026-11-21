import { describe, expect, it } from "vitest";
import { edgeKindStyle } from "./kind-style";

describe("edgeKindStyle", () => {
    it("maps the known kinds", () => {
        expect(edgeKindStyle("sync")).toMatchObject({ id: "sync", animated: false, glyph: "arrow" });
        expect(edgeKindStyle("sync").dash).toBeUndefined();
        expect(edgeKindStyle("async")).toMatchObject({ id: "async", dash: "6 6", animated: true, glyph: "dots" });
        expect(edgeKindStyle("data")).toMatchObject({ id: "data", animated: false, glyph: "data" });
    });
    it("is case and whitespace tolerant", () => {
        expect(edgeKindStyle(" Async ").id).toBe("async");
    });
    it("falls back to a neutral solid style", () => {
        expect(edgeKindStyle("grpc")).toMatchObject({ id: "unknown", label: "grpc", animated: false, glyph: "arrow" });
        expect(edgeKindStyle("").label).toBe("Unspecified");
        expect(edgeKindStyle(undefined).dash).toBeUndefined();
    });
    it("always describes the kind", () => {
        for (const k of ["sync", "async", "data", "x"]) expect(edgeKindStyle(k).description.length).toBeGreaterThan(10);
    });
});
