import { describe, expect, it } from "vitest";
import type { BuildOrder, Constraint } from "./api";
import {
    addConstraint,
    buildBadge,
    constraintsChangeText,
    constraintsDiff,
    contractChangeSummary,
    contractSummary,
    counterState,
    dependsOptions,
    edgeContractDirty,
    edgeContractLine,
    moveConstraint,
    normalizeContract,
    normalizeEdgeContract,
    removeConstraint,
    saveErrorTarget,
    toggleDependency,
    toggleSeverity,
    validateConstraints,
    validateContract,
    validateEdgeContract,
} from "./node-contract";

const rule = (id: string, text: string, severity: Constraint["severity"] = "must"): Constraint => ({ id, text, severity });

describe("node contract", () => {
    it("normalizes by trimming and dropping blanks; empty becomes {} so a PATCH clears", () => {
        expect(normalizeContract({ payloads: "  ", invariants: [" a ", ""], errors: [] })).toEqual({ invariants: ["a"] });
        expect(normalizeContract(undefined)).toEqual({});
    });

    it("flags the server limits", () => {
        expect(validateContract({ payloads: "x".repeat(8001) })).toHaveLength(1);
        expect(validateContract({ invariants: Array.from({ length: 33 }, (_, i) => `i${i}`) })).toHaveLength(1);
        expect(validateContract({ errors: ["y".repeat(301)] })).toHaveLength(1);
        expect(validateContract({ payloads: "x".repeat(8000), owns_data: ["z".repeat(300)] })).toEqual([]);
    });

    it("counts near and over the limit", () => {
        expect(counterState(10, 100).near).toBe(false);
        expect(counterState(90, 100)).toMatchObject({ near: true, over: false, text: "90 / 100" });
        expect(counterState(101, 100).over).toBe(true);
    });

    it("summarizes and diffs contracts briefly", () => {
        expect(contractSummary(undefined)).toBe("none");
        expect(contractSummary({ payloads: "p", invariants: ["a", "b"], errors: ["e"] })).toBe("payloads, 2 invariants, 1 error");
        expect(contractChangeSummary({ invariants: ["a"] }, { invariants: ["a"] })).toBe("unchanged");
        expect(contractChangeSummary({ invariants: ["a"] }, {})).toBe("cleared");
        expect(contractChangeSummary(undefined, { payloads: "p", errors: ["e"] })).toBe("changed: payloads, errors 0 → 1");
    });
});

describe("edge contract", () => {
    it("normalizes and validates ref and text", () => {
        expect(normalizeEdgeContract({ format: undefined, ref: " ", text: "" })).toEqual({});
        expect(normalizeEdgeContract({ format: "openapi", ref: " api.yaml " })).toEqual({ format: "openapi", ref: "api.yaml" });
        expect(validateEdgeContract({ ref: "../secret" }).ref).toMatch(/\.\./);
        expect(validateEdgeContract({ ref: "r".repeat(301) }).ref).toBeTruthy();
        expect(validateEdgeContract({ text: "t".repeat(4001) }).text).toBeTruthy();
        expect(validateEdgeContract({ format: "text", text: "ok" })).toEqual({});
    });

    it("tracks dirtiness on the normalized form and renders one line", () => {
        expect(edgeContractDirty(undefined, { ref: " " })).toBe(false);
        expect(edgeContractDirty({ format: "text" }, { format: "openapi" })).toBe(true);
        expect(edgeContractLine({ format: "openapi", ref: "api.yaml" })).toBe("OpenAPI · api.yaml");
        expect(edgeContractLine(undefined)).toBe("");
    });
});

describe("project rules editing", () => {
    it("adds with a unique id, up to 24", () => {
        let list: Constraint[] = [rule("rule-2", "a")];
        list = addConstraint(list);
        expect(list.map((c) => c.id)).toEqual(["rule-2", "rule-3"]);
        const full = Array.from({ length: 24 }, (_, i) => rule(`r${i}`, "x"));
        expect(addConstraint(full)).toBe(full);
    });

    it("moves, removes and toggles without mutating", () => {
        const list = [rule("a", "1"), rule("b", "2"), rule("c", "3")];
        expect(moveConstraint(list, 2, -1).map((c) => c.id)).toEqual(["a", "c", "b"]);
        expect(moveConstraint(list, 0, -1)).toBe(list);
        expect(moveConstraint(list, 2, 1)).toBe(list);
        expect(removeConstraint(list, 1).map((c) => c.id)).toEqual(["a", "c"]);
        expect(toggleSeverity(list, 0)[0].severity).toBe("should");
        expect(list[0].severity).toBe("must");
    });

    it("validates text per row and the list size", () => {
        expect(validateConstraints([rule("a", "ok")]).ok).toBe(true);
        const blank = validateConstraints([rule("a", "  ")]);
        expect(blank.ok).toBe(false);
        expect(blank.rows[0]).not.toBe("");
        expect(validateConstraints([rule("a", "x".repeat(301))]).ok).toBe(false);
        expect(validateConstraints(Array.from({ length: 25 }, (_, i) => rule(`r${i}`, "x"))).list).not.toBe("");
    });
});

describe("constraints diff", () => {
    it("lists added, removed and changed rules by id", () => {
        const cur = [rule("a", "keep"), rule("b", "old", "should"), rule("c", "gone")];
        const prop = [rule("a", "keep"), rule("b", "new", "must"), rule("d", "fresh")];
        expect(constraintsDiff(cur, prop)).toEqual([
            { kind: "changed", text: "MUST: new", was: "SHOULD: old" },
            { kind: "added", text: "MUST: fresh" },
            { kind: "removed", text: "MUST: gone" },
        ]);
        expect(constraintsChangeText(cur, prop)).toBe("Project rules: 1 added, 1 removed, 1 changed");
    });

    it("treats an absent list as empty and notices a pure reorder", () => {
        expect(constraintsDiff(undefined, [rule("a", "x")])).toHaveLength(1);
        expect(constraintsChangeText([rule("a", "x"), rule("b", "y")], [rule("b", "y"), rule("a", "x")])).toBe("Reorders project rules");
    });
});

const order: BuildOrder = {
    order: [
        { id: "db", name: "Database", status: "draft", done: false, wave: 1, depends_on: [], blocked_by: [] },
        { id: "api", name: "API", status: "draft", done: false, wave: 2, depends_on: ["db"], blocked_by: ["db"] },
    ],
    waves: [["db"], ["api"]],
    cycles: [],
    unordered: [],
};

describe("build order", () => {
    it("builds the badge with blockers by name", () => {
        expect(buildBadge(order, "api")).toMatchObject({ wave: 2, text: "wave 2 · blocked by Database" });
        expect(buildBadge(order, "db")?.text).toBe("wave 1");
        expect(buildBadge(order, "missing")).toBeNull();
        expect(buildBadge(null, "db")).toBeNull();
    });

    it("flags cycle members that are not ordered", () => {
        const cyc: BuildOrder = { ...order, order: [], cycles: [["x", "y"]], unordered: ["x", "y"] };
        expect(buildBadge(cyc, "x")).toMatchObject({ wave: null, inCycle: true });
    });
});

describe("dependency picker", () => {
    it("excludes self, keeps unknown current ids, sorts by name", () => {
        expect(dependsOptions(order, "api", ["ghost"]).map((o) => o.id)).toEqual(["db", "ghost"]);
        expect(dependsOptions(null, "api", undefined)).toEqual([]);
    });

    it("toggles and never adds self", () => {
        expect(toggleDependency(["a"], "b", "me")).toEqual(["a", "b"]);
        expect(toggleDependency(["a", "b"], "a", "me")).toEqual(["b"]);
        expect(toggleDependency(["a"], "me", "me")).toEqual(["a"]);
    });
});

describe("save error placement", () => {
    it("routes server messages next to their fields", () => {
        expect(saveErrorTarget("dependency cycle: a -> b -> a")).toBe("depends");
        expect(saveErrorTarget("invalid contract: payloads too long")).toBe("contract");
        expect(saveErrorTarget("request failed (500)")).toBe("general");
    });
});
