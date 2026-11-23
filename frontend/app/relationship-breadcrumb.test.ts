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

    it("omits grouping containers and keeps summary ancestors", () => {
        expect(visibleCrumbs([n("zone", "container"), n("client-pwa", "app")]).map((c) => c.id)).toEqual([
            "client-pwa",
        ]);
        expect(
            visibleCrumbs([n("zone", "container"), n("web", "app"), n("inner", "Container"), n("leaf", "service")]).map(
                (c) => c.id,
            ),
        ).toEqual(["web", "leaf"]);
    });

    it("keeps the viewed node even when it is a container", () => {
        expect(visibleCrumbs([n("web", "app"), n("zone", "container")]).map((c) => c.id)).toEqual(["web", "zone"]);
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
