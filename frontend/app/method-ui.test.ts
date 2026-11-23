import { describe, expect, it } from "vitest";
import type { LintFinding } from "./api";
import {
    checkStatus,
    decideStageChange,
    findingTarget,
    gatesSummary,
    groupFindings,
    nextStage,
    orderRules,
    stageChipLabel,
    stageMove,
    switchNotice,
} from "./method-ui";

const stages = [
    { id: "design", name: "Design" },
    { id: "build", name: "Build" },
    { id: "ship", name: "Ship" },
];

describe("groupFindings", () => {
    const f = (rule: string, severity: "advisory" | "enforced", node_id?: string): LintFinding => ({ rule, severity, node_id, message: `${rule} ${node_id}` });
    it("puts enforced first and sorts within groups", () => {
        const groups = groupFindings([f("b", "advisory", "n1"), f("z", "enforced", "n2"), f("a", "advisory", "n3"), f("a", "enforced", "n9"), f("a", "enforced", "n1")]);
        expect(groups.map((g) => g.severity)).toEqual(["enforced", "advisory"]);
        expect(groups[0].findings.map((x) => x.node_id)).toEqual(["n1", "n9", "n2"]);
        expect(groups[1].findings.map((x) => x.rule)).toEqual(["a", "b"]);
    });
    it("drops empty groups", () => {
        expect(groupFindings([f("a", "advisory")]).map((g) => g.severity)).toEqual(["advisory"]);
        expect(groupFindings([])).toEqual([]);
    });
    it("does not mutate input", () => {
        const input = [f("b", "advisory"), f("a", "advisory")];
        groupFindings(input);
        expect(input[0].rule).toBe("b");
    });
});

describe("findingTarget", () => {
    it("prefers node, then relationship, else null", () => {
        expect(findingTarget({ rule: "r", severity: "advisory", node_id: "api", message: "" })?.href).toBe("/node/api");
        expect(findingTarget({ rule: "r", severity: "advisory", relationship_id: "a-b", message: "" })?.href).toBe("/relationship/a-b");
        expect(findingTarget({ rule: "r", severity: "advisory", message: "" })).toBeNull();
    });
});

describe("checkStatus", () => {
    it("reads ok, ok with advisory, issues", () => {
        expect(checkStatus({ ok: true, counts: { advisory: 0, enforced: 0, total: 0 } })).toBe("OK: no issues found.");
        expect(checkStatus({ ok: true, counts: { advisory: 2, enforced: 0, total: 2 } })).toBe("OK, with advisory issues: 0 enforced issues, 2 advisory issues.");
        expect(checkStatus({ ok: false, counts: { advisory: 0, enforced: 1, total: 1 } })).toBe("Issues found: 1 enforced issue, 0 advisory issues.");
    });
});

describe("stage moves", () => {
    it("classifies direction", () => {
        expect(stageMove(stages, "design", "build")).toBe("later");
        expect(stageMove(stages, "build", "design")).toBe("earlier");
        expect(stageMove(stages, "build", "build")).toBe("same");
        expect(stageMove(stages, "build", "nope")).toBe("unknown");
        expect(stageMove(stages, null, "design")).toBe("later");
    });
    it("finds the next stage", () => {
        expect(nextStage(stages, "design")?.id).toBe("build");
        expect(nextStage(stages, "ship")).toBeNull();
        expect(nextStage(stages, null)?.id).toBe("design");
    });
    it("confirms only a later move with enforced findings, never blocks", () => {
        expect(decideStageChange("later", 0)).toEqual({ action: "apply" });
        expect(decideStageChange("later", 3)).toEqual({ action: "confirm", enforced: 3, message: "3 enforced issues. Advance anyway?" });
        expect(decideStageChange("later", 1)).toMatchObject({ message: "1 enforced issue. Advance anyway?" });
        expect(decideStageChange("earlier", 5)).toEqual({ action: "apply" });
        expect(decideStageChange("same", 5)).toEqual({ action: "apply" });
    });
    it("labels the chip", () => {
        expect(stageChipLabel({ name: "Design" })).toBe("Stage: Design");
        expect(stageChipLabel(null)).toBe("Stage: not set");
    });
});

describe("switchNotice", () => {
    it("says the model is unchanged", () => {
        expect(switchNotice("Freeform", { stage: null })).toBe("Switched to Freeform. Your model was not changed.");
    });
    it("explains a reset and a clear", () => {
        expect(switchNotice("Design first", { stage: { id: "design", name: "Design" }, stage_reset: true, previous_stage: "x" })).toBe(
            'Switched to Design first. Your model was not changed. The previous stage "x" does not exist in this method. The stage was reset to Design.',
        );
        expect(switchNotice("Freeform", { stage: null, stage_reset: true })).toContain("stage was cleared");
    });
});

describe("orderRules / gatesSummary", () => {
    it("orders enforced, advisory, off then id", () => {
        const out = orderRules([
            { id: "b", severity: "off" },
            { id: "z", severity: "advisory" },
            { id: "c", severity: "enforced" },
            { id: "a", severity: "advisory" },
        ]);
        expect(out.map((r) => r.id)).toEqual(["c", "a", "z", "b"]);
    });
    it("lists gates in stage order with names, skipping empty", () => {
        expect(gatesSummary({ build: ["done"], design: ["x", "y"], ship: [] }, stages)).toEqual([
            { stage: "Design", statuses: ["x", "y"] },
            { stage: "Build", statuses: ["done"] },
        ]);
    });
});
