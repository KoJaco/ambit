import { describe, expect, it } from "vitest";
import { nodeSettingsSummary, nodeSettingsSummaryText } from "./node-settings";

const base = { implementation: [] as string[], scope: [] as string[], protected: false };

describe("nodeSettingsSummary", () => {
    it("reports no scope for an empty node", () => {
        expect(nodeSettingsSummary(base, "none")).toEqual([{ kind: "scope", label: "No scope set" }]);
    });

    it("counts effective globs (scope falls back to implementation)", () => {
        const items = nodeSettingsSummary({ ...base, implementation: ["a/**", " ", "b/**"] }, "none");
        expect(items).toEqual([{ kind: "scope", label: "2 globs" }]);
        expect(nodeSettingsSummary({ ...base, scope: ["x"] }, "none")[0].label).toBe("1 glob");
    });

    it("surfaces protected and assignment state first", () => {
        const items = nodeSettingsSummary({ ...base, scope: ["x"], protected: true }, "this");
        expect(nodeSettingsSummaryText(items)).toBe("Protected, Assigned, 1 glob");
        expect(nodeSettingsSummary(base, "other")[0].label).toBe("Assigned elsewhere");
        expect(nodeSettingsSummary(base, "dangling")[0].label).toBe("Assignment dangling");
    });
});
