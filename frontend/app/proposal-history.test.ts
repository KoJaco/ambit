import { describe, expect, it, vi } from "vitest";
import type { NodeSnapshot, OpDiff } from "./api";
import { planAccepted, runSteps, type StepApi } from "./proposal-history";

function snap(id: string, over: Partial<NodeSnapshot> = {}): NodeSnapshot {
    return { id, name: id, type: "service", status: "draft", implementation: [], scope: [], protected: false, markdown: "", ...over };
}
function op(over: Partial<OpDiff> & Pick<OpDiff, "op" | "node_id">): OpDiff {
    return { index: 0, status: "pending", stale: false, current: null, proposed: null, relationship: null, current_relationship: null, ...over };
}

// Group a and b (both under "p") into container "g".
const group: OpDiff[] = [
    op({ index: 0, op: "create_node", node_id: "g", proposed: snap("g", { type: "container", parent_id: "p" }) }),
    op({ index: 1, op: "update_node", node_id: "a", fields: ["parent_id"], current: snap("a", { parent_id: "p" }), proposed: snap("a", { parent_id: "g" }) }),
    op({ index: 2, op: "update_node", node_id: "b", fields: ["parent_id"], current: snap("b", { parent_id: "p" }), proposed: snap("b", { parent_id: "g" }) }),
];

describe("planAccepted", () => {
    it("undoes a group by moving members back, then deleting the container", () => {
        const plan = planAccepted(group)!;
        expect(plan.undo).toEqual([
            { kind: "patch", id: "b", patch: { parent_id: "p" } },
            { kind: "patch", id: "a", patch: { parent_id: "p" } },
            { kind: "delete", id: "g" },
        ]);
        expect(plan.redo.map((s) => s.kind)).toEqual(["create", "patch", "patch"]);
    });

    it("restores root as an empty parent_id (ungroup)", () => {
        const plan = planAccepted([
            op({ op: "update_node", node_id: "a", fields: ["parent_id"], current: snap("a", { parent_id: "g" }), proposed: snap("a") }),
        ])!;
        expect(plan.redo).toEqual([{ kind: "patch", id: "a", patch: { parent_id: "" } }]);
        expect(plan.undo).toEqual([{ kind: "patch", id: "a", patch: { parent_id: "g" } }]);
    });

    it("recreates a deleted container on undo (ungroup and delete container)", () => {
        const plan = planAccepted([op({ op: "delete_node", node_id: "g", current: snap("g", { type: "container" }) })])!;
        expect(plan.undo[0]).toMatchObject({ kind: "create", node: { id: "g" } });
        expect(plan.redo).toEqual([{ kind: "delete", id: "g" }]);
    });

    it("maps spec to markdown and reverses relationships", () => {
        const plan = planAccepted([
            op({ op: "update_node", node_id: "a", fields: ["spec"], current: snap("a", { markdown: "old" }), proposed: snap("a", { markdown: "new" }) }),
            op({ op: "set_relationship", node_id: "a", relationship: { id: "r1", from: "a", to: "b", label: "x", kind: "sync" } }),
        ])!;
        expect(plan.undo).toEqual([
            { kind: "delete-rel", id: "r1" },
            { kind: "patch", id: "a", patch: { markdown: "old" } },
        ]);
    });

    it("gives up on fields it cannot restore", () => {
        const unknown = op({ op: "update_node", node_id: "a", fields: ["schema"], current: snap("a"), proposed: snap("a") });
        expect(planAccepted([unknown])).toBeNull();
    });
});

describe("runSteps", () => {
    const api = (over: Partial<StepApi> = {}): StepApi => ({
        createNode: vi.fn(async (input) => ({ ...snap(input.name), name: input.name })),
        updateNode: vi.fn(async (id) => snap(id)),
        deleteNode: vi.fn(async () => {}),
        setRelationship: vi.fn(),
        deleteRelationship: vi.fn(async () => {}),
        ...over,
    } as StepApi);

    it("creates then patches a recreated node", async () => {
        const a = api();
        await runSteps([{ kind: "create", node: snap("g", { status: "specified", protected: true }) }], a);
        expect(a.createNode).toHaveBeenCalledWith(expect.objectContaining({ name: "g", parent_id: "" }));
        expect(a.updateNode).toHaveBeenCalledWith("g", expect.objectContaining({ status: "specified", protected: true }));
    });

    it("rolls back and throws when the recreated id differs", async () => {
        const a = api({ createNode: vi.fn(async () => snap("g-2")) } as Partial<StepApi>);
        await expect(runSteps([{ kind: "create", node: snap("g") }], a)).rejects.toThrow("original id");
        expect(a.deleteNode).toHaveBeenCalledWith("g-2");
    });
});

describe("node model history", () => {
    const snap = (extra: object) => ({
        id: "api", name: "API", type: "service", status: "draft", implementation: [], scope: [], protected: false, markdown: "", ...extra,
    });

    it("restores depends_on and contract on undo of an update", () => {
        const plan = planAccepted([
            {
                index: 0, op: "update_node", node_id: "api", status: "pending", stale: false, fields: ["depends_on", "contract"],
                current: snap({}), proposed: snap({ depends_on: ["db"], contract: { payloads: "p" } }),
                relationship: null, current_relationship: null,
            },
        ]);
        expect(plan?.undo).toEqual([{ kind: "patch", id: "api", patch: { depends_on: [], contract: {} } }]);
        expect(plan?.redo).toEqual([{ kind: "patch", id: "api", patch: { depends_on: ["db"], contract: { payloads: "p" } } }]);
    });

    it("clears an edge contract explicitly when undoing a set that added one", () => {
        const before = { id: "r1", from: "a", to: "b", label: "l", kind: "sync" };
        const plan = planAccepted([
            {
                index: 0, op: "set_relationship", node_id: "a", status: "pending", stale: false, current: null, proposed: null,
                relationship: { ...before, contract: { format: "text" as const, text: "t" } }, current_relationship: before,
            },
        ]);
        expect(plan?.undo).toEqual([{ kind: "set-rel", rel: { ...before, contract: {} } }]);
    });

    it("re-clears an edge contract on redo and restores it on undo", () => {
        const before = { id: "r1", from: "a", to: "b", label: "l", kind: "sync", contract: { format: "openapi" as const, ref: "x.yaml" } };
        const { contract: _c, ...bare } = before;
        const plan = planAccepted([
            op({ op: "set_relationship", node_id: "a", relationship: { ...bare, contract_cleared: true }, current_relationship: before }),
        ]);
        expect(plan?.redo).toEqual([{ kind: "set-rel", rel: { ...bare, contract: {} } }]);
        expect(plan?.undo).toEqual([{ kind: "set-rel", rel: before }]);
    });

    it("does not try to undo a set_constraints operation", () => {
        expect(
            planAccepted([
                {
                    index: 0, op: "set_constraints", node_id: "", status: "pending", stale: false, current: null, proposed: null,
                    relationship: null, current_relationship: null,
                },
            ]),
        ).toBeNull();
    });
});
