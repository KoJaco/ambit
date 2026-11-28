import { describe, expect, it } from "vitest";
import type { BuildItem, BuildOrder } from "./api";
import { chipDescription, earlierWavesNote, groupGates, readyPills, summaryText, waveColumns, withGateCleared } from "./run-plan";

const item = (id: string, wave: number, extra: Partial<BuildItem> = {}): BuildItem => ({
    id,
    name: id.toUpperCase(),
    status: "planned",
    done: false,
    wave,
    class: "agent-parallel",
    depends_on: [],
    blocked_by: [],
    ...extra,
});

const order: BuildOrder = {
    order: [
        item("a", 0, { human_gates: [{ index: 0, kind: "decision", note: "pick db", cleared: true }, { index: 1, kind: "credential", cleared: false }] }),
        item("b", 1, { class: "blocked", blocked_by: ["a"], human_gates: [{ index: 0, kind: "decision", cleared: false }] }),
        item("c", 0, { done: true }),
    ],
    waves: [["a", "c"], ["b"]],
    cycles: [],
    unordered: [],
    externals: [{ id: "x", name: "Stripe", status: "planned", human_gates: [{ index: 0, kind: "install", cleared: false }] }],
    ready_now: ["a"],
    max_parallel: 2,
    parallel_set: ["a"],
};

describe("summaryText", () => {
    it("reports ready and parallel counts", () => expect(summaryText(order)).toBe("Ready now: 1, up to 2 agents in parallel"));
    it("handles nothing ready", () => expect(summaryText({ ready_now: [], max_parallel: 0 })).toBe("Ready now: 0"));
});

describe("waveColumns", () => {
    it("groups by wave ascending and counts done", () => {
        const cols = waveColumns(order);
        expect(cols.map((c) => [c.wave, c.items.map((i) => i.id), c.doneCount])).toEqual([
            [0, ["a", "c"], 1],
            [1, ["b"], 0],
        ]);
    });
    it("is empty for no items", () => expect(waveColumns({ order: [] })).toEqual([]));
});

describe("groupGates", () => {
    const groups = groupGates(order);
    it("orders kinds and drops empty ones", () => expect(groups.map((g) => g.kind)).toEqual(["decision", "credential", "install"]));
    it("puts uncleared first and counts remaining", () => {
        const d = groups[0];
        expect(d.rows.map((r) => r.key)).toEqual(["b#0", "a#0"]);
        expect(d.remaining).toBe(1);
    });
    it("marks externals", () => expect(groups[2].rows[0]).toMatchObject({ nodeId: "x", external: true }));
});

describe("withGateCleared", () => {
    it("flips only the targeted gate and does not mutate", () => {
        const next = withGateCleared(order, "b", 0, true);
        expect(next.order[1].human_gates![0].cleared).toBe(true);
        expect(next.order[0].human_gates![1].cleared).toBe(false);
        expect(order.order[1].human_gates![0].cleared).toBe(false);
        expect(withGateCleared(order, "x", 0, true).externals[0].human_gates![0].cleared).toBe(true);
    });
});

describe("chipDescription", () => {
    it("includes class and blockers", () => {
        expect(chipDescription(order.order[1], (id) => id.toUpperCase())).toBe("B, Blocked, blocked by A");
    });
});

describe("earlierWavesNote", () => {
    it("says earlier waves are complete when the first listed wave is after wave 1", () => {
        expect(earlierWavesNote([{ wave: 1 }, { wave: 2 }])).toBe("Earlier waves are complete.");
    });
    it("is empty when wave 1 is listed or there are no waves", () => {
        expect(earlierWavesNote([{ wave: 0 }, { wave: 1 }])).toBe("");
        expect(earlierWavesNote([])).toBe("");
    });
});

describe("readyPills", () => {
    const plan = {
        order: [item("a", 0), item("b", 0, { class: "needs-human" }), item("c", 0)],
        ready_now: ["a", "b", "c", "x"],
        parallel_set: ["a"],
    };
    it("labels each pill with its class and marks only the parallel set", () => {
        const pills = readyPills(plan);
        expect(pills.map((p) => p.classText)).toEqual(["Agent", "Needs you", "Agent", ""]);
        expect(pills.map((p) => p.inParallelSet)).toEqual([true, false, false, false]);
        expect(pills[1].class).toBe("needs-human");
        expect(pills[3].name).toBe("x");
    });
});
