import { describe, expect, it } from "vitest";
import type { OpDiff, ProposalDiff } from "./api";
import { detailNeedsConfirm, proposalDetailBlocks, proposalDetailView, specDiffSummary } from "./proposal-detail";

function node(partial: Partial<OpDiff["proposed"]> & { name: string }): NonNullable<OpDiff["proposed"]> {
    return {
        id: partial.id ?? "n",
        name: partial.name,
        type: partial.type ?? "service",
        status: partial.status ?? "draft",
        parent_id: partial.parent_id,
        implementation: partial.implementation ?? [],
        scope: partial.scope ?? [],
        protected: partial.protected ?? false,
        markdown: partial.markdown ?? "",
        icon: partial.icon,
        color: partial.color,
    };
}

function op(partial: Partial<OpDiff> & Pick<OpDiff, "op">): OpDiff {
    return {
        index: 0,
        node_id: "n",
        status: "pending",
        stale: false,
        fields: [],
        current: null,
        proposed: null,
        relationship: null,
        current_relationship: null,
        ...partial,
    };
}

describe("proposal detail blocks", () => {
    it("describes a create with proposed markdown", () => {
        const blocks = proposalDetailBlocks(
            op({
                op: "create_node",
                node_id: "platform",
                proposed: node({ id: "platform", name: "Platform", markdown: "new spec" }),
            }),
        );
        const prose = blocks.find((block) => block.kind === "prose" && block.label === "Spec");
        expect(prose).toMatchObject({ after: "new spec" });
    });

    it("shows spec markdown before and after on update", () => {
        const blocks = proposalDetailBlocks(
            op({
                op: "update_node",
                node_id: "scratch",
                fields: ["spec"],
                current: node({ name: "Scratch", markdown: "old" }),
                proposed: node({ name: "Scratch", markdown: "new proposed" }),
            }),
        );
        const prose = blocks.find((block) => block.kind === "prose");
        expect(prose).toMatchObject({ before: "old", after: "new proposed" });
        expect(JSON.stringify(blocks)).not.toContain("spec changed");
    });

    it("mentions stale mismatch without comparing hashes", () => {
        const blocks = proposalDetailBlocks(
            op({
                op: "update_node",
                node_id: "scratch",
                stale: true,
                stale_reason: "base_hash mismatch",
                fields: ["spec"],
                current: node({ name: "Scratch", markdown: "now" }),
                proposed: node({ name: "Scratch", markdown: "then" }),
            }),
        );
        expect(blocks.some((block) => block.kind === "line" && block.text.includes("Stale"))).toBe(true);
        expect(
            detailNeedsConfirm(
                op({ op: "update_node", stale: true, stale_reason: "base_hash mismatch", status: "pending" }),
            ),
        ).toBe(true);
    });

    it("describes a relationship change", () => {
        const blocks = proposalDetailBlocks(
            op({
                op: "set_relationship",
                node_id: "billing",
                current: node({ id: "billing", name: "Billing" }),
                proposed: null,
                relationship: { from: "billing", to: "scratch", label: "charges", kind: "sync" },
            }),
        );
        expect(blocks.some((block) => block.kind === "line" && block.text.includes("billing → scratch"))).toBe(true);
    });
});

function diffOf(...operations: OpDiff[]): ProposalDiff {
    return { proposal_id: "p", created_at: "", source: "agent", operations: operations.map((o, index) => ({ ...o, index })) };
}

describe("proposal detail view", () => {
    const zone = op({ op: "create_node", node_id: "zone", proposed: node({ id: "zone", name: "Demo Notifications", type: "module" }) });
    const queue = op({
        op: "create_node",
        node_id: "q",
        proposed: node({ id: "q", name: "Queue", type: "queue", parent_id: "zone", icon: "bell" }),
    });

    it("resolves create parent from sibling snapshots", () => {
        const view = proposalDetailView(diffOf(zone, queue), queue);
        if (view.kind !== "create") throw new Error("kind");
        expect(view.node.parentName).toBe("Demo Notifications");
        expect(view.node.color).toBe("green");
        expect(view.node.icon).toBe("bell");
        expect(view.description).toContain("inside Demo Notifications");
    });

    it("resolves connection endpoints, falling back to ids", () => {
        const rel = op({
            op: "set_relationship",
            node_id: "q",
            relationship: { from: "q", to: "api", label: "enqueue", kind: "async" },
        });
        const view = proposalDetailView(diffOf(zone, queue, rel), rel);
        if (view.kind !== "connect") throw new Error("kind");
        expect(view.from.name).toBe("Queue");
        expect(view.to).toMatchObject({ name: "api", known: false });
        expect(view.previous).toBeNull();
        expect(view.description).toBe("New connection from Queue to api labelled enqueue, async.");
    });

    it("marks changed fields on an update and summarises spec", () => {
        const upd = op({
            op: "update_node",
            node_id: "a",
            fields: ["name", "parent_id", "spec"],
            current: node({ id: "a", name: "Old", markdown: "x\ny" }),
            proposed: node({ id: "a", name: "New", parent_id: "zone", markdown: "x\nz\nw" }),
        });
        const view = proposalDetailView(diffOf(zone, upd), upd);
        if (view.kind !== "update") throw new Error("kind");
        expect(view.after.changed).toEqual(["name", "parent"]);
        expect(view.changes.find((c) => c.field === "parent_id")).toMatchObject({ before: "(root)", after: "Demo Notifications" });
        expect(view.specSummary).toBe("Spec: 2 lines added, 1 removed");
    });

    it("shows removed connection with interior nodes", () => {
        const del = op({
            op: "delete_relationship",
            node_id: "a",
            relationship: { from: "a", to: "b", label: "calls", kind: "sync" },
            removes_nodes: [{ id: "mid", name: "Mid" }],
        });
        const view = proposalDetailView(diffOf(del), del);
        if (view.kind !== "remove-connection") throw new Error("kind");
        expect(view.interior.map((n) => n.name)).toEqual(["Mid"]);
        expect(view.description).toContain("Removed connection");
        expect(view.description).toContain("Also removes Mid");
    });

    it("shows a removed node", () => {
        const del = op({ op: "delete_node", node_id: "a", current: node({ id: "a", name: "Gone" }) });
        const view = proposalDetailView(diffOf(del), del);
        expect(view).toMatchObject({ kind: "remove-node", description: "Removed node Gone, at the top level." });
    });

    it("summarises spec diffs", () => {
        expect(specDiffSummary("a", "a")).toBe("Spec unchanged");
        expect(specDiffSummary("", "a")).toBe("Spec: 1 line added");
        expect(specDiffSummary("a\nb", "a")).toBe("Spec: 1 line removed");
    });
});
