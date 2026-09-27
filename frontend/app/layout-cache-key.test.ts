import { describe, expect, it } from "vitest";
import { layoutCacheKey, rootLayoutKey } from "./api";

describe("layout cache key", () => {
    it("uses _root for / and the node id for /node/:nodeId", () => {
        expect(rootLayoutKey).toBe("_root");
        expect(layoutCacheKey(undefined)).toBe("_root");
        expect(layoutCacheKey("orders-service")).toBe("orders-service");
    });
});
