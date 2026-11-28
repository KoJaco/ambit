import { describe, expect, it } from "vitest";
import { autoAcceptCanToggle, autoAcceptHint, autoAcceptStatus } from "./auto-accept";

const policy = (over: Partial<Parameters<typeof autoAcceptStatus>[0]>) => ({ auto_accept: false, stage: "build", effective: false, ...over });

describe("auto-accept copy", () => {
    it("toggles only in the build stage", () => {
        expect(autoAcceptCanToggle(policy({ stage: "build" }))).toBe(true);
        expect(autoAcceptCanToggle(policy({ stage: "design" }))).toBe(false);
        expect(autoAcceptCanToggle(policy({ stage: "" }))).toBe(false);
        expect(autoAcceptCanToggle(null)).toBe(false);
    });
    it("says a stageless method has no stages", () => {
        expect(autoAcceptHint(policy({ stage: "" }))).toContain("This method has no stages.");
        expect(autoAcceptHint(policy({ stage: "" }))).not.toContain("unknown");
        expect(autoAcceptHint(policy({ stage: "design" }))).toContain("The current stage is design.");
        expect(autoAcceptHint(policy({ stage: "build" }))).toBe("");
    });
    it("never shows the raw identifier", () => {
        const off = autoAcceptStatus(policy({ reason: "auto_accept is off" }));
        expect(off).toBe("Not active: Auto-accept is off");
        expect(autoAcceptStatus(policy({ auto_accept: true, stage: "", reason: 'the project stage is ""' }))).not.toMatch(/auto_accept|""/);
    });
    it("reports active", () => {
        expect(autoAcceptStatus(policy({ auto_accept: true, effective: true }))).toBe("Active");
    });
});
