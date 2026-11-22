import { describe, expect, it } from "vitest";
import { avoidLabelCollisions, estimatePillSize, labelGap, layoutEdgeLabels, maxLabelShift, type LabelBox } from "./label-layout";
import { routeEdges, type Rect } from "./edge-routing";

const horiz = (x: number, y: number): LabelBox["points"] => [
    { x: x - 100, y },
    { x: x + 100, y },
];
const box = (id: string, x: number, y: number, w = 120, h = 24): LabelBox => ({ id, x, y, w, h, points: horiz(x, y) });

function overlaps(a: LabelBox, p: { x: number; y: number }, b: LabelBox, q: { x: number; y: number }) {
    return Math.abs(p.x - q.x) < (a.w + b.w) / 2 && Math.abs(p.y - q.y) < (a.h + b.h) / 2;
}

describe("avoidLabelCollisions", () => {
    it("leaves non-colliding pills where they are", () => {
        const out = avoidLabelCollisions([box("a", 0, 0), box("b", 0, 100)]);
        expect(out.get("a")).toEqual({ x: 0, y: 0 });
        expect(out.get("b")).toEqual({ x: 0, y: 100 });
    });
    it("separates identical anchors so no two pills overlap", () => {
        const boxes = ["a", "b", "c", "d"].map((id) => box(id, 50, 50));
        const out = avoidLabelCollisions(boxes);
        for (let i = 0; i < boxes.length; i++)
            for (let j = i + 1; j < boxes.length; j++)
                expect(overlaps(boxes[i], out.get(boxes[i].id)!, boxes[j], out.get(boxes[j].id)!)).toBe(false);
    });
    it("keeps pills near their anchors", () => {
        const boxes = ["a", "b", "c"].map((id) => box(id, 0, 0));
        const out = avoidLabelCollisions(boxes);
        for (const b of boxes) {
            const p = out.get(b.id)!;
            expect(Math.hypot(p.x - b.x, p.y - b.y)).toBeLessThanOrEqual(maxLabelShift);
        }
    });
    it("keeps the minimum gap between neighbours", () => {
        const out = avoidLabelCollisions([box("a", 0, 0), box("b", 0, 10)]);
        const dy = Math.abs(out.get("a")!.y - out.get("b")!.y);
        const dx = Math.abs(out.get("a")!.x - out.get("b")!.x);
        expect(dy >= 24 + labelGap || dx >= 120 + labelGap).toBe(true);
    });
    it("is independent of input order and repeatable", () => {
        const boxes = ["c", "a", "b"].map((id) => box(id, 10, 10));
        const one = avoidLabelCollisions(boxes);
        const two = avoidLabelCollisions([...boxes].reverse());
        expect([...one].sort()).toEqual([...two].sort());
        expect(avoidLabelCollisions(boxes)).toEqual(one);
    });
    it("uses the segment normal for vertical segments", () => {
        const v = (id: string): LabelBox => ({ id, x: 0, y: 0, w: 40, h: 24, points: [{ x: 0, y: -200 }, { x: 0, y: 200 }] });
        const out = avoidLabelCollisions([v("a"), v("b")]);
        expect(overlaps(v("a"), out.get("a")!, v("b"), out.get("b")!)).toBe(false);
    });
    it("handles an empty list", () => {
        expect(avoidLabelCollisions([]).size).toBe(0);
    });
});

describe("estimatePillSize", () => {
    it("caps the width and is single-line height", () => {
        expect(estimatePillSize("x".repeat(200)).w).toBe(176);
        expect(estimatePillSize("ab").h).toBe(24);
        expect(estimatePillSize("ab", 14).w).toBeGreaterThan(estimatePillSize("ab").w);
    });
});

describe("layoutEdgeLabels", () => {
    it("separates labels of parallel edges between the same nodes", () => {
        const rects = new Map<string, Rect>([["a", { x: 0, y: 0, w: 100, h: 60 }], ["b", { x: 400, y: 0, w: 100, h: 60 }]]);
        const edges = ["1", "2", "3"].map((id) => ({ id, source: "a", target: "b" }));
        const routes = routeEdges(edges, rects);
        const sizes = new Map(edges.map((e) => [e.id, estimatePillSize("integrates")]));
        const out = layoutEdgeLabels(routes, sizes);
        const ys = edges.map((e) => out.get(e.id)!);
        for (let i = 0; i < ys.length; i++)
            for (let j = i + 1; j < ys.length; j++) {
                const dx = Math.abs(ys[i].x - ys[j].x);
                const dy = Math.abs(ys[i].y - ys[j].y);
                expect(dx >= sizes.get("1")!.w || dy >= sizes.get("1")!.h).toBe(true);
            }
    });
    it("passes through edges without a pill", () => {
        const rects = new Map<string, Rect>([["a", { x: 0, y: 0, w: 100, h: 60 }], ["b", { x: 400, y: 0, w: 100, h: 60 }]]);
        const routes = routeEdges([{ id: "1", source: "a", target: "b" }], rects);
        const out = layoutEdgeLabels(routes, new Map());
        expect(out.get("1")).toEqual({ x: routes.get("1")!.labelX, y: routes.get("1")!.labelY });
    });
});

describe("node obstacles", () => {
    const hitsRect = (p: { x: number; y: number }, b: LabelBox, r: Rect) =>
        Math.abs(p.x - (r.x + r.w / 2)) < (b.w + r.w) / 2 && Math.abs(p.y - (r.y + r.h / 2)) < (b.h + r.h) / 2;
    it("moves a pill off a node", () => {
        const node: Rect = { x: -10, y: -30, w: 20, h: 60 };
        const b = box("a", 0, 0);
        const p = avoidLabelCollisions([b], [node]).get("a")!;
        expect(hitsRect(p, b, node)).toBe(false);
        expect(p.y).toBe(0);
    });
    it("leaves a pill that is clear of nodes", () => {
        const out = avoidLabelCollisions([box("a", 0, 0)], [{ x: 500, y: 500, w: 80, h: 60 }]);
        expect(out.get("a")).toEqual({ x: 0, y: 0 });
    });
    it("falls back deterministically when nothing is free", () => {
        const wall: Rect = { x: -2000, y: -2000, w: 4000, h: 4000 };
        const b = box("a", 0, 0);
        const first = avoidLabelCollisions([b], [wall]).get("a");
        expect(avoidLabelCollisions([b], [wall]).get("a")).toEqual(first);
        expect(first).toBeDefined();
    });
    it("is independent of input order", () => {
        const nodes: Rect[] = [{ x: -40, y: -30, w: 80, h: 60 }, { x: 60, y: -30, w: 80, h: 60 }];
        const boxes = ["a", "b", "c"].map((id) => box(id, 0, 0));
        const one = avoidLabelCollisions(boxes, nodes);
        const two = avoidLabelCollisions([...boxes].reverse(), [...nodes].reverse());
        for (const id of ["a", "b", "c"]) expect(two.get(id)).toEqual(one.get(id));
    });
    it("layoutEdgeLabels keeps pills off supplied rects only", () => {
        const rects = new Map<string, Rect>([["a", { x: 0, y: 0, w: 100, h: 60 }], ["b", { x: 400, y: 0, w: 100, h: 60 }]]);
        const routes = routeEdges([{ id: "1", source: "a", target: "b" }], rects);
        const sizes = new Map([["1", estimatePillSize("stores")]]);
        const anchor = { x: routes.get("1")!.labelX, y: routes.get("1")!.labelY };
        const blocker: Rect = { x: anchor.x - 30, y: anchor.y - 30, w: 60, h: 60 };
        const moved = layoutEdgeLabels(routes, sizes, [blocker]).get("1")!;
        expect(moved).not.toEqual(anchor);
        expect(layoutEdgeLabels(routes, sizes, []).get("1")).toEqual(anchor);
    });
});
