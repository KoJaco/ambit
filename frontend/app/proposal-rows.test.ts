import { describe, expect, it } from "vitest";
import type { OpDiff, ProposalDiff } from "./api";
import { hierarchySections, proposalRows, summarizeProposal, unresolvedParentIds } from "./proposal-rows";

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
    };
}

function diff(operations: OpDiff[]): ProposalDiff {
    return {
        proposal_id: "p-20260928-1043-a91f",
        created_at: "2026-09-28T10:43:00Z",
        source: "seed_model",
        summary: "fixture",
        operations,
    };
}

describe("proposal rows", () => {
    it("renders a create from the proposed node", () => {
        const rows = proposalRows(
            diff([
                {
                    index: 0,
                    op: "create_node",
                    node_id: "platform",
                    status: "pending",
                    stale: false,
                    fields: [],
                    current: null,
                    proposed: node({ id: "platform", name: "Platform", type: "boundary" }),
                    relationship: null,
                    current_relationship: null,
                },
            ]),
        );
        expect(rows).toHaveLength(1);
        expect(rows[0].summary).toBe("Create Platform");
        expect(rows[0].detail).toContain("boundary");
        expect(rows[0].stale).toBe(false);
        expect(rows[0].needsConfirm).toBe(false);
        expect(rows[0].canAccept).toBe(true);
    });

    it("lists the fields an update changes", () => {
        const prose = "a long specification that must not become the row";
        const rows = proposalRows(
            diff([
                {
                    index: 1,
                    op: "update_node",
                    node_id: "payments-service",
                    status: "pending",
                    stale: false,
                    fields: ["name", "spec"],
                    current: node({ id: "payments-service", name: "Payments", markdown: "old" }),
                    proposed: node({ id: "payments-service", name: "Payments Service", markdown: prose }),
                    relationship: null,
                    current_relationship: null,
                },
            ]),
        );
        expect(rows[0].summary).toBe("Update payments-service (name, spec)");
        expect(rows[0].detail).toContain("name: Payments → Payments Service");
        expect(rows[0].detail).toContain("spec changed");
        expect(rows[0].detail).not.toContain(prose);
    });

    it("copies a stale update flag instead of comparing hashes", () => {
        const rows = proposalRows(
            diff([
                {
                    index: 2,
                    op: "update_node",
                    node_id: "payments-service",
                    status: "pending",
                    stale: true,
                    stale_reason: "base_hash mismatch",
                    fields: ["spec"],
                    current: node({ name: "Payments", markdown: "now" }),
                    proposed: node({ name: "Payments", markdown: "then" }),
                    relationship: null,
                    current_relationship: null,
                },
            ]),
        );
        expect(rows[0].stale).toBe(true);
        expect(rows[0].staleReason).toBe("base_hash mismatch");
        expect(rows[0].needsConfirm).toBe(true);
        expect(rows[0].canAccept).toBe(true);
    });

    it("renders a relationship from the operation, not a hash", () => {
        const rows = proposalRows(
            diff([
                {
                    index: 3,
                    op: "set_relationship",
                    node_id: "orders-service",
                    status: "pending",
                    stale: false,
                    current: node({ id: "orders-service", name: "Orders" }),
                    proposed: null,
                    relationship: {
                        from: "orders-service",
                        to: "payments-service",
                        label: "requests authorisation",
                        kind: "sync",
                    },
                    current_relationship: null,
                },
            ]),
        );
        expect(rows[0].summary).toBe("orders-service → payments-service");
        expect(rows[0].detail).toBe("requests authorisation · sync");
        expect(rows[0].stale).toBe(false);
    });
});

describe("delete_relationship rows", () => {
    it("renders a Remove connection row naming the interior nodes it removes", () => {
        const rows = proposalRows(
            diff([
                {
                    index: 0,
                    op: "delete_relationship",
                    node_id: "a",
                    status: "pending",
                    stale: false,
                    current: node({ id: "a", name: "Alpha" }),
                    proposed: null,
                    relationship: { id: "r1", from: "a", to: "b", label: "calls", kind: "" },
                    current_relationship: null,
                    removes_nodes: [{ id: "s1", name: "Step one" }],
                },
            ]),
        );
        expect(rows[0].verb).toBe("Remove connection");
        expect(rows[0].title).toBe("Alpha → b");
        expect(rows[0].change).toContain("Step one");
    });
});

describe("review presentation", () => {
    const move = (id: string, parent: string): OpDiff => ({
        index: 0,
        op: "update_node",
        node_id: id,
        status: "pending",
        stale: false,
        fields: ["parent_id"],
        current: node({ id, name: id.toUpperCase(), parent_id: "walkflow-platform" }),
        proposed: node({ id, name: id.toUpperCase(), parent_id: parent || undefined }),
        relationship: null,
        current_relationship: null,
    });

    it("uses names and humanised fields", () => {
        const rows = proposalRows(diff([move("a", "")]));
        expect(rows[0].verb).toBe("Update");
        expect(rows[0].title).toBe("A");
        expect(rows[0].change).toBe("Parent: walkflow-platform → root");
    });
});

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

const create = (index: number, id: string, name: string, parent?: string) =>
    op({ index, op: "create_node", node_id: id, proposed: node({ id, name, parent_id: parent }) });
const connect = (index: number, from: string, to: string, label = "") =>
    op({ index, op: "set_relationship", relationship: { from, to, label, kind: "" } });

describe("summarizeProposal", () => {
    it("counts adds, changes and removals in plain language", () => {
        const s = summarizeProposal(
            diff([
                create(0, "z", "Zone"),
                create(1, "a", "A", "z"),
                connect(2, "z", "a"),
                op({ index: 3, op: "update_node", node_id: "x", fields: ["name"], current: node({ id: "x", name: "X" }), proposed: node({ id: "x", name: "Y" }) }),
                op({ index: 4, op: "delete_relationship", relationship: { from: "a", to: "b", label: "", kind: "" } }),
            ]),
        );
        expect(s.headline).toBe("Adds 2 nodes · 1 connection; Changes 1; Removes 1 connection");
        expect(s.source).toBe("Proposed by seed_model");
        expect(s.staleCount).toBe(0);
    });

    it("ignores settled ops, counts stale pending ones and names canvas proposals", () => {
        const s = summarizeProposal({
            ...diff([
                { ...create(0, "a", "A"), status: "accepted" },
                { ...create(1, "b", "B"), stale: true },
            ]),
            source: "group_nodes",
        });
        expect(s.headline).toBe("Adds 1 node");
        expect(s.staleCount).toBe(1);
        expect(s.source).toBe("Staged by you");
        expect(summarizeProposal(diff([])).headline).toBe("No pending changes");
    });
});

describe("hierarchySections", () => {
    it("puts a new subtree and its inner edges in a new zone, outer edges in Connections", () => {
        const sections = hierarchySections(
            diff([
                create(0, "z", "Billing"),
                create(1, "a", "API", "z"),
                create(2, "b", "DB", "z"),
                connect(3, "a", "b", "reads"),
                connect(4, "a", "ext", "calls"),
                create(5, "lone", "Cache", "root1"),
                op({ index: 6, op: "delete_node", node_id: "old", current: node({ id: "old", name: "Old", parent_id: "root1" }) }),
            ]),
        );
        expect(sections.map((s) => [s.kind, s.title, s.items.length])).toEqual([
            ["new-zone", "New zone: Billing", 4],
            ["existing-zone", "In root1", 2],
            ["connections", "Connections", 1],
        ]);
        expect(sections[0].items.map((i) => i.label)).toContain('API → DB "reads"');
        expect(sections[2].items[0].label).toBe('API → ext "calls"');
    });

    it("lists existing parents that nothing names yet", () => {
        const d = diff([create(0, "lone", "Cache", "root1"), create(1, "z", "Z", "lone")]);
        expect(unresolvedParentIds(d)).toEqual(["root1"]);
        expect(unresolvedParentIds(d, new Map([["root1", 1]]))).toEqual([]);
    });

    it("names existing parent zones from the lookup, falling back to the id", () => {
        const d = diff([create(0, "lone", "Cache", "root1")]);
        expect(hierarchySections(d, new Map([["root1", "Platform"]]))[0].title).toBe("In Platform");
        expect(hierarchySections(d)[0].title).toBe("In root1");
    });
});
