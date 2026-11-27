import { describe, expect, it, vi } from "vitest";
import { isGroupingContainer, nodeAncestorChain, returnNodeIdAfterDelete, visibleCrumbs } from "./relationship-breadcrumb";

const nodes: Record<string, Record<string, string | undefined>> = {
    root: { id: "root", name: "Root", type: "app" },
    zone: { id: "zone", name: "Zone", type: "container", parent_id: "root" },
    a: { id: "a", name: "A", type: "service", parent_id: "zone" },
    b: { id: "b", name: "B", type: "service", parent_id: "zone" },
    mid: { id: "mid", name: "Mid", type: "service", relationship_id: "r1" },
};
vi.mock("./api", () => ({
    getNode: async (id: string) => nodes[id],
    getRelationshipLevel: async () => ({ relationship: { id: "r1", from: "a", to: "b", label: "", kind: "" } }),
}));

describe("returnNodeIdAfterDelete", () => {
    it("prefers drill source node", () => {
        expect(
            returnNodeIdAfterDelete(
                { id: "r1", from: "orders-service", to: "payments-service", label: "", kind: "" },
                "platform",
            ),
        ).toBe("platform");
    });

    it("falls back to from endpoint", () => {
        expect(
            returnNodeIdAfterDelete(
                { id: "r1", from: "orders-service", to: "payments-service", label: "", kind: "" },
                undefined,
            ),
        ).toBe("orders-service");
    });
});

describe("visibleCrumbs", () => {
    const n = (id: string, type: string) => ({ id, name: id, type });

    it("makes every ancestor a link, grouping containers included", () => {
        const crumbs = visibleCrumbs([n("zone", "container"), n("web", "app"), n("inner", "Container"), n("leaf", "service")]);
        expect(crumbs.map((c) => c.id)).toEqual(["zone", "web", "inner", "leaf"]);
        expect(crumbs.every((c) => c.link)).toBe(true);
    });

    it("builds node hrefs and handles empty chains", () => {
        expect(visibleCrumbs([n("a", "app")])[0].href).toBe("/node/a");
        expect(visibleCrumbs([])).toEqual([]);
    });

    it("detects containers by type", () => {
        expect(isGroupingContainer({ type: " container " })).toBe(true);
        expect(isGroupingContainer({ type: "service" })).toBe(false);
    });
});

describe("nodeAncestorChain", () => {
    it("links every ancestor, including container groups", async () => {
        const crumbs = await nodeAncestorChain("a", undefined, true);
        expect(crumbs.map((c) => [c.id, c.link])).toEqual([["root", true], ["zone", true], ["a", true]]);
    });

    it("continues above a relationship interior node", async () => {
        const crumbs = await nodeAncestorChain("mid", undefined, true);
        expect(crumbs.map((c) => c.id)).toEqual(["root", "zone", "mid"]);
    });
});
