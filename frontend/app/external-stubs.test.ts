import { describe, expect, it } from "vitest";
import type { Crossing } from "./api";
import { buildExternalStubs, isStubId, straightSideFor, stubTitle } from "./external-stubs";

const crossing = (over: Partial<Crossing>): Crossing => ({
    node_id: "a",
    direction: "out",
    label: "calls",
    kind: "sync",
    other_id: "z",
    relationship_id: "r1",
    drillable: false,
    ...over,
});

describe("buildExternalStubs", () => {
    it("builds nothing without crossings", () => {
        expect(buildExternalStubs(undefined)).toEqual({ stubs: [], edges: [] });
    });

    it("puts an incoming stub on the source end with its straight side facing away", () => {
        const { stubs, edges } = buildExternalStubs([crossing({ direction: "in", relationship_id: "r2" })]);
        expect(stubs[0].straightSide).toBe("left");
        expect(edges[0].source).toBe(stubs[0].id);
        expect(edges[0].target).toBe("a");
        expect(edges[0].rel).toMatchObject({ id: "r2", from: "z", to: "a" });
    });

    it("puts an outgoing stub on the target end with a straight right side", () => {
        const { stubs, edges } = buildExternalStubs([crossing({})]);
        expect(stubs[0].straightSide).toBe("right");
        expect(edges[0].source).toBe("a");
        expect(edges[0].target).toBe(stubs[0].id);
        expect(straightSideFor("out")).toBe("right");
    });

    it("only treats stubs as drillable with a relationship id, and ignores duplicates", () => {
        const { stubs } = buildExternalStubs([
            crossing({ drillable: true, relationship_id: undefined }),
            crossing({ drillable: true }),
            crossing({ drillable: true }),
        ]);
        expect(stubs).toHaveLength(2);
        expect(stubs[0].drillable).toBe(false);
        expect(stubs[1].drillable).toBe(true);
        expect(isStubId(stubs[1].id)).toBe(true);
        expect(isStubId("a")).toBe(false);
    });
});

describe("stubTitle", () => {
    it("prefers the node name, then the label, then the id", () => {
        expect(stubTitle("Billing", "calls", "z")).toBe("Billing");
        expect(stubTitle(undefined, "calls", "z")).toBe("calls");
        expect(stubTitle(" ", "", "z")).toBe("z");
    });
});
