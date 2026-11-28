import { describe, expect, it } from "vitest";
import { nextTab, showEnablePartsAction } from "./sidebar-tabs";

const both = ["nodes", "parts"] as const;
describe("nextTab", () => {
    it("moves and wraps with arrows", () => {
        expect(nextTab("nodes", "ArrowRight", [...both])).toBe("parts");
        expect(nextTab("parts", "ArrowRight", [...both])).toBe("nodes");
        expect(nextTab("nodes", "ArrowLeft", [...both])).toBe("parts");
    });
    it("supports Home and End and ignores other keys", () => {
        expect(nextTab("parts", "Home", [...both])).toBe("nodes");
        expect(nextTab("nodes", "End", [...both])).toBe("parts");
        expect(nextTab("nodes", "a", [...both])).toBeNull();
    });
});

describe("showEnablePartsAction", () => {
    it("shows only on the Nodes tab in whole scope", () => {
        expect(showEnablePartsAction("nodes", "whole")).toBe(true);
        expect(showEnablePartsAction("nodes", "partial")).toBe(false);
        expect(showEnablePartsAction("parts", "partial")).toBe(false);
        expect(showEnablePartsAction("nodes", null)).toBe(false);
    });
});
