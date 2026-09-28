import { describe, expect, it } from "vitest";
import { returnNodeIdAfterDelete } from "./relationship-breadcrumb";

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
