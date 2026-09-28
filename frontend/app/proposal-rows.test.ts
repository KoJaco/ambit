import { describe, expect, it } from "vitest";
import type { OpDiff, ProposalDiff } from "./api";
import { proposalRows } from "./proposal-rows";

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
