import { describe, expect, it } from "vitest";
import { layoutCacheKey, orphanedGroupSizeKeys, rootLayoutKey } from "./api";

describe("layout cache key", () => {
    it("uses _root for / and the node id for /node/:nodeId", () => {
        expect(rootLayoutKey).toBe("_root");
        expect(layoutCacheKey({})).toBe("_root");
        expect(layoutCacheKey({ nodeId: "orders-service" })).toBe("orders-service");
        expect(layoutCacheKey({ relationshipId: "calls" })).toBe("_rel_calls");
    });

    it("leaves keys unchanged when nothing is expanded", () => {
        expect(layoutCacheKey({ expanded: [] })).toBe("_root");
        expect(layoutCacheKey({ nodeId: "orders-service", expanded: [] })).toBe("orders-service");
    });

    it("appends the expanded-set hash, independent of order", () => {
        const key = layoutCacheKey({ nodeId: "orders-service", expanded: ["b", "a"] });
        expect(key).toMatch(/^orders-service~[0-9a-f]{8}$/);
        expect(layoutCacheKey({ nodeId: "orders-service", expanded: ["a", "b"] })).toBe(key);
        expect(layoutCacheKey({ nodeId: "orders-service", expanded: ["a"] })).not.toBe(key);
        expect(layoutCacheKey({ expanded: ["a"] })).toMatch(/^_root~[0-9a-f]{8}$/);
    });

    it("never hashes relationship levels", () => {
        expect(layoutCacheKey({ relationshipId: "calls", expanded: ["a"] })).toBe("_rel_calls");
    });
});

describe("orphaned group-size keys", () => {
    it("selects only hashed group-size keys", () => {
        expect(
            orphanedGroupSizeKeys([
                "ambit:group-sizes:_root",
                "ambit:group-sizes:_root~ab12",
                "ambit:group-sizes:orders~cd34",
                "ambit:view-modes:_root~ab12",
                "ambit:layout-sig:_root~ab12",
            ]),
        ).toEqual(["ambit:group-sizes:_root~ab12", "ambit:group-sizes:orders~cd34"]);
    });
});
