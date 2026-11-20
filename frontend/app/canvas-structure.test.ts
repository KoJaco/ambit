import { describe, expect, it } from "vitest";
import {
    containerAt,
    defaultGroupName,
    groupableSelection,
    outsideReparentTarget,
    groupButtonLabel,
    resolveGroupName,
    stageErrorMessage,
    drillsOnDoubleClick,
} from "./canvas-structure";

describe("defaultGroupName", () => {
    it("suggests Group, then numbers past collisions", () => {
        expect(defaultGroupName([])).toBe("Group");
        expect(defaultGroupName(["Api", "group"])).toBe("Group 2");
        expect(defaultGroupName(["Group", "Group 2"])).toBe("Group 3");
    });
});

describe("containerAt", () => {
    const rects = [
        { id: "outer", x: 0, y: 0, width: 400, height: 400 },
        { id: "inner", x: 50, y: 50, width: 100, height: 100 },
    ];
    it("picks the innermost rect under the point", () => {
        expect(containerAt({ x: 60, y: 60 }, rects)).toBe("inner");
        expect(containerAt({ x: 300, y: 300 }, rects)).toBe("outer");
    });
    it("honours exclusions and misses", () => {
        expect(containerAt({ x: 60, y: 60 }, rects, ["inner"])).toBe("outer");
        expect(containerAt({ x: 900, y: 900 }, rects)).toBeNull();
    });
});

describe("groupableSelection", () => {
    it("needs two or more siblings", () => {
        expect(groupableSelection([{ id: "a", selected: true }])).toEqual([]);
        expect(
            groupableSelection([
                { id: "a", selected: true },
                { id: "b", selected: true },
                { id: "c" },
            ]),
        ).toEqual(["a", "b"]);
    });
    it("rejects mixed parents", () => {
        expect(
            groupableSelection([
                { id: "a", selected: true },
                { id: "b", selected: true, parentId: "g" },
            ]),
        ).toEqual([]);
    });
});

describe("labels", () => {
    it("formats the button and name", () => {
        expect(groupButtonLabel(3)).toBe("Group 3 nodes");
        expect(resolveGroupName("  Backend ", "Group")).toBe("Backend");
        expect(resolveGroupName("   ", "Group")).toBe("Group");
    });
    it("maps errors to the server message", () => {
        expect(stageErrorMessage(new Error("node is already in that group"))).toBe("node is already in that group");
        expect(stageErrorMessage("x")).toBe("Could not stage the proposal");
    });
});

describe("outsideReparentTarget", () => {
    it("falls back to the model root at the root level", () => {
        expect(outsideReparentTarget(null, undefined)).toBe("");
        expect(outsideReparentTarget(null, "")).toBe("");
    });
    it("falls back to the level's node on a nested level", () => {
        expect(outsideReparentTarget(null, "x")).toBe("x");
    });
    it("prefers the container under the pointer", () => {
        expect(outsideReparentTarget("other", "x")).toBe("other");
    });
});

describe("drillsOnDoubleClick", () => {
    it("does not drill into an expanded group", () => {
        expect(drillsOnDoubleClick("container")).toBe(false);
    });
    it("drills from summary cards and other node types", () => {
        expect(drillsOnDoubleClick("card")).toBe(true);
        expect(drillsOnDoubleClick("schema")).toBe(true);
        expect(drillsOnDoubleClick(undefined)).toBe(true);
    });
});
