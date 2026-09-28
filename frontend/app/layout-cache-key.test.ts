import { describe, expect, it } from "vitest";
import { layoutCacheKey, rootLayoutKey } from "./api";

describe("layout cache key", () => {
    it("uses _root for / and the node id for /node/:nodeId", () => {
        expect(rootLayoutKey).toBe("_root");
        expect(layoutCacheKey({})).toBe("_root");
        expect(layoutCacheKey({ nodeId: "orders-service" })).toBe("orders-service");
        expect(layoutCacheKey({ relationshipId: "calls" })).toBe("_rel_calls");
    });
});
