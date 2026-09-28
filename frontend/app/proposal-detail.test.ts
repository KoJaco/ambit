import { describe, expect, it } from "vitest";
import type { OpDiff } from "./api";
import { detailNeedsConfirm, proposalDetailBlocks } from "./proposal-detail";

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
