import { describe, expect, it } from "vitest";
import type { Crossing } from "./api";
import { buildExternalStubs, isStubActivationKey, isStubId, stubAriaLabel, straightSideFor, stubTitle } from "./external-stubs";

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
            crossing({ drillable: true, relationship_id: undefined, label: "other" }),
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

describe("buildExternalStubs duplicates", () => {
    it("collapses crossings identical in node, direction, other end and label, keeping the first id", () => {
        const { stubs, edges } = buildExternalStubs([
            crossing({ relationship_id: "x", drillable: true }),
            crossing({ relationship_id: "x-2", drillable: true }),
            crossing({ relationship_id: "y", label: "reads" }),
            crossing({ relationship_id: "z", direction: "in" }),
        ]);
        expect(stubs.map((s) => s.relationshipId)).toEqual(["x", "y", "z"]);
        expect(edges).toHaveLength(3);
    });
});

describe("stubTitle", () => {
    it("prefers the node name, then the label, then the id", () => {
        expect(stubTitle("Billing", "calls", "z")).toBe("Billing");
        expect(stubTitle(undefined, "calls", "z")).toBe("calls");
        expect(stubTitle(" ", "", "z")).toBe("z");
    });
});

describe("stub accessibility helpers", () => {
    it("names a stub by relationship and direction", () => {
        expect(stubAriaLabel("calls", "in", "Billing")).toBe("Open relationship: calls (incoming from Billing)");
        expect(stubAriaLabel(" ", "out", "Billing")).toBe("Open relationship: unnamed (outgoing to Billing)");
    });

    it("activates on Enter and Space only", () => {
        expect(isStubActivationKey("Enter")).toBe(true);
        expect(isStubActivationKey(" ")).toBe(true);
        expect(isStubActivationKey("a")).toBe(false);
    });
});
