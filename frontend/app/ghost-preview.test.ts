import { describe, expect, it } from "vitest";
import type { OpDiff, ProposalDiff } from "./api";
import { buildGhostPreview, ghostKey, layoutCovers, placeGhosts, type Rect } from "./ghost-preview";

function op(partial: Partial<OpDiff> & { op: string; index: number }): OpDiff {
    return {
        node_id: "",
        status: "pending",
        stale: false,
        current: null,
        proposed: null,
        relationship: null,
        current_relationship: null,
        ...partial,
    };
}

function snap(id: string, name: string, parent?: string, extra: object = {}) {
    return { id, name, type: "service", status: "draft", parent_id: parent, implementation: [], scope: [], protected: false, markdown: "", ...extra };
}

function diff(id: string, operations: OpDiff[]): ProposalDiff {
    return { proposal_id: id, created_at: "", source: "seed_model", operations };
}

const rel = (from: string, to: string, extra: object = {}) => ({ from, to, label: "calls", kind: "sync", ...extra });

describe("buildGhostPreview", () => {
    const base = { parentId: "app", nodeIds: ["a", "b"], edgeIds: ["r1", "r2"] };

    it("shows creates under the level, nested creates inside new zones, and skips other levels", () => {
        const d = diff("p", [
            op({ op: "create_node", index: 0, node_id: "zone", proposed: snap("zone", "Zone", "app") }),
            op({ op: "create_node", index: 1, node_id: "kid", proposed: snap("kid", "Kid", "zone") }),
            op({ op: "create_node", index: 2, node_id: "elsewhere", proposed: snap("elsewhere", "Else", "other") }),
            op({ op: "create_node", index: 3, node_id: "interior", proposed: snap("interior", "In", "app", { relationship_id: "r1" }) }),
        ]);
        const preview = buildGhostPreview({ ...base, diffs: [d] });
        expect(preview.ghostNodes.map((n) => n.realId)).toEqual(["zone", "kid"]);
        const [zone, kid] = preview.ghostNodes;
        expect(zone.hasChildren).toBe(true);
        expect(kid.parentGhostId).toBe(zone.id);
        expect(kid.hasChildren).toBe(false);
        expect(zone.key).toBe(ghostKey("p", 0));
    });

    it("resolves a child listed before its new parent, and uses the root level when parentId is null", () => {
        const d = diff("p", [
            op({ op: "create_node", index: 0, node_id: "kid", proposed: snap("kid", "Kid", "zone") }),
            op({ op: "create_node", index: 1, node_id: "zone", proposed: snap("zone", "Zone") }),
        ]);
        const preview = buildGhostPreview({ diffs: [d], parentId: null, nodeIds: [], edgeIds: [] });
        expect(preview.ghostNodes.map((n) => n.realId).sort()).toEqual(["kid", "zone"]);
    });

    it("ignores operations that are no longer pending or carry an error", () => {
        const d = diff("p", [
            op({ op: "create_node", index: 0, node_id: "x", status: "accepted", proposed: snap("x", "X", "app") }),
            op({ op: "delete_node", index: 1, node_id: "a", error: "boom" }),
            op({ op: "update_node", index: 2, node_id: "b", status: "rejected" }),
        ]);
        const preview = buildGhostPreview({ ...base, diffs: [d] });
        expect(preview.ghostNodes).toEqual([]);
        expect(preview.nodeMarks).toEqual({});
    });

    it("marks real nodes: delete wins over update; nodes off this level are ignored", () => {
        const d = diff("p", [
            op({ op: "update_node", index: 0, node_id: "a" }),
            op({ op: "delete_node", index: 1, node_id: "a" }),
            op({ op: "update_node", index: 2, node_id: "b" }),
            op({ op: "delete_node", index: 3, node_id: "far" }),
        ]);
        const { nodeMarks } = buildGhostPreview({ ...base, diffs: [d] });
        expect(nodeMarks.a).toEqual({ nodeId: "a", kind: "removed", key: ghostKey("p", 1) });
        expect(nodeMarks.b.kind).toBe("changed");
        expect(nodeMarks.far).toBeUndefined();
    });

    it("draws new connections between real or ghost endpoints only", () => {
        const d = diff("p", [
            op({ op: "create_node", index: 0, node_id: "new", proposed: snap("new", "New", "app") }),
            op({ op: "set_relationship", index: 1, relationship: rel("a", "new") }),
            op({ op: "set_relationship", index: 2, relationship: rel("a", "b", { label: "reads" }) }),
            op({ op: "set_relationship", index: 3, relationship: rel("a", "hidden") }),
        ]);
        const { ghostEdges, ghostNodes } = buildGhostPreview({ ...base, diffs: [d] });
        expect(ghostEdges.map((e) => [e.source, e.target, e.label])).toEqual([
            ["a", ghostNodes[0].id, "calls"],
            ["a", "b", "reads"],
        ]);
    });

    it("flags connections that carry a contract", () => {
        const d = diff("p", [
            op({ op: "set_relationship", index: 0, relationship: rel("a", "b", { contract: { format: "text", text: "x" } }) }),
            op({ op: "set_relationship", index: 1, relationship: rel("b", "a", { contract: {} }) }),
        ]);
        const { ghostEdges } = buildGhostPreview({ ...base, diffs: [d] });
        expect(ghostEdges.map((e) => e.hasContract)).toEqual([true, undefined]);
    });

    it("marks an updated connection as changed and a deleted one as removed", () => {
        const d = diff("p", [
            op({ op: "set_relationship", index: 0, relationship: rel("a", "b", { label: "new label" }), current_relationship: rel("a", "b", { id: "r1" }) }),
            op({ op: "delete_relationship", index: 1, current_relationship: rel("b", "a", { id: "r2", label: "old" }), relationship: rel("b", "a", { id: "r2" }), removes_nodes: [{ id: "b", name: "B" }] }),
            op({ op: "delete_relationship", index: 2, current_relationship: rel("a", "b", { id: "gone" }) }),
        ]);
        const preview = buildGhostPreview({ ...base, diffs: [d] });
        expect(preview.ghostEdges).toEqual([]);
        expect(preview.edgeMarks.r1).toMatchObject({ kind: "changed", label: "new label" });
        expect(preview.edgeMarks.r2).toMatchObject({ kind: "removed", label: "old" });
        expect(preview.edgeMarks.gone).toBeUndefined();
        expect(preview.nodeMarks.b.kind).toBe("removed");
    });

    it("does not mutate its input", () => {
        const d = diff("p", [op({ op: "create_node", index: 0, node_id: "n", proposed: snap("n", "N", "app") })]);
        const before = JSON.stringify(d);
        buildGhostPreview({ ...base, diffs: [d] });
        expect(JSON.stringify(d)).toBe(before);
    });
});

describe("placeGhosts", () => {
    const ghost = (id: string, parentGhostId?: string, hasChildren = false) => ({
        id, realId: id, name: id, type: "service", parentGhostId, hasChildren, key: id, proposalId: "p", index: 0,
    });
    const overlaps = (a: Rect, b: Rect) =>
        a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height;

    it("places beside the connected real node without overlapping anything", () => {
        const obstacles = {
            a: { x: 0, y: 0, width: 220, height: 96 },
            blocker: { x: 260, y: 0, width: 220, height: 96 },
        };
        const g = ghost("g");
        const edge = { id: "e", source: "a", target: "g", label: "", kind: "", key: "k", proposalId: "p", index: 0 };
        const layout = placeGhosts([g], [edge], obstacles);
        const rect = { ...layout.positions.g, ...layout.sizes.g };
        for (const o of Object.values(obstacles)) expect(overlaps(rect, o)).toBe(false);
        expect(layout.positions.g.x).toBeGreaterThanOrEqual(220);
    });

    it("is deterministic and keeps ghosts apart", () => {
        const obstacles = { a: { x: 0, y: 0, width: 220, height: 96 } };
        const gs = [ghost("g1"), ghost("g2")];
        const one = placeGhosts(gs, [], obstacles);
        expect(placeGhosts(gs, [], obstacles)).toEqual(one);
        expect(overlaps({ ...one.positions.g1, ...one.sizes.g1 }, { ...one.positions.g2, ...one.sizes.g2 })).toBe(false);
    });

    it("sizes a ghost container around its members and positions members relative to it", () => {
        const layout = placeGhosts([ghost("z", undefined, true), ghost("k1", "z"), ghost("k2", "z")], [], {});
        expect(layout.sizes.z.width).toBeGreaterThan(layout.sizes.k1.width * 2);
        expect(layout.positions.k1).toEqual({ x: 16, y: 48 });
        expect(layout.positions.k2.x).toBeGreaterThan(layout.positions.k1.x);
        expect(layout.positions.z).toEqual({ x: 0, y: 0 });
    });
});

describe("set_constraints operations", () => {
    it("are ignored by the canvas preview", () => {
        const preview = buildGhostPreview({
            diffs: [
                {
                    proposal_id: "p1",
                    created_at: "",
                    source: "mcp",
                    operations: [
                        {
                            index: 0,
                            op: "set_constraints",
                            node_id: "",
                            status: "pending",
                            stale: false,
                            current: null,
                            proposed: null,
                            relationship: null,
                            current_relationship: null,
                            proposed_constraints: [{ id: "a", text: "x", severity: "must" }],
                        },
                    ],
                },
            ],
            parentId: null,
            nodeIds: ["n"],
            edgeIds: [],
        });
        expect(preview).toEqual({ ghostNodes: [], ghostEdges: [], nodeMarks: {}, edgeMarks: {} });
    });
});

describe("layoutCovers", () => {
    it("rejects a cached layout that lacks a ghost", () => {
        const layout = { positions: { a: { x: 0, y: 0 } }, sizes: { a: { width: 1, height: 1 } } };
        expect(layoutCovers(layout, [{ id: "a" }])).toBe(true);
        expect(layoutCovers(layout, [{ id: "a" }, { id: "b" }])).toBe(false);
        expect(layoutCovers(null, [{ id: "a" }])).toBe(false);
    });
});
