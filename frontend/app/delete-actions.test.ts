import { describe, expect, it, vi } from "vitest";
import type { NodeDetail } from "./api";
import { checkNodeDelete, deleteNodeUndoable, nodeCreateInput, nodeDeleteMessage } from "./delete-actions";
import { createHistory } from "./history";

const node: NodeDetail = {
    id: "n1",
    name: "Cache",
    type: "service",
    status: "draft",
    parent_id: "p",
    implementation: ["src/**"],
    scope: [],
    protected: false,
    markdown: "spec",
};

describe("checkNodeDelete", () => {
    it("refuses nodes with children", () => {
        expect(checkNodeDelete("n1", 2, {}).ok).toBe(false);
    });

    it("counts relationships that touch the node, including crossings", () => {
        const level = {
            relationships: [
                { from: "n1", to: "b" },
                { from: "c", to: "n1" },
                { from: "b", to: "c" },
            ],
            crossings: [{ node_id: "n1" }, { node_id: "b" }],
        };
        expect(checkNodeDelete("n1", 0, level)).toEqual({ ok: true, relationshipCount: 3 });
    });
});

describe("nodeDeleteMessage", () => {
    it("warns when connections are lost", () => {
        expect(nodeDeleteMessage("Cache", 0)).toContain("undo");
        expect(nodeDeleteMessage("Cache", 1)).toContain("1 connection?");
        expect(nodeDeleteMessage("Cache", 2)).toContain("cannot be undone");
    });
});

describe("nodeCreateInput", () => {
    it("keeps content and drops empty presentation fields", () => {
        const input = nodeCreateInput(node);
        expect(input).toMatchObject({ name: "Cache", parent_id: "p", markdown: "spec", status: "draft" });
        expect(input).not.toHaveProperty("icon");
        expect(input).not.toHaveProperty("relationship_id");
    });
});

describe("deleteNodeUndoable", () => {
    it("records an entry that recreates the node with its original id", async () => {
        const history = createHistory();
        const deleteNode = vi.fn(async () => undefined);
        const createNode = vi.fn(async () => ({ id: "n1" }));
        await deleteNodeUndoable(node, 0, { history, deleteNode, createNode });
        expect(deleteNode).toHaveBeenCalledWith("n1");
        expect(await history.undo()).toBe("delete Cache");
        expect(createNode).toHaveBeenCalledWith(nodeCreateInput(node));
        expect(createNode).toHaveBeenCalledWith(expect.objectContaining({ id: "n1" }));
        await history.redo();
        expect(deleteNode).toHaveBeenLastCalledWith("n1");
    });

    it("clears history instead when relationships were removed", async () => {
        const history = createHistory();
        history.record({ label: "x", undo: async () => undefined, redo: async () => undefined });
        await deleteNodeUndoable(node, 2, { history, deleteNode: async () => undefined, createNode: async () => ({ id: "z" }) });
        expect(history.get().canUndo).toBe(false);
    });

    it("records nothing when the delete fails", async () => {
        const history = createHistory();
        await expect(
            deleteNodeUndoable(node, 0, {
                history,
                deleteNode: async () => {
                    throw new Error("nope");
                },
                createNode: async () => ({ id: "z" }),
            }),
        ).rejects.toThrow("nope");
        expect(history.get().canUndo).toBe(false);
    });
});
