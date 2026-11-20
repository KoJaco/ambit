// Pure edge routing for the flow canvas: side-aware anchors, lane spreading, rounded orthogonal paths
// and a cheap channel search that keeps forward edges from cutting through unrelated nodes.

export type Rect = { x: number; y: number; w: number; h: number };
export type Side = "left" | "right" | "top" | "bottom";
export type Point = { x: number; y: number };
export type RouteEdge = { id: string; source: string; target: string };
export type EdgeRoute = {
    sourceSide: Side;
    targetSide: Side;
    points: Point[];
    path: string;
    labelX: number;
    labelY: number;
};

export const sideGap = 24;
export const laneSpacing = 10;
export const pairOffset = 8;
const cornerRadius = 12;
const stubLength = 24;
const clearance = 14;

const cx = (r: Rect) => r.x + r.w / 2;
const cy = (r: Rect) => r.y + r.h / 2;

export function chooseSides(src: Rect, tgt: Rect): { source: Side; target: Side } {
    if (tgt.x >= src.x + src.w + sideGap) return { source: "right", target: "left" };
    if (tgt.x + tgt.w + sideGap <= src.x) {
        // Backward: route around via the vertical side facing the other node, bottom when level.
        const above = cy(tgt) < cy(src) - Math.min(src.h, tgt.h) / 2;
        return above ? { source: "top", target: "top" } : { source: "bottom", target: "bottom" };
    }
    return cy(tgt) >= cy(src) ? { source: "bottom", target: "top" } : { source: "top", target: "bottom" };
}

// +1 for the edge whose source id sorts first, -1 for its reverse, so a reciprocal pair separates.
export function pairSign(source: string, target: string): number {
    return source < target ? 1 : -1;
}

export function reciprocalOffset(source: string, target: string, hasReverse: boolean): number {
    return hasReverse ? pairSign(source, target) * pairOffset : 0;
}

// Evenly spread n lanes around the side centre. Returns offsets along the side axis.
export function laneOffsets(n: number, sideLength: number): number[] {
    if (n <= 1) return [0];
    const spacing = Math.min(laneSpacing, Math.max(0, (sideLength - 16) / (n - 1)));
    return Array.from({ length: n }, (_, i) => (i - (n - 1) / 2) * spacing);
}

export function sideNormal(side: Side): Point {
    return side === "left" ? { x: -1, y: 0 } : side === "right" ? { x: 1, y: 0 } : side === "top" ? { x: 0, y: -1 } : { x: 0, y: 1 };
}

function sidePoint(rect: Rect, side: Side, along: number): Point {
    switch (side) {
        case "left":
            return { x: rect.x, y: cy(rect) + along };
        case "right":
            return { x: rect.x + rect.w, y: cy(rect) + along };
        case "top":
            return { x: cx(rect) + along, y: rect.y };
        default:
            return { x: cx(rect) + along, y: rect.y + rect.h };
    }
}

function sideLength(rect: Rect, side: Side) {
    return side === "left" || side === "right" ? rect.h : rect.w;
}

function contains(outer: Rect, inner: Rect) {
    return inner.x >= outer.x && inner.y >= outer.y && inner.x + inner.w <= outer.x + outer.w && inner.y + inner.h <= outer.y + outer.h;
}

function segmentHits(a: Point, b: Point, r: Rect, pad = 2): boolean {
    const minX = Math.min(a.x, b.x);
    const maxX = Math.max(a.x, b.x);
    const minY = Math.min(a.y, b.y);
    const maxY = Math.max(a.y, b.y);
    return maxX > r.x - pad && minX < r.x + r.w + pad && maxY > r.y - pad && minY < r.y + r.h + pad;
}

export function polylineBlocked(points: Point[], obstacles: Rect[]): boolean {
    for (let i = 0; i < points.length - 1; i++) {
        // The first and last segments leave/enter the endpoint nodes; they are tested like any other.
        for (const o of obstacles) if (segmentHits(points[i], points[i + 1], o)) return true;
    }
    return false;
}

// Candidate y values for a horizontal run, nearest to the preferred y first.
export function channelCandidates(obstacles: Rect[], preferred: number): number[] {
    const ys = new Set<number>();
    for (const o of obstacles) {
        ys.add(o.y - clearance);
        ys.add(o.y + o.h + clearance);
    }
    return [...ys].sort((a, b) => Math.abs(a - preferred) - Math.abs(b - preferred));
}

function dedupe(points: Point[]): Point[] {
    const out: Point[] = [];
    for (const p of points) {
        const last = out[out.length - 1];
        if (!last || Math.abs(last.x - p.x) > 0.01 || Math.abs(last.y - p.y) > 0.01) out.push(p);
    }
    // Drop collinear middle points.
    return out.filter((p, i) => {
        if (i === 0 || i === out.length - 1) return true;
        const a = out[i - 1];
        const b = out[i + 1];
        return !((a.x === p.x && p.x === b.x) || (a.y === p.y && p.y === b.y));
    });
}

export function roundedPath(points: Point[], radius = cornerRadius): string {
    if (points.length === 0) return "";
    let d = `M ${points[0].x} ${points[0].y}`;
    for (let i = 1; i < points.length - 1; i++) {
        const prev = points[i - 1];
        const p = points[i];
        const next = points[i + 1];
        const l1 = Math.hypot(p.x - prev.x, p.y - prev.y);
        const l2 = Math.hypot(next.x - p.x, next.y - p.y);
        const r = Math.min(radius, l1 / 2, l2 / 2);
        if (r < 0.5) {
            d += ` L ${p.x} ${p.y}`;
            continue;
        }
        const a = { x: p.x - ((p.x - prev.x) / l1) * r, y: p.y - ((p.y - prev.y) / l1) * r };
        const b = { x: p.x + ((next.x - p.x) / l2) * r, y: p.y + ((next.y - p.y) / l2) * r };
        d += ` L ${a.x} ${a.y} Q ${p.x} ${p.y} ${b.x} ${b.y}`;
    }
    const last = points[points.length - 1];
    if (points.length > 1) d += ` L ${last.x} ${last.y}`;
    return d;
}

export function longestSegmentMidpoint(points: Point[]): Point {
    let best = { len: -1, x: points[0]?.x ?? 0, y: points[0]?.y ?? 0 };
    for (let i = 0; i < points.length - 1; i++) {
        const len = Math.hypot(points[i + 1].x - points[i].x, points[i + 1].y - points[i].y);
        if (len > best.len) best = { len, x: (points[i].x + points[i + 1].x) / 2, y: (points[i].y + points[i + 1].y) / 2 };
    }
    return { x: best.x, y: best.y };
}

type Planned = { edge: RouteEdge; src: Rect; tgt: Rect; sides: { source: Side; target: Side }; hasReverse: boolean };

// Route every edge. Obstacles are every rect except those containing or contained by an endpoint.
export function routeEdges(edges: RouteEdge[], rects: Map<string, Rect>): Map<string, EdgeRoute> {
    const planned: Planned[] = [];
    const pairs = new Set(edges.map((e) => `${e.source}\0${e.target}`));
    for (const edge of edges) {
        const src = rects.get(edge.source);
        const tgt = rects.get(edge.target);
        if (!src || !tgt) continue;
        planned.push({ edge, src, tgt, sides: chooseSides(src, tgt), hasReverse: pairs.has(`${edge.target}\0${edge.source}`) });
    }

    // Lane slots per (node, side), ordered by where the other end lies to limit crossings.
    type Slot = { id: string; end: "source" | "target"; key: number; tie: number };
    const slots = new Map<string, Slot[]>();
    const addSlot = (node: string, side: Side, slot: Slot) => {
        const k = `${node}\0${side}`;
        const list = slots.get(k);
        if (list) list.push(slot);
        else slots.set(k, [slot]);
    };
    for (const p of planned) {
        const vertical = (s: Side) => s === "left" || s === "right";
        addSlot(p.edge.source, p.sides.source, {
            id: p.edge.id,
            end: "source",
            key: vertical(p.sides.source) ? cy(p.tgt) : cx(p.tgt),
            tie: pairSign(p.edge.source, p.edge.target),
        });
        addSlot(p.edge.target, p.sides.target, {
            id: p.edge.id,
            end: "target",
            key: vertical(p.sides.target) ? cy(p.src) : cx(p.src),
            tie: pairSign(p.edge.source, p.edge.target),
        });
    }
    const lane = new Map<string, { index: number; count: number; offset: number }>();
    for (const [k, list] of slots) {
        const [node, side] = k.split("\0") as [string, Side];
        const rect = rects.get(node)!;
        list.sort((a, b) => a.key - b.key || a.tie - b.tie || a.id.localeCompare(b.id));
        const offsets = laneOffsets(list.length, sideLength(rect, side));
        list.forEach((slot, index) => lane.set(`${slot.id}\0${slot.end}`, { index, count: list.length, offset: offsets[index] }));
    }

    const result = new Map<string, EdgeRoute>();
    for (const p of planned) {
        const { edge, src, tgt, sides } = p;
        const sl = lane.get(`${edge.id}\0source`)!;
        const tl = lane.get(`${edge.id}\0target`)!;
        const recip = reciprocalOffset(edge.source, edge.target, p.hasReverse);
        const s = sidePoint(src, sides.source, sl.offset + recip);
        const t = sidePoint(tgt, sides.target, tl.offset + recip);
        const obstacles = [...rects.entries()]
            .filter(([id, r]) => id !== edge.source && id !== edge.target && !contains(r, src) && !contains(r, tgt) && !contains(src, r) && !contains(tgt, r))
            .map(([, r]) => r);
        const shift = (sl.index - (sl.count - 1) / 2) * 6;
        let points: Point[];
        if (sides.source === "right") {
            const midX = (s.x + t.x) / 2 + shift;
            points = dedupe([s, { x: midX, y: s.y }, { x: midX, y: t.y }, t]);
            if (polylineBlocked(points, obstacles)) {
                const x1 = s.x + stubLength;
                const x2 = t.x - stubLength;
                const span = obstacles.filter((o) => o.x < t.x && o.x + o.w > s.x);
                for (const y of channelCandidates(span, (s.y + t.y) / 2)) {
                    const candidate = dedupe([s, { x: x1, y: s.y }, { x: x1, y }, { x: x2, y }, { x: x2, y: t.y }, t]);
                    if (!polylineBlocked(candidate, obstacles)) {
                        points = candidate;
                        break;
                    }
                }
            }
        } else if (sides.source === "bottom" && sides.target === "bottom") {
            const y = Math.max(src.y + src.h, tgt.y + tgt.h) + stubLength + sl.index * 8;
            points = dedupe([s, { x: s.x, y }, { x: t.x, y }, t]);
        } else if (sides.source === "top" && sides.target === "top") {
            const y = Math.min(src.y, tgt.y) - stubLength - sl.index * 8;
            points = dedupe([s, { x: s.x, y }, { x: t.x, y }, t]);
        } else {
            const midY = (s.y + t.y) / 2 + shift;
            points = dedupe([s, { x: s.x, y: midY }, { x: t.x, y: midY }, t]);
        }
        const label = longestSegmentMidpoint(points);
        result.set(edge.id, {
            sourceSide: sides.source,
            targetSide: sides.target,
            points,
            path: roundedPath(points),
            labelX: label.x,
            labelY: label.y,
        });
    }
    return result;
}
