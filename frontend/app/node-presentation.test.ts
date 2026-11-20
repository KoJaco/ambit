import { describe, expect, it } from "vitest";
import { defaultColorForType, resolveNodeColor, statusLabel, statusTone } from "./node-presentation";

describe("node presentation resolve", () => {
    it("maps known types to palette tokens", () => {
        expect(defaultColorForType("service")).toBe("blue");
        expect(defaultColorForType("SERVICE")).toBe("blue");
        expect(defaultColorForType("unknown-thing")).toBe("neutral");
    });

    it("uses stored color override", () => {
        expect(resolveNodeColor("service", "rose")).toBe("rose");
        expect(resolveNodeColor("service")).toBe("blue");
    });
});

describe("status helpers", () => {
    it("labels known statuses and passes unknown through", () => {
        expect(statusLabel("draft")).toBe("Draft");
        expect(statusLabel("specified")).toBe("Specified");
        expect(statusLabel("assigned")).toBe("Assigned");
        expect(statusLabel("in_progress")).toBe("In progress");
        expect(statusLabel("done")).toBe("Done");
        expect(statusLabel("blocked")).toBe("Blocked");
        expect(statusLabel("weird")).toBe("weird");
    });

    it("maps statuses to tones", () => {
        expect(statusTone("in_progress")).toBe("blue");
        expect(statusTone("assigned")).toBe("blue");
        expect(statusTone("done")).toBe("teal");
        expect(statusTone("blocked")).toBe("rose");
        expect(statusTone("specified")).toBe("violet");
        expect(statusTone("draft")).toBe("neutral");
        expect(statusTone("weird")).toBe("neutral");
    });
});
