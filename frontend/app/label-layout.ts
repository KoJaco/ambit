// Pure label collision avoidance for edge pills. Each pill is a box centred on its anchor. Boxes are
// placed greedily in a stable order (top to bottom, left to right, then id); a box that would overlap
// an already placed one tries nearby positions, nearest first: sliding along its own edge segment,
// then stepping along the segment normal. Node rectangles are obstacles like other pills. The result depends only on the input values, never on
// input order. Cost is O(n^2 * candidates), fine for the tens of pills a level shows.
import type { EdgeRoute, Point, Rect } from "./edge-routing";

export type LabelBox = { id: string; x: number; y: number; w: number; h: number; points?: Point[] };

export const labelGap = 4;
export const maxLabelShift = 96;
export const pillMaxWidth = 176;
export const pillHeight = 24;
const slideStep = 12;
const slideInset = 8;
const maxCandidatesPerAxis = 12;

// Width estimate for a single-line pill (11px text) so layout does not need DOM measurement.
export function estimatePillSize(label: string, extraWidth = 0): { w: number; h: number } {
    return { w: Math.min(pillMaxWidth, Math.ceil(label.length * 6.3) + 22 + extraWidth), h: pillHeight };
}

function overlap(a: Pick<LabelBox, "x" | "y" | "w" | "h">, b: Pick<LabelBox, "x" | "y" | "w" | "h">): number {
    const ox = (a.w + b.w) / 2 + labelGap - Math.abs(a.x - b.x);
    const oy = (a.h + b.h) / 2 + labelGap - Math.abs(a.y - b.y);
    return ox > 0 && oy > 0 ? ox * oy : 0;
}

// Overlap with a node rectangle (top-left origin), including the same gap kept between pills.
function overlapRect(a: Pick<LabelBox, "x" | "y" | "w" | "h">, r: Rect): number {
    const ox = (a.w + r.w) / 2 + labelGap - Math.abs(a.x - (r.x + r.w / 2));
    const oy = (a.h + r.h) / 2 + labelGap - Math.abs(a.y - (r.y + r.h / 2));
    return ox > 0 && oy > 0 ? ox * oy : 0;
}

type Candidate = { x: number; y: number; dist: number };

function longestSegment(points: Point[] | undefined): { a: Point; b: Point } | null {
    if (!points || points.length < 2) return null;
    let best = { a: points[0], b: points[1], len: -1 };
    for (let i = 0; i < points.length - 1; i++) {
        const len = Math.hypot(points[i + 1].x - points[i].x, points[i + 1].y - points[i].y);
        if (len > best.len) best = { a: points[i], b: points[i + 1], len };
    }
    return best.len > 0 ? best : null;
}

export function labelCandidates(box: LabelBox): Candidate[] {
    const seg = longestSegment(box.points);
    let dir = { x: 1, y: 0 };
    let tMin = 0;
    let tMax = 0;
    if (seg) {
        const len = Math.hypot(seg.b.x - seg.a.x, seg.b.y - seg.a.y);
        dir = { x: (seg.b.x - seg.a.x) / len, y: (seg.b.y - seg.a.y) / len };
        const t = (box.x - seg.a.x) * dir.x + (box.y - seg.a.y) * dir.y;
        tMin = slideInset - t;
        tMax = len - slideInset - t;
    }
    const normal = { x: -dir.y, y: dir.x };
    const out: Candidate[] = [{ x: box.x, y: box.y, dist: 0 }];
    for (let k = 1; k <= maxCandidatesPerAxis; k++) {
        for (const sign of [-1, 1]) {
            const d = sign * k * slideStep;
            if (d < tMin || d > tMax) continue;
            out.push({ x: box.x + dir.x * d, y: box.y + dir.y * d, dist: Math.abs(d) });
        }
    }
    const slides = out.sort((p, q) => p.dist - q.dist || p.y - q.y || p.x - q.x);
    const extent = Math.abs(normal.x) * box.w + Math.abs(normal.y) * box.h + labelGap;
    const limit = Math.max(maxLabelShift, extent);
    const steps: Candidate[] = [];
    for (let k = 1; k <= maxCandidatesPerAxis && k * extent <= limit; k++) {
        for (const sign of [-1, 1]) {
            const d = sign * k * extent;
            steps.push({ x: box.x + normal.x * d, y: box.y + normal.y * d, dist: Math.abs(d) });
        }
    }
    // Slides along the segment first, then steps across it.
    return [...slides, ...steps.sort((p, q) => p.dist - q.dist || p.y - q.y || p.x - q.x)];
}

// `obstacles` are node bodies (absolute, top-left rects). Pills are kept off them like off other pills.
export function avoidLabelCollisions(boxes: LabelBox[], obstacles: Rect[] = []): Map<string, Point> {
    const order = [...boxes].sort((a, b) => a.y - b.y || a.x - b.x || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0));
    const placed: LabelBox[] = [];
    const result = new Map<string, Point>();
    for (const box of order) {
        let best: { c: Candidate; area: number } | null = null;
        for (const c of labelCandidates(box)) {
            const moved = { ...box, x: c.x, y: c.y };
            let area = 0;
            for (const other of placed) area += overlap(moved, other);
            for (const rect of obstacles) area += overlapRect(moved, rect);
            if (area === 0) {
                best = { c, area };
                break;
            }
            if (!best || area < best.area) best = { c, area };
        }
        const c = best!.c;
        placed.push({ ...box, x: c.x, y: c.y });
        result.set(box.id, { x: c.x, y: c.y });
    }
    return result;
}

// Convenience for the canvas: nudge the label anchors of routed edges. Edges without a size entry
// (no pill) keep their anchor and take no space.
export function layoutEdgeLabels(
    routes: Map<string, EdgeRoute>,
    sizes: Map<string, { w: number; h: number }>,
    obstacles: Rect[] = [],
): Map<string, Point> {
    const boxes: LabelBox[] = [];
    const out = new Map<string, Point>();
    for (const [id, route] of routes) {
        const size = sizes.get(id);
        if (!size) {
            out.set(id, { x: route.labelX, y: route.labelY });
            continue;
        }
        boxes.push({ id, x: route.labelX, y: route.labelY, w: size.w, h: size.h, points: route.points });
    }
    for (const [id, p] of avoidLabelCollisions(boxes, obstacles)) out.set(id, p);
    return out;
}
