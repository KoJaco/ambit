import { describe, expect, it } from "vitest";
import { canvasEdgeType, canvasNodeType, nodeLayoutSize } from "./canvas-presentation";

describe("canvas presentation", () => {
    it("maps renderer ids onto canvas node types", () => {
        expect(canvasNodeType(undefined)).toBe("card");
        expect(canvasNodeType("")).toBe("card");
        expect(canvasNodeType("card")).toBe("card");
        expect(canvasNodeType("unknown")).toBe("card");
        expect(canvasNodeType("schema")).toBe("schema");
        expect(canvasNodeType("placeholder")).toBe("placeholder");
        expect(canvasNodeType("group")).toBe("group");
    });

    it("maps relationship kind onto edge types", () => {
        expect(canvasEdgeType("sync")).toBe("button");
        expect(canvasEdgeType("async")).toBe("animated");
        expect(canvasEdgeType("data")).toBe("data");
        expect(canvasEdgeType("other")).toBe("arch");
        expect(canvasEdgeType(undefined)).toBe("arch");
    });

    it("gives schema and group nodes a larger layout box", () => {
        expect(nodeLayoutSize("card")).toEqual({ width: 220, height: 96 });
        expect(nodeLayoutSize("schema", 4).height).toBeGreaterThan(96);
        expect(nodeLayoutSize("group").width).toBeGreaterThan(220);
    });
});
