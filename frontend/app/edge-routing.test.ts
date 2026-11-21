import { describe, expect, it } from "vitest";
import {
    chooseSides,
    laneOffsets,
    pairSign,
    reciprocalOffset,
    routeEdges,
    sideNormal,
    polylineBlocked,
    type Rect,
} from "./edge-routing";

const r = (x: number, y: number, w = 100, h = 60): Rect => ({ x, y, w, h });

describe("chooseSides", () => {
    it("forward exits right and enters left", () => {
        expect(chooseSides(r(0, 0), r(300, 0))).toEqual({ source: "right", target: "left" });
    });
    it("backward level uses bottom/bottom", () => {
        expect(chooseSides(r(300, 0), r(0, 0))).toEqual({ source: "bottom", target: "bottom" });
    });
    it("backward with target above uses top/top", () => {
        expect(chooseSides(r(300, 300), r(0, 0))).toEqual({ source: "top", target: "top" });
    });
    it("stacked uses bottom->top and top->bottom", () => {
        expect(chooseSides(r(0, 0), r(20, 200))).toEqual({ source: "bottom", target: "top" });
        expect(chooseSides(r(0, 200), r(20, 0))).toEqual({ source: "top", target: "bottom" });
    });
});

describe("pair and lane offsets", () => {
    it("reciprocal pair gets opposite signs", () => {
        expect(pairSign("a", "b")).toBe(1);
        expect(pairSign("b", "a")).toBe(-1);
        expect(reciprocalOffset("a", "b", true)).toBe(8);
        expect(reciprocalOffset("b", "a", true)).toBe(-8);
        expect(reciprocalOffset("a", "b", false)).toBe(0);
    });
    it("spreads lanes symmetrically and distinctly", () => {
        const o = laneOffsets(3, 60);
        expect(o).toEqual([-10, 0, 10]);
        expect(new Set(laneOffsets(5, 60)).size).toBe(5);
        expect(laneOffsets(1, 60)).toEqual([0]);
    });
    it("fans out two edges leaving the same side", () => {
        const rects = new Map([["a", r(0, 100)], ["b", r(300, 0)], ["c", r(300, 200)]]);
        const routes = routeEdges([{ id: "1", source: "a", target: "b" }, { id: "2", source: "a", target: "c" }], rects);
        const y1 = routes.get("1")!.points[0].y;
        const y2 = routes.get("2")!.points[0].y;
        expect(y1).toBeLessThan(y2);
    });
    it("spreads three parallel edges between the same nodes at least 10px apart", () => {
        const rects = new Map([["a", r(0, 0, 100, 120)], ["b", r(300, 0, 100, 120)]]);
        const routes = routeEdges(["1", "2", "3"].map((id) => ({ id, source: "a", target: "b" })), rects);
        const ys = ["1", "2", "3"].map((id) => routes.get(id)!.points[0].y).sort((x, y) => x - y);
        expect(ys[1] - ys[0]).toBeGreaterThanOrEqual(10);
        expect(ys[2] - ys[1]).toBeGreaterThanOrEqual(10);
    });
    it("separates a reciprocal pair into different lanes", () => {
        const rects = new Map([["a", r(0, 0)], ["b", r(300, 0)]]);
        const routes = routeEdges([{ id: "1", source: "a", target: "b" }, { id: "2", source: "b", target: "a" }], rects);
        expect(routes.get("1")!.points[0]).not.toEqual(routes.get("2")!.points.at(-1));
        expect(routes.get("2")!.sourceSide).toBe("bottom");
    });
});

describe("arrowhead direction", () => {
    it("final segment enters the target against the side normal", () => {
        const cases: [Rect, Rect][] = [
            [r(0, 0), r(300, 0)],
            [r(300, 0), r(0, 0)],
            [r(300, 300), r(0, 0)],
            [r(0, 0), r(20, 200)],
            [r(0, 200), r(20, 0)],
        ];
        for (const [a, b] of cases) {
            const route = routeEdges([{ id: "e", source: "a", target: "b" }], new Map([["a", a], ["b", b]])).get("e")!;
            const pts = route.points;
            const [p, q] = [pts[pts.length - 2], pts[pts.length - 1]];
            const n = sideNormal(route.targetSide);
            expect(Math.sign(q.x - p.x)).toBe(-n.x || 0);
            expect(Math.sign(q.y - p.y)).toBe(-n.y || 0);
        }
    });
});

describe("obstacle avoidance", () => {
    it("routes a blocked forward edge through a free channel", () => {
        const rects = new Map([["a", r(0, 0)], ["m", r(200, 0)], ["b", r(400, 0)]]);
        const route = routeEdges([{ id: "e", source: "a", target: "b" }], rects).get("e")!;
        expect(polylineBlocked(route.points, [rects.get("m")!])).toBe(false);
    });
    it("ignores ancestor groups that contain both endpoints", () => {
        const rects = new Map([["g", r(-50, -50, 700, 300)], ["a", r(0, 0)], ["b", r(400, 0)]]);
        const route = routeEdges([{ id: "e", source: "a", target: "b" }], rects).get("e")!;
        expect(route.points).toHaveLength(2);
    });
});
