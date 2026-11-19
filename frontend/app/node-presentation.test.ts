import { describe, expect, it } from "vitest";
import { defaultColorForType, resolveNodeColor } from "./node-presentation";

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
