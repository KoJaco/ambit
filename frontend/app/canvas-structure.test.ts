import { describe, expect, it } from "vitest";
import {
    descendantIds,
    resolveDropTarget,
    straddledGroups,
    acceptsDrop,
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

describe("acceptsDrop", () => {
    it("accepts open groups and collapsed summary cards only", () => {
        expect(acceptsDrop({ type: "container" })).toBe(true);
        expect(acceptsDrop({ type: "card", data: { renderer: "group", child_count: 3, expanded: false } })).toBe(true);
        expect(acceptsDrop({ type: "card", data: { renderer: "default", child_count: 2 } })).toBe(true);
        expect(acceptsDrop({ type: "card", data: { child_count: 0 } })).toBe(false);
        expect(acceptsDrop({ type: "external" })).toBe(false);
        expect(acceptsDrop({ type: "ghost" })).toBe(false);
    });

    it("drops onto a summary card found by containerAt, preferring the smaller target", () => {
        const rects = [
            { id: "zone", x: 0, y: 0, width: 600, height: 400 },
            { id: "summary", x: 100, y: 100, width: 220, height: 140 },
        ];
        expect(containerAt({ x: 150, y: 150 }, rects, ["dragged"])).toBe("summary");
        expect(containerAt({ x: 500, y: 300 }, rects, ["dragged"])).toBe("zone");
    });
});

describe("resolveDropTarget", () => {
    // parent P holds leaf A, open group G (holding K) and summary card S; outside sits top-level leaf T and group H.
    const nodes = [
        { id: "P" },
        { id: "A", parentId: "P" },
        { id: "G", parentId: "P" },
        { id: "K", parentId: "G" },
        { id: "S", parentId: "P" },
        { id: "T" },
        { id: "H" },
    ];
    const rects = [
        { id: "P", x: 0, y: 0, width: 1000, height: 600 },
        { id: "G", x: 300, y: 50, width: 300, height: 300 },
        { id: "S", x: 700, y: 50, width: 200, height: 100 },
        { id: "H", x: 1200, y: 0, width: 300, height: 300 },
    ];

    it("lets a member drop onto a sibling group at the same level", () => {
        expect(resolveDropTarget({ nodeId: "A", nodes, rects, point: { x: 400, y: 200 } })).toBe("G");
    });

    it("lets a member drop onto a sibling summary card", () => {
        expect(resolveDropTarget({ nodeId: "A", nodes, rects, point: { x: 750, y: 100 } })).toBe("S");
    });

    it("returns null for a member still over free space in its own group", () => {
        expect(resolveDropTarget({ nodeId: "A", nodes, rects, point: { x: 100, y: 500 } })).toBeNull();
    });

    it("never targets the node itself, its descendants or its own ancestors", () => {
        expect(resolveDropTarget({ nodeId: "G", nodes, rects, point: { x: 400, y: 200 } })).toBeNull();
        expect(resolveDropTarget({ nodeId: "P", nodes, rects, point: { x: 400, y: 200 } })).toBeNull();
        expect(resolveDropTarget({ nodeId: "G", nodes, rects: [...rects, { id: "K", x: 320, y: 100, width: 100, height: 100 }], point: { x: 340, y: 120 } })).toBeNull();
    });

    it("sends a member dragged out of its group to the container under the pointer, else the level", () => {
        expect(resolveDropTarget({ nodeId: "A", nodes, rects, point: { x: 1300, y: 100 } })).toBe("H");
        expect(resolveDropTarget({ nodeId: "A", nodes, rects, point: { x: 1100, y: 500 }, levelNodeId: "lvl" })).toBe("lvl");
        expect(resolveDropTarget({ nodeId: "A", nodes, rects, point: { x: 1100, y: 500 } })).toBe("");
    });

    it("keeps a top-level node dropping into any container, including a nested one", () => {
        expect(resolveDropTarget({ nodeId: "T", nodes, rects, point: { x: 400, y: 200 } })).toBe("G");
        expect(resolveDropTarget({ nodeId: "T", nodes, rects, point: { x: 1300, y: 100 } })).toBe("H");
        expect(resolveDropTarget({ nodeId: "T", nodes, rects, point: { x: 1100, y: 500 } })).toBeNull();
    });
});

describe("descendantIds", () => {
    it("collects children and grandchildren", () => {
        const set = descendantIds("a", [{ id: "a" }, { id: "b", parentId: "a" }, { id: "c", parentId: "b" }, { id: "d" }]);
        expect([...set].sort()).toEqual(["b", "c"]);
    });
});

describe("straddledGroups", () => {
    const nodes = [{ id: "P" }, { id: "n", parentId: "P" }, { id: "G", parentId: "P" }, { id: "X" }];
    const groups = [
        { id: "P", x: 0, y: 0, width: 1000, height: 600 },
        { id: "G", x: 300, y: 50, width: 300, height: 300 },
    ];

    it("flags a node half over a group it was not dropped into", () => {
        expect(straddledGroups({ id: "n", x: 200, y: 100, width: 220, height: 96 }, groups, nodes)).toEqual(["G"]);
    });

    it("ignores clear space and the node's own ancestors", () => {
        expect(straddledGroups({ id: "n", x: 20, y: 400, width: 220, height: 96 }, groups, nodes)).toEqual([]);
    });

    it("ignores the node's own descendants", () => {
        expect(straddledGroups({ id: "G", x: 300, y: 50, width: 300, height: 300 }, groups, nodes)).toEqual([]);
    });
});
