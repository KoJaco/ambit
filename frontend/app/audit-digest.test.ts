import { describe, expect, it } from "vitest";
import { digestSummary, entryViews, opText, ruleLabel, ruleRows, shortProposalId } from "./audit-digest";
import { autoAcceptCanToggle, autoAcceptHint, autoAcceptStatus } from "./auto-accept";

describe("audit digest", () => {
    it("shortens ids and summarizes counts", () => {
        expect(shortProposalId("abcdef123456")).toBe("abcdef12");
        expect(shortProposalId("abc")).toBe("abc");
        expect(digestSummary({ proposals: 1, ops: 1 })).toBe("1 proposal accepted automatically, 1 change.");
        expect(digestSummary({ proposals: 2, ops: 5 })).toBe("2 proposals accepted automatically, 5 changes.");
    });
    it("sorts rules by count then name without mutating", () => {
        const input = [{ rule: "b", count: 1 }, { rule: "a", count: 1 }, { rule: "c", count: 3 }];
        expect(ruleRows(input).map((r) => r.rule)).toEqual(["c", "a", "b"]);
        expect(input[0].rule).toBe("b");
        expect(ruleRows(undefined)).toEqual([]);
    });
    it("builds entry views newest first", () => {
        const now = new Date("2026-03-10T12:00:00Z");
        const views = entryViews(
            [
                { ts: "2026-03-10T09:00:00Z", proposal_id: "old11111xyz", actor: "architect-agent", ops: [] },
                { ts: "2026-03-10T11:55:00Z", proposal_id: "new22222xyz", actor: "human", ops: [{ index: 0, op: "update_node", node: "api", rule: "spec-text", diff_summary: "spec edited" }] },
            ],
            now,
        );
        expect(views.map((v) => v.shortId)).toEqual(["new22222", "old11111"]);
        expect(views[0].age).toBe("5 min ago");
        expect(views[0].actor).toBe("Human");
        expect(views[0].ops[0].text).toBe("update_node api: spec edited");
        expect(entryViews(undefined)).toEqual([]);
    });
    it("formats ops and rules", () => {
        expect(opText({ op: "add_node", node: "", diff_summary: "" })).toBe("add_node");
        expect(ruleLabel("human")).toBe("human accept");
        expect(ruleLabel("spec-text")).toBe("spec-text");
    });
});

describe("auto-accept copy", () => {
    const base = { auto_accept: true, stage: "design", effective: false };
    it("only toggles in build", () => {
        expect(autoAcceptCanToggle(null)).toBe(false);
        expect(autoAcceptCanToggle(base)).toBe(false);
        expect(autoAcceptCanToggle({ ...base, stage: "build" })).toBe(true);
    });
    it("shows status with reason and a hint outside build", () => {
        expect(autoAcceptStatus({ ...base, reason: "stage is design" })).toContain("Not active: The current stage is design.");
        expect(autoAcceptStatus({ ...base, stage: "build", effective: true })).toBe("Active");
        expect(autoAcceptHint(base)).toContain("design");
        expect(autoAcceptHint({ ...base, stage: "build" })).toBe("");
    });
});
