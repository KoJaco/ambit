import type { BuildClass, BuildItem, BuildOrder, ExternalItem, GateState, HumanGateKind } from "./api";

export const GATE_KIND_ORDER: HumanGateKind[] = ["decision", "credential", "phone", "deploy_approval", "install"];

export const GATE_KIND_LABEL: Record<HumanGateKind, string> = {
    decision: "Decisions",
    credential: "Credentials",
    phone: "Phone",
    deploy_approval: "Deploy approvals",
    install: "Installs",
};

export const CLASS_LABEL: Record<BuildClass, string> = {
    "agent-parallel": "Agent can build",
    "needs-human": "Needs you",
    blocked: "Blocked",
};

// "Ready now: N, up to M agents in parallel". ready_now includes nodes that need you;
// max_parallel counts only the agent-parallel ones, so M can be smaller than N.
export function summaryText(order: Pick<BuildOrder, "ready_now" | "max_parallel">): string {
    const n = order.ready_now.length;
    if (n === 0) return "Ready now: 0";
    return `Ready now: ${n}, up to ${order.max_parallel} agent${order.max_parallel === 1 ? "" : "s"} in parallel`;
}

export type WaveColumn = { wave: number; items: BuildItem[]; doneCount: number };

// Groups items by wave number, ascending, keeping build order inside each wave.
export function waveColumns(order: Pick<BuildOrder, "order">): WaveColumn[] {
    const byWave = new Map<number, BuildItem[]>();
    for (const item of order.order) {
        const list = byWave.get(item.wave);
        if (list) list.push(item);
        else byWave.set(item.wave, [item]);
    }
    return [...byWave.keys()]
        .sort((a, b) => a - b)
        .map((wave) => {
            const items = byWave.get(wave)!;
            return { wave, items, doneCount: items.filter((i) => i.done).length };
        });
}

// Waves are numbered from 1 and never renumbered. When the first wave listed is later than 1,
// the earlier ones hold only done or external nodes and are not drawn: say so.
export function earlierWavesNote(columns: Pick<WaveColumn, "wave">[]): string {
    return columns.length > 0 && columns[0].wave + 1 > 1 ? "Earlier waves are complete." : "";
}

export type ReadyPill = {
    id: string;
    name: string;
    class: BuildClass | null;
    // Short class text shown on the pill (icon alongside): "Needs you" or "Agent".
    classText: string;
    // Only nodes in the parallel set carry the parallel marker.
    inParallelSet: boolean;
};

const PILL_CLASS_TEXT: Record<BuildClass, string> = { "agent-parallel": "Agent", "needs-human": "Needs you", blocked: "Blocked" };

// One pill per ready node, carrying its class (ready_now mixes nodes that need you with nodes an agent can build).
export function readyPills(order: Pick<BuildOrder, "ready_now" | "parallel_set" | "order">): ReadyPill[] {
    const byId = new Map(order.order.map((item) => [item.id, item]));
    const parallel = new Set(order.parallel_set ?? []);
    return order.ready_now.map((id) => {
        const item = byId.get(id);
        const cls = item?.class ?? null;
        return { id, name: item?.name ?? id, class: cls, classText: cls ? PILL_CLASS_TEXT[cls] : "", inParallelSet: parallel.has(id) };
    });
}

export type GateRow = {
    key: string;
    nodeId: string;
    nodeName: string;
    index: number;
    kind: HumanGateKind;
    note?: string;
    cleared: boolean;
    external: boolean;
};

export type GateGroup = { kind: HumanGateKind; label: string; rows: GateRow[]; remaining: number };

function rowsOf(id: string, name: string, gates: GateState[] | undefined, external: boolean): GateRow[] {
    return (gates ?? []).map((g) => ({
        key: `${id}#${g.index}`,
        nodeId: id,
        nodeName: name,
        index: g.index,
        kind: g.kind,
        note: g.note,
        cleared: g.cleared,
        external,
    }));
}

// Every human gate across buildable nodes and externals, grouped by kind in a fixed order.
// Uncleared gates sort first inside a group so the open work is on top; empty kinds are dropped.
export function groupGates(order: Pick<BuildOrder, "order" | "externals">): GateGroup[] {
    const rows: GateRow[] = [
        ...order.order.flatMap((i) => rowsOf(i.id, i.name, i.human_gates, false)),
        ...order.externals.flatMap((e) => rowsOf(e.id, e.name, e.human_gates, true)),
    ];
    const groups: GateGroup[] = [];
    for (const kind of GATE_KIND_ORDER) {
        const mine = rows.filter((r) => r.kind === kind);
        if (mine.length === 0) continue;
        const sorted = [...mine.filter((r) => !r.cleared), ...mine.filter((r) => r.cleared)];
        groups.push({ kind, label: GATE_KIND_LABEL[kind], rows: sorted, remaining: mine.filter((r) => !r.cleared).length });
    }
    return groups;
}

// Returns a new order with one gate flipped, for optimistic updates and rollback.
// The class of a node is left to the server: it is re-derived on the next fetch.
export function withGateCleared(order: BuildOrder, nodeId: string, index: number, cleared: boolean): BuildOrder {
    const flip = (gates: GateState[] | undefined) => gates?.map((g) => (g.index === index ? { ...g, cleared } : g));
    return {
        ...order,
        order: order.order.map((i) => (i.id === nodeId ? { ...i, human_gates: flip(i.human_gates) } : i)),
        externals: order.externals.map((e: ExternalItem) => (e.id === nodeId ? { ...e, human_gates: flip(e.human_gates) } : e)),
    };
}

export function gateRowLabel(row: GateRow): string {
    return row.note ? `${row.nodeName}: ${row.note}` : row.nodeName;
}

export function remainingGates(groups: GateGroup[]): number {
    return groups.reduce((n, g) => n + g.remaining, 0);
}

export function chipDescription(item: BuildItem, nameOf: (id: string) => string): string {
    const parts = [item.name, CLASS_LABEL[item.class]];
    if (item.done) parts.push("done");
    if (item.blocked_by.length) parts.push(`blocked by ${item.blocked_by.map(nameOf).join(", ")}`);
    return parts.join(", ");
}
