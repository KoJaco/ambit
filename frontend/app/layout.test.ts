import { describe, expect, it } from "vitest";
import { computeGroupSizes, containerPadding, containerSize, layoutCompound, layoutLevel } from "./layout";

describe("layoutCompound", () => {
    it("lays out a flat level like layoutLevel", async () => {
        const edges = [{ from: "a", to: "b" }];
        const flat = await layoutLevel(["a", "b"], edges);
        const compound = await layoutCompound([{ id: "a" }, { id: "b" }], edges);
        expect(compound.positions).toEqual(flat);
        expect(compound.groupSizes).toEqual({});
    });

    it("nests members inside a group with relative positions and header padding", async () => {
        const result = await layoutCompound(
            [{ id: "g" }, { id: "m1", parentId: "g" }, { id: "m2", parentId: "g" }, { id: "other" }],
            [
                { from: "m1", to: "m2" },
                { from: "m2", to: "other" },
                { from: "g", to: "m1" },
            ],
        );
        for (const id of ["m1", "m2"]) {
            expect(result.positions[id].x).toBeGreaterThanOrEqual(containerPadding.left);
            expect(result.positions[id].y).toBeGreaterThanOrEqual(containerPadding.top);
        }
        const size = result.groupSizes.g;
        expect(size.width).toBeGreaterThan(220 * 2);
        for (const id of ["m1", "m2"]) {
            expect(result.positions[id].x + 220).toBeLessThanOrEqual(size.width);
            expect(result.positions[id].y + 96).toBeLessThanOrEqual(size.height);
        }
        expect(result.positions.other).toBeDefined();
    });

    it("supports two levels of nesting", async () => {
        const result = await layoutCompound(
            [{ id: "g" }, { id: "h", parentId: "g" }, { id: "x", parentId: "h" }, { id: "y", parentId: "g" }],
            [{ from: "x", to: "y" }],
        );
        expect(result.groupSizes.h).toBeDefined();
        expect(result.groupSizes.g.width).toBeGreaterThan(result.groupSizes.h.width);
    });
});

describe("containerSize", () => {
    it("wraps members plus padding with a minimum", () => {
        const size = containerSize(["a"], { a: { x: 16, y: 48 } }, { a: { width: 100, height: 50 } });
        expect(size.width).toBeGreaterThanOrEqual(116 + containerPadding.right);
        expect(size.height).toBeGreaterThanOrEqual(98 + containerPadding.bottom);
        expect(containerSize([], {}, {}).width).toBeGreaterThan(0);
    });
});

describe("computeGroupSizes", () => {
    it("sizes nested containers bottom-up", () => {
        const nodes = [{ id: "g" }, { id: "h", parentId: "g" }, { id: "x", parentId: "h" }];
        const sizes = computeGroupSizes(
            nodes,
            { g: { x: 0, y: 0 }, h: { x: 16, y: 48 }, x: { x: 16, y: 48 } },
            { x: { width: 220, height: 96 } },
        );
        expect(sizes.h.width).toBe(16 + 220 + containerPadding.right);
        expect(sizes.h.height).toBe(48 + 96 + containerPadding.bottom);
        expect(sizes.g.width).toBeGreaterThan(sizes.h.width);
        expect(sizes.g.height).toBeGreaterThan(sizes.h.height);
        expect(sizes.x).toBeUndefined();
    });
});
