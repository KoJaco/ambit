import { describe, expect, it } from "vitest";
import { canvasEdgeType, canvasNodeType, isStackedCard, nodeLayoutSize, stackOffset } from "./canvas-presentation";

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

    it("renders open groups as containers and collapsed groups as summary cards", () => {
        expect(canvasNodeType("card", 3, true)).toBe("container");
        expect(canvasNodeType("schema", 3, true)).toBe("container");
        expect(canvasNodeType("group", 3, false)).toBe("card");
        expect(canvasNodeType("group", 0, false)).toBe("group");
        expect(canvasNodeType(undefined, 3, false)).toBe("card");
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

describe("stacked summary card footprint", () => {
    it("reserves the sheet offset on the right and bottom only for stacked cards", () => {
        const flat = nodeLayoutSize("card");
        const stacked = nodeLayoutSize("card", 0, true);
        expect(stacked).toEqual({ width: flat.width + stackOffset, height: flat.height + stackOffset });
    });

    it("stacks only collapsed card nodes with children", () => {
        expect(isStackedCard("card", 3, false)).toBe(true);
        expect(isStackedCard("group", 3, false)).toBe(true);
        expect(isStackedCard("card", 0, false)).toBe(false);
        expect(isStackedCard("card", 3, true)).toBe(false);
        expect(isStackedCard("schema", 3, false)).toBe(false);
    });
});
