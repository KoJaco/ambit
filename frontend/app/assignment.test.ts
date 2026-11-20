import { describe, expect, it } from "vitest";
import { assignmentLabel, assignmentState } from "./assignment";

const nodes = [
    { id: "a", name: "Alpha" },
    { id: "b", name: "Beta" },
];

describe("assignmentLabel", () => {
    it("reports no assignment", () => {
        expect(assignmentLabel(null, "a", nodes)).toBe("No active assignment");
        expect(assignmentLabel({ node_id: null }, "a", nodes)).toBe("No active assignment");
    });
    it("reports this node", () => {
        expect(assignmentLabel({ node_id: "a", assigned_at: "" }, "a", nodes)).toBe("Assigned to this node");
    });
    it("names another node, falling back to its id", () => {
        expect(assignmentLabel({ node_id: "b" }, "a", nodes)).toBe("Assigned to Beta");
        expect(assignmentLabel({ node_id: "zzz" }, "a", nodes)).toBe("Assigned to zzz");
    });
    it("flags a dangling assignment", () => {
        const a = { node_id: "ghost", dangling: true };
        expect(assignmentState(a, "ghost")).toBe("dangling");
        expect(assignmentLabel(a, "a", nodes)).toBe("Assigned to ghost (node no longer exists)");
    });
});
