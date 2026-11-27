import { describe, expect, it } from "vitest";
import type { OpDiff, ProposalDiff } from "./api";
import { detailCanAccept, detailNames, detailNeedsConfirm, proposalDetailHead, proposalDetailBlocks, proposalDetailView, specDiffSummary, unresolvedNodeIds } from "./proposal-detail";

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

describe("fetched node snapshots", () => {
    const api = node({ id: "api", name: "Public API", type: "service", parent_id: "core", color: "blue" });
    const core = node({ id: "core", name: "Core", type: "zone" });
    const queue = node({ id: "q", name: "Queue" });
    const create = op({ op: "create_node", node_id: "q", proposed: queue });
    const rel = op({
        op: "set_relationship",
        node_id: "q",
        relationship: { from: "q", to: "api", label: "enqueue", kind: "async" },
    });
    const diff = diffOf(create, rel);

    it("lists unresolved endpoints, then their parents", () => {
        expect(unresolvedNodeIds(diff, rel)).toEqual(["api"]);
        expect(unresolvedNodeIds(diff, rel, new Map([["api", api]]))).toEqual(["core"]);
        expect(unresolvedNodeIds(diff, rel, new Map([["api", api], ["core", core]]))).toEqual([]);
    });

    it("resolves endpoints from fetched snapshots", () => {
        const view = proposalDetailView(diff, rel, new Map([["api", api], ["core", core]]));
        if (view.kind !== "connect") throw new Error("kind");
        expect(view.to).toMatchObject({ name: "Public API", known: true, type: "service", parentName: "Core" });
    });

    it("prefers proposal snapshots over fetched ones", () => {
        const stale = node({ id: "q", name: "Old queue" });
        const view = proposalDetailView(diff, rel, new Map([["q", stale]]));
        if (view.kind !== "connect") throw new Error("kind");
        expect(view.from.name).toBe("Queue");
    });

    it("resolves an existing parent of an update", () => {
        const upd = op({
            op: "update_node",
            node_id: "a",
            fields: ["parent_id"],
            current: node({ id: "a", name: "A" }),
            proposed: node({ id: "a", name: "A", parent_id: "core" }),
        });
        const d = diffOf(upd);
        expect(unresolvedNodeIds(d, upd)).toEqual(["core"]);
        const view = proposalDetailView(d, upd, new Map([["core", core]]));
        if (view.kind !== "update") throw new Error("kind");
        expect(view.after.parentName).toBe("Core");
        expect(view.changes[0].after).toBe("Core");
    });
});

describe("detail head names", () => {
    const rel = { id: "r", from: "a", to: "b", label: "", kind: "" };
    const op = { index: 0, op: "set_relationship", node_id: "a", status: "pending", stale: false, fields: [], current: null, proposed: null, relationship: rel, current_relationship: null } as OpDiff;
    const names = new Map([["a", "Walkflow"], ["b", "Payments API"]]);

    it("uses names when known and ids otherwise", () => {
        expect(proposalDetailHead(op, names)[0]).toEqual({ kind: "heading", text: "Set relationship · Walkflow → Payments API" });
        expect(proposalDetailHead(op)[0]).toEqual({ kind: "heading", text: "Set relationship · a → b" });
        expect(proposalDetailBlocks(op, names).find((b) => b.kind === "line" && b.text.includes("→"))).toEqual({ kind: "line", text: "Walkflow → Payments API" });
        expect(proposalDetailHead({ ...op, op: "delete_node", relationship: null }, names)[0]).toEqual({ kind: "heading", text: "Delete Walkflow" });
    });

    it("builds a name lookup from fetched nodes", () => {
        const fetched = new Map([["x", { id: "x", name: "Ex" } as never]]);
        const d = { proposal_id: "p", created_at: "", source: "", operations: [op] } as ProposalDiff;
        expect(detailNames(d, fetched).get("x")).toBe("Ex");
    });
});

describe("node model fields", () => {
    const names = new Map([["db", "Database"]]);

    it("shows depends_on and contract before and after on an update", () => {
        const o = op({
            op: "update_node",
            node_id: "api",
            fields: ["depends_on", "contract"],
            current: { ...node({ id: "api", name: "API" }), depends_on: [] },
            proposed: { ...node({ id: "api", name: "API" }), depends_on: ["db"], contract: { payloads: "POST /x", invariants: ["i"] } },
        });
        const blocks = proposalDetailBlocks(o, names);
        expect(blocks).toContainEqual({ kind: "pair", label: "Depends on", before: "(none)", after: "Database" });
        expect(blocks).toContainEqual({
            kind: "pair",
            label: "Contract",
            before: "none",
            after: "payloads, 1 invariant (changed: payloads, invariants 0 → 1)",
        });
        expect(blocks).toContainEqual({ kind: "prose", label: "Contract payloads", before: "", after: "POST /x" });
        expect(blocks).toContainEqual({ kind: "list", label: "Invariants", before: [], after: ["i"] });
        const diff: ProposalDiff = { proposal_id: "p", created_at: "", source: "mcp", operations: [o] };
        const view = proposalDetailView(diff, o);
        expect(view.kind).toBe("update");
        if (view.kind === "update") {
            expect(view.changes.map((c) => c.label)).toEqual(["depends on", "contract"]);
            expect(view.changes[0].after).toBe("db");
        }
    });

    it("previews a create with its dependencies and contract", () => {
        const o = op({
            op: "create_node",
            node_id: "api",
            proposed: { ...node({ id: "api", name: "API" }), depends_on: ["db"], contract: { errors: ["e"] } },
        });
        const diff: ProposalDiff = { proposal_id: "p", created_at: "", source: "mcp", operations: [o] };
        const view = proposalDetailView(diff, o, new Map([["db", { ...node({ id: "db", name: "Database" }) }]]));
        expect(view.kind === "create" && view.notes).toEqual(["Depends on Database", "Contract: 1 error"]);
    });

    it("shows format and ref for a connection with a contract", () => {
        const o = op({
            op: "set_relationship",
            node_id: "a",
            relationship: { from: "a", to: "b", label: "calls", kind: "sync", contract: { format: "jsonschema", ref: "s.json" } },
        });
        const diff: ProposalDiff = { proposal_id: "p", created_at: "", source: "mcp", operations: [o] };
        const view = proposalDetailView(diff, o);
        expect(view.kind === "connect" && view.edge.contract).toBe("JSON Schema · s.json");
        expect(proposalDetailBlocks(o)).toContainEqual({ kind: "pair", label: "contract", before: "—", after: "JSON Schema · s.json" });
    });

    it("shows a cleared edge contract instead of unchanged", () => {
        const o = op({
            op: "set_relationship",
            node_id: "a",
            relationship: { from: "a", to: "b", label: "calls", kind: "sync", contract_cleared: true },
            current_relationship: { from: "a", to: "b", label: "calls", kind: "sync", contract: { format: "openapi", ref: "api.yaml" } },
        });
        expect(proposalDetailBlocks(o)).toContainEqual({ kind: "pair", label: "contract", before: "OpenAPI · api.yaml", after: "Contract cleared (was: OpenAPI · api.yaml)" });
    });

    it("treats node-now-has-dependents as stale and not confirmable", () => {
        const o = op({ op: "delete_node", node_id: "a", stale: true, stale_reason: "node now has dependents" });
        expect(proposalDetailHead(o).map((b) => (b.kind === "line" ? b.text : ""))).toContain("Stale: the node now has dependents. Reject this operation; it cannot be applied.");
        expect(detailCanAccept(o)).toBe(false);
        expect(detailNeedsConfirm(o)).toBe(false);
    });

    it("renders set_constraints as Project rules lines", () => {
        const o = op({
            op: "set_constraints",
            node_id: "",
            current_constraints: [{ id: "a", text: "old", severity: "should" }],
            proposed_constraints: [{ id: "b", text: "new", severity: "must" }],
        });
        const diff: ProposalDiff = { proposal_id: "p", created_at: "", source: "mcp", operations: [o] };
        expect(proposalDetailHead(o)[0].kind === "heading" && proposalDetailHead(o)[0]).toMatchObject({ text: "Update project rules" });
        const texts = proposalDetailBlocks(o).map((b) => ("text" in b ? b.text : ""));
        expect(texts).toContain("Added: MUST: new");
        expect(texts).toContain("Removed: SHOULD: old");
        const view = proposalDetailView(diff, o);
        expect(view.kind).toBe("rules");
        expect(unresolvedNodeIds(diff, o)).toEqual([]);
    });
});

describe("rename detail", () => {
    it("describes a rename", () => {
        const blocks = proposalDetailBlocks(op({ op: "rename_node", node_id: "a", new_id: "alpha" }));
        expect(blocks.some((b) => "text" in b && b.text === "Rename a → alpha")).toBe(true);
    });
});
