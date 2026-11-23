import { describe, expect, it } from "vitest";
import { computeGroupSizes, containerPadding, containerSize, layoutCompound, layoutLevel, maxSizes, seedPositions } from "./layout";

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

    it("keeps the member footprint as the floor and honours larger overrides", () => {
        const nodes = [{ id: "g" }, { id: "x", parentId: "g" }];
        const positions = { g: { x: 0, y: 0 }, x: { x: 16, y: 48 } };
        const leaf = { x: { width: 232, height: 108 } };
        const min = computeGroupSizes(nodes, positions, leaf);
        expect(min.g).toEqual({ width: 16 + 232 + containerPadding.right, height: 48 + 108 + containerPadding.bottom });
        const bigger = computeGroupSizes(nodes, positions, leaf, { g: { width: 900, height: 700 } });
        expect(bigger.g).toEqual({ width: 900, height: 700 });
        const smaller = computeGroupSizes(nodes, positions, leaf, { g: { width: 10, height: 10 } });
        expect(smaller.g).toEqual(min.g);
    });

    it("lets a resized nested container raise its parent's minimum", () => {
        const nodes = [{ id: "g" }, { id: "h", parentId: "g" }, { id: "x", parentId: "h" }];
        const positions = { g: { x: 0, y: 0 }, h: { x: 16, y: 48 }, x: { x: 16, y: 48 } };
        const leaf = { x: { width: 220, height: 96 } };
        const base = computeGroupSizes(nodes, positions, leaf);
        const grown = computeGroupSizes(nodes, positions, leaf, { h: { width: base.h.width + 200, height: base.h.height } });
        expect(grown.g.width).toBe(base.g.width + 200);
    });
});

describe("seedPositions", () => {
    const sizes = { a: { width: 220, height: 96 }, b: { width: 220, height: 96 }, m1: { width: 220, height: 96 }, m2: { width: 220, height: 96 } };

    it("keeps placed nodes and places only newly revealed members inside their group", () => {
        const nodes = [{ id: "a" }, { id: "g" }, { id: "m1", parentId: "g" }, { id: "m2", parentId: "g" }];
        const out = seedPositions(
            nodes,
            [
                { id: "a", position: { x: 500, y: 7 } },
                { id: "g", position: { x: 10, y: 20 } },
            ],
            { ...sizes, g: { width: 220, height: 96 } },
        );
        expect(out.a).toEqual({ x: 500, y: 7 });
        expect(out.g).toEqual({ x: 10, y: 20 });
        expect(out.m1).toEqual({ x: containerPadding.left, y: containerPadding.top });
        expect(out.m2.x).toBeGreaterThan(out.m1.x + 220 - 1);
    });

    it("keeps the slot of a collapsed group and drops its members", () => {
        const out = seedPositions(
            [{ id: "a" }, { id: "g" }],
            [
                { id: "a", position: { x: 1, y: 2 } },
                { id: "g", position: { x: 30, y: 40 } },
                { id: "m1", parentId: "g", position: { x: 16, y: 48 } },
            ],
            sizes,
        );
        expect(out).toEqual({ a: { x: 1, y: 2 }, g: { x: 30, y: 40 } });
    });

    it("treats a node whose parent changed as new", () => {
        const out = seedPositions(
            [{ id: "g" }, { id: "m1", parentId: "g" }],
            [
                { id: "g", position: { x: 0, y: 0 } },
                { id: "m1", position: { x: 900, y: 900 } },
            ],
            sizes,
        );
        expect(out.m1).toEqual({ x: containerPadding.left, y: containerPadding.top });
    });

    it("uses hints only when every new sibling has one", () => {
        const nodes = [{ id: "g" }, { id: "m1", parentId: "g" }, { id: "m2", parentId: "g" }];
        const placed = [{ id: "g", position: { x: 0, y: 0 } }];
        const hinted = seedPositions(nodes, placed, sizes, { m1: { x: 5, y: 6 }, m2: { x: 7, y: 8 } });
        expect(hinted.m1).toEqual({ x: 5, y: 6 });
        const partial = seedPositions(nodes, placed, sizes, { m1: { x: 5, y: 6 } });
        expect(partial.m1).toEqual({ x: containerPadding.left, y: containerPadding.top });
    });

    it("places new top-level nodes to the right of existing ones", () => {
        const out = seedPositions([{ id: "a" }, { id: "b" }], [{ id: "a", position: { x: 100, y: 0 } }], sizes);
        expect(out.b.x).toBeGreaterThanOrEqual(100 + 220);
    });
});

describe("group growth on toggle", () => {
    const sizes = { m: { width: 220, height: 96 } };
    const nodes = [{ id: "g" }, { id: "m", parentId: "g" }];
    const positions = { g: { x: 0, y: 0 }, m: { x: 16, y: 48 } };

    it("never shrinks below the previous size", () => {
        const min = computeGroupSizes(nodes, positions, sizes).g;
        const floor = { g: { width: min.width + 300, height: min.height + 200 } };
        expect(computeGroupSizes(nodes, positions, sizes, maxSizes({}, floor)).g).toEqual(floor.g);
    });

    it("grows when the member bounds exceed the previous size", () => {
        const grown = computeGroupSizes(nodes, positions, { m: { width: 900, height: 700 } }, { g: { width: 300, height: 200 } }).g;
        expect(grown.width).toBeGreaterThan(900);
        expect(grown.height).toBeGreaterThan(700);
    });

    it("maxSizes takes the larger side per id", () => {
        expect(maxSizes({ a: { width: 1, height: 9 } }, { a: { width: 5, height: 2 }, b: { width: 3, height: 3 } })).toEqual({
            a: { width: 5, height: 9 },
            b: { width: 3, height: 3 },
        });
    });
});
