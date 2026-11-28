import { describe, expect, it } from "vitest";
import { proposalAge, proposalTimestamp } from "./proposal-meta";

const now = new Date("2026-03-10T12:00:00Z");

describe("proposalAge", () => {
    it("is relative for recent proposals", () => {
        expect(proposalAge("2026-03-10T11:59:50Z", now)).toBe("just now");
        expect(proposalAge("2026-03-10T11:55:00Z", now)).toBe("5 min ago");
        expect(proposalAge("2026-03-10T09:00:00Z", now)).toBe("3 h ago");
    });
    it("falls back to a date after a day and to empty when unknown", () => {
        expect(proposalAge("2026-03-01T12:00:00Z", now)).not.toMatch(/ago/);
        expect(proposalAge("2026-03-01T12:00:00Z", now)).not.toBe("");
        expect(proposalAge(undefined, now)).toBe("");
        expect(proposalAge("not a date", now)).toBe("");
    });
});

describe("proposalTimestamp", () => {
    it("formats valid times only", () => {
        expect(proposalTimestamp("2026-03-10T12:00:00Z")).not.toBe("");
        expect(proposalTimestamp("")).toBe("");
        expect(proposalTimestamp("nope")).toBe("");
    });
});
