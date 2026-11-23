import { describe, expect, it } from "vitest";
import type { LintFinding } from "./api";
import {
    checkStatus,
    checkStatusLine,
    formatRuleParams,
    containingLevelHref,
    goToLabel,
    locateRelationships,
    readGoToSelect,
    relationshipDisplay,
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
        expect(findingTarget({ rule: "r", severity: "advisory", node_id: "api", message: "" })).toEqual({ kind: "node", id: "api" });
        expect(findingTarget({ rule: "r", severity: "advisory", relationship_id: "a-b", message: "" })).toEqual({ kind: "relationship", id: "a-b" });
        expect(findingTarget({ rule: "r", severity: "advisory", message: "" })).toBeNull();
    });
    it("routes to the containing level", () => {
        expect(containingLevelHref("platform")).toBe("/node/platform");
        expect(containingLevelHref(undefined)).toBe("/");
        expect(containingLevelHref("")).toBe("/");
    });
    it("reads go-to state defensively", () => {
        expect(readGoToSelect(null)).toBeNull();
        expect(readGoToSelect({ select: { kind: "node", id: "a", name: "A" } })).toEqual({ kind: "node", id: "a", name: "A" });
        expect(readGoToSelect({ select: { kind: "edge", name: "x" } })).toBeNull();
    });
    it("builds unique display-name labels", () => {
        const nameOf = (id: string) => ({ a: "API", b: "DB" })[id] ?? id;
        expect(relationshipDisplay({ label: "reads", from: "a", to: "b" }, "r1", nameOf)).toBe("reads");
        expect(relationshipDisplay({ label: " ", from: "a", to: "b" }, "r1", nameOf)).toBe("API \u2192 DB");
        expect(relationshipDisplay(undefined, "r1", nameOf)).toBe("r1");
        expect(goToLabel("node", "API", "max_depth")).toBe("Go to node API, max_depth");
    });
});

describe("locateRelationships", () => {
    const rel = (id: string, aggregated = false) => ({ id, from: "a", to: "b", label: "", kind: "uses", aggregated });
    const levels: Record<string, { children: { id: string; child_count?: number }[]; relationships: ReturnType<typeof rel>[] }> = {
        "": { children: [{ id: "p", child_count: 2 }], relationships: [rel("top")] },
        p: { children: [{ id: "x", child_count: 0 }, { id: "q", child_count: 1 }], relationships: [rel("agg", true), rel("mid")] },
        q: { children: [], relationships: [rel("deep"), rel("agg")] },
    };
    const fetchLevel = async (id?: string) => levels[id ?? ""] as never;
    it("finds the containing level of each wanted edge, skipping aggregated ones", async () => {
        const out = await locateRelationships(["top", "mid", "deep", "agg", "missing"], fetchLevel);
        expect(out.get("top")?.levelId).toBeNull();
        expect(out.get("mid")?.levelId).toBe("p");
        expect(out.get("deep")?.levelId).toBe("q");
        expect(out.get("agg")?.levelId).toBe("q");
        expect(out.has("missing")).toBe(false);
    });
    it("stops once everything is found", async () => {
        const seen: string[] = [];
        await locateRelationships(["top"], async (id) => (seen.push(id ?? "root"), levels[id ?? ""] as never));
        expect(seen).toEqual(["root"]);
    });
});

describe("checkStatus", () => {
    it("reads ok, ok with advisory, issues", () => {
        expect(checkStatus({ ok: true, counts: { advisory: 0, enforced: 0, total: 0 } })).toBe("OK: no issues found.");
        expect(checkStatus({ ok: true, counts: { advisory: 2, enforced: 0, total: 2 } })).toBe("OK, with advisory issues: 0 enforced issues, 2 advisory issues.");
        expect(checkStatus({ ok: false, counts: { advisory: 0, enforced: 1, total: 1 } })).toBe("Issues found: 1 enforced issue, 0 advisory issues.");
    });
});

describe("checkStatusLine / formatRuleParams", () => {
    const result = { ok: false, counts: { advisory: 0, enforced: 1, total: 1 } } as never;
    it("lets an error replace the result status", () => {
        expect(checkStatusLine({ loading: false, error: "boom", result })).toBe("Model check failed: boom");
        expect(checkStatusLine({ loading: true, error: null, result })).toBe("Checking the model.");
        expect(checkStatusLine({ loading: false, error: null, result })).toContain("Issues found");
        expect(checkStatusLine({ loading: false, error: null, result: null })).toBe("");
    });
    it("formats rule params", () => {
        expect(formatRuleParams(undefined)).toBe("");
        expect(formatRuleParams({ max_depth: 4, name: "x" })).toBe("max_depth 4, name x");
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
    const name = (id: string) => ({ design: "Design", build: "Build" })[id] ?? id;
    it("says nothing about the stage when there is none before or after", () => {
        expect(switchNotice("Freeform", { method: "freeform", stage: "" } as never)).toBe("Switched to Freeform. Your model was not changed.");
    });
    it("mentions a stage set when moving to a staged method", () => {
        expect(switchNotice("Design first", { method: "design-first", stage: "design" } as never, name)).toBe(
            "Switched to Design first. Stage set to Design. Your model was not changed.",
        );
    });
    it("mentions a reset to the initial stage", () => {
        expect(switchNotice("Design first", { method: "design-first", stage: "design", previous_stage: "gone", stage_reset: true } as never, name)).toContain("Stage set to Design.");
    });
    it("mentions a kept stage", () => {
        expect(switchNotice("Other", { method: "o", stage: "build", previous_stage: "build" } as never, name)).toContain("Stage stays Build.");
    });
    it("mentions a cleared stage", () => {
        expect(switchNotice("Freeform", { method: "freeform", stage: "", previous_stage: "build", stage_reset: true } as never, name)).toBe(
            "Switched to Freeform. Stage cleared, this method has no stages. Your model was not changed.",
        );
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
