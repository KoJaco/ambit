import { describe, expect, it } from "vitest";
import { computeGroupSizes, containerPadding, containerSize, layoutCompound, layoutLevel, maxSizes, pushOverlaps, seedPositions } from "./layout";

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

describe("pushOverlaps", () => {
    const nodes = [{ id: "g" }, { id: "right" }, { id: "below" }, { id: "far" }, { id: "other", parentId: "p" }, { id: "p" }];
    const old = { g: { width: 220, height: 96 } };

    it("pushes an overlapped sibling right just far enough and never moves the grower", () => {
        const pos = { g: { x: 0, y: 0 }, right: { x: 260, y: 0 }, below: { x: 0, y: 500 }, far: { x: 2000, y: 0 }, p: { x: 0, y: 900 }, other: { x: 0, y: 0 } };
        const out = pushOverlaps(nodes, pos, old, { g: { width: 500, height: 96 } });
        expect(out.g).toEqual({ x: 0, y: 0 });
        expect(out.right).toEqual({ x: 540, y: 0 });
        expect(out.below).toEqual({ x: 0, y: 500 });
        expect(out.far).toEqual({ x: 2000, y: 0 });
    });

    it("pushes down for siblings below and cascades to keep order", () => {
        const pos = { g: { x: 0, y: 0 }, right: { x: 260, y: 0 }, below: { x: 0, y: 136 }, far: { x: 600, y: 0 }, p: { x: 0, y: 900 }, other: { x: 5, y: 5 } };
        const out = pushOverlaps(nodes, pos, old, { g: { width: 500, height: 300 } });
        expect(out.below.y).toBe(340);
        expect(out.right.x).toBe(540);
        // "far" was clear of g but is now run into by "right" and moves on, keeping its place in the row.
        expect(out.far.x).toBeGreaterThan(out.right.x + 220);
    });

    it("leaves nodes alone when nothing grew or nothing overlaps", () => {
        const pos = { g: { x: 0, y: 0 }, right: { x: 260, y: 0 }, below: { x: 0, y: 136 }, far: { x: 600, y: 0 }, p: { x: 0, y: 900 }, other: { x: 0, y: 0 } };
        expect(pushOverlaps(nodes, pos, old, { g: { width: 220, height: 96 } })).toEqual(pos);
        // Growth below the 0.5px tolerance is ignored; real growth only moves by what it takes.
        expect(pushOverlaps(nodes, pos, old, { g: { width: 220.4, height: 96 } })).toEqual(pos);
        expect(pushOverlaps(nodes, pos, old, { g: { width: 230, height: 96 } }).right.x).toBe(270);
    });

    it("only interacts with siblings under the same parent", () => {
        const pos = { g: { x: 0, y: 0 }, right: { x: 2000, y: 0 }, below: { x: 0, y: 2000 }, far: { x: 3000, y: 0 }, p: { x: 0, y: 900 }, other: { x: 300, y: 0 } };
        const out = pushOverlaps(nodes, pos, old, { g: { width: 600, height: 96 } });
        expect(out.other).toEqual({ x: 300, y: 0 });
    });
});

describe("seedPositions with external stubs", () => {
    it("keeps a dragged stub where it is across a toggle", () => {
        const nodes = [{ id: "a" }, { id: "ext:r1" }, { id: "b" }];
        const placed = [
            { id: "a", position: { x: 0, y: 0 } },
            { id: "ext:r1", position: { x: 777, y: -55 } },
        ];
        const out = seedPositions(nodes, placed, {});
        expect(out["ext:r1"]).toEqual({ x: 777, y: -55 });
        expect(out.a).toEqual({ x: 0, y: 0 });
    });
});

describe("seedPositions on membership change", () => {
    const sizes = { a: { width: 220, height: 96 }, b: { width: 220, height: 96 }, c: { width: 220, height: 96 } };

    it("keeps hand-arranged top-level nodes and places a new node beside them", () => {
        const placed = [
            { id: "a", position: { x: 500, y: 40 } },
            { id: "b", position: { x: 30, y: 700 } },
        ];
        const out = seedPositions([{ id: "a" }, { id: "b" }, { id: "c" }], placed, sizes);
        expect(out.a).toEqual({ x: 500, y: 40 });
        expect(out.b).toEqual({ x: 30, y: 700 });
        expect(out.c.x).toBeGreaterThan(500);
    });

    it("places a new group around members that were on screen, which do not move", () => {
        const placed = [
            { id: "a", position: { x: 500, y: 300 } },
            { id: "b", position: { x: 800, y: 340 } },
            { id: "c", position: { x: 0, y: 0 } },
        ];
        const nodes = [{ id: "g" }, { id: "a", parentId: "g" }, { id: "b", parentId: "g" }, { id: "c" }];
        const out = seedPositions(nodes, placed, sizes);
        expect(out.c).toEqual({ x: 0, y: 0 });
        expect(out.g).toEqual({ x: 500 - containerPadding.left, y: 300 - containerPadding.top });
        // Absolute position of each member is unchanged.
        expect(out.a.x + out.g.x).toBe(500);
        expect(out.a.y + out.g.y).toBe(300);
        expect(out.b.x + out.g.x).toBe(800);
        expect(out.b.y + out.g.y).toBe(340);
    });

    it("places a node moved into an existing group fresh, inside the group", () => {
        const placed = [
            { id: "g", position: { x: 100, y: 100 } },
            { id: "a", position: { x: 900, y: 900 } },
        ];
        const out = seedPositions([{ id: "g" }, { id: "a", parentId: "g" }], placed, sizes);
        expect(out.g).toEqual({ x: 100, y: 100 });
        expect(out.a).toEqual({ x: containerPadding.left, y: containerPadding.top });
    });

    it("places a node that left its group beside the top-level nodes, not on the old group", () => {
        const placed = [
            { id: "g", position: { x: 100, y: 100 } },
            { id: "a", parentId: "g", position: { x: 30, y: 60 } },
        ];
        const out = seedPositions([{ id: "g" }, { id: "a" }], placed, sizes);
        expect(out.g).toEqual({ x: 100, y: 100 });
        expect(out.a.x).toBeGreaterThan(100 + 220);
    });
});

describe("group shrink after members leave", () => {
    it("computes a smaller size from the remaining members", () => {
        const sizes = { a: { width: 220, height: 96 }, b: { width: 220, height: 96 } };
        const nodes2 = [{ id: "g" }, { id: "a", parentId: "g" }, { id: "b", parentId: "g" }];
        const two = computeGroupSizes(nodes2, { g: { x: 0, y: 0 }, a: { x: 16, y: 48 }, b: { x: 16, y: 400 } }, sizes);
        const one = computeGroupSizes(nodes2.slice(0, 2), { g: { x: 0, y: 0 }, a: { x: 16, y: 48 } }, sizes);
        expect(one.g.height).toBeLessThan(two.g.height);
    });
});

describe("new container and orphaned members", () => {
    const sizes = { a: { width: 220, height: 96 }, b: { width: 220, height: 96 }, c: { width: 220, height: 96 } };

    it("pushes a non-member sibling that sits between the members of a new container", () => {
        const placed = [
            { id: "a", position: { x: 500, y: 300 } },
            { id: "b", position: { x: 800, y: 340 } },
            { id: "c", position: { x: 650, y: 320 } },
        ];
        const nodes = [{ id: "g" }, { id: "a", parentId: "g" }, { id: "b", parentId: "g" }, { id: "c" }];
        const seeded = seedPositions(nodes, placed, sizes);
        const newSizes: Record<string, { width: number; height: number }> = { ...sizes, ...computeGroupSizes(nodes, seeded, sizes) };
        const out = pushOverlaps(nodes, seeded, {}, newSizes);
        const g = out.g;
        expect(g).toEqual(seeded.g);
        const box = { ...g, ...newSizes.g };
        const c = out.c;
        const clearsX = c.x >= box.x + box.width || c.x + 220 <= box.x;
        const clearsY = c.y >= box.y + box.height || c.y + 96 <= box.y;
        expect(clearsX || clearsY).toBe(true);
    });

    it("keeps the absolute spot of a member whose old parent was deleted", () => {
        const placed = [
            { id: "g", position: { x: 400, y: 200 } },
            { id: "a", parentId: "g", position: { x: 16, y: 48 } },
        ];
        // g is gone (ungroup with delete); a is now top level.
        const out = seedPositions([{ id: "a" }], placed, sizes);
        expect(out.a).toEqual({ x: 416, y: 248 });
    });
});
