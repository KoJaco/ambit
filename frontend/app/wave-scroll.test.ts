import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const css = readFileSync(new URL("./app.css", import.meta.url), "utf8");
const sheet = readFileSync(new URL("./components/run-plan-sheet.tsx", import.meta.url), "utf8");

describe("wave scroll region", () => {
    it("keeps a visible scrollbar", () => {
        expect(css).toMatch(/\.wave-scroll\s*\{[^}]*scrollbar-width:\s*auto/);
        expect(css).not.toMatch(/scrollbar-width:\s*none|scrollbar\s*\{\s*display:\s*none/);
        expect(css).toContain(".wave-scroll::-webkit-scrollbar-thumb");
    });
    it("does not let columns shrink and has no key or wheel handlers", () => {
        expect(sheet).toMatch(/className="w-56 min-w-\[14rem\] shrink-0"/);
        expect(sheet).toContain("wave-scroll");
        expect(sheet).not.toMatch(/onWheel|onKeyDown/);
    });
});
