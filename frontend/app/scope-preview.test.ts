import { describe, expect, it } from "vitest";
import { previewScope } from "./scope-preview";

// These cases mirror the Go scope rule in spirit and are allowed to drift until a shared
// fixture exists. See docs/planning/open-questions.md, "Scope glob matching is implemented twice".

describe("scope preview", () => {
    it("previews an empty scope as the implementation globs", () => {
        const preview = previewScope({
            implementation: ["services/orders/**", " "],
            scope: ["", "  "],
            protected: false,
        });
        expect(preview.globs).toEqual(["services/orders/**"]);
        expect(preview.defaultsToImplementation).toBe(true);
        expect(preview.protected).toBe(false);
    });

    it("previews an explicit scope as itself", () => {
        const preview = previewScope({
            implementation: ["services/orders/**"],
            scope: ["services/orders/api/**"],
            protected: false,
        });
        expect(preview.globs).toEqual(["services/orders/api/**"]);
        expect(preview.defaultsToImplementation).toBe(false);
    });

    it("reports protected separately from scope", () => {
        const preview = previewScope({
            implementation: ["services/orders/**"],
            scope: [],
            protected: true,
        });
        expect(preview.globs).toEqual(["services/orders/**"]);
        expect(preview.protected).toBe(true);
        expect(preview.defaultsToImplementation).toBe(true);
    });
});
