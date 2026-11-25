import { describe, expect, it } from "vitest";
import { isGroupingContainer, returnNodeIdAfterDelete, visibleCrumbs } from "./relationship-breadcrumb";

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

    it("keeps grouping containers in the chain as non-links", () => {
        const crumbs = visibleCrumbs([n("zone", "container"), n("web", "app"), n("inner", "Container"), n("leaf", "service")]);
        expect(crumbs.map((c) => c.id)).toEqual(["zone", "web", "inner", "leaf"]);
        expect(crumbs.map((c) => c.link)).toEqual([false, true, false, true]);
    });

    it("keeps the viewed node as a link even when it is a container", () => {
        const crumbs = visibleCrumbs([n("web", "app"), n("zone", "container")]);
        expect(crumbs.map((c) => c.id)).toEqual(["web", "zone"]);
        expect(crumbs[1].link).toBe(true);
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
