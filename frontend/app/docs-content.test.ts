import { describe, expect, it } from "vitest";
import { docsSections, findSection, inlineText, mcpConfigSnippet } from "./docs-content";

describe("docs content", () => {
    it("has the expected sections with unique ids", () => {
        expect(docsSections.map((s) => s.title)).toEqual([
            "What ambit is for",
            "Core ideas",
            "Working on the canvas",
            "Connecting an agent",
            "Keyboard shortcuts",
        ]);
        expect(new Set(docsSections.map((s) => s.id)).size).toBe(docsSections.length);
        expect(findSection("canvas")?.title).toBe("Working on the canvas");
        expect(findSection("nope")).toBeUndefined();
    });

    it("ships a valid MCP config", () => {
        expect(JSON.parse(mcpConfigSnippet)).toEqual({ mcpServers: { ambit: { command: "ambit", args: ["mcp"] } } });
    });

    it("includes every CLI command", () => {
        const code = docsSections
            .flatMap((s) => s.blocks)
            .flatMap((b) => (b.type === "code" ? [b.text] : []))
            .join("\n");
        for (const cmd of ["ambit init", "ambit start", "ambit check", "ambit hook install"]) expect(code).toContain(cmd);
    });

    it("flattens inline content", () => {
        expect(inlineText(["a ", { code: "b" }, " ", { action: "open-shortcuts", text: "c" }])).toBe("a b c");
    });

    it("links to the shortcuts dialog", () => {
        const kb = findSection("keyboard")!;
        const hasAction = kb.blocks.some((b) => b.type === "p" && b.content.some((c) => typeof c !== "string" && "action" in c));
        expect(hasAction).toBe(true);
    });

    it("says groups have no Open link and describes multi-select", () => {
        const text = JSON.stringify(docsSections);
        expect(text).not.toContain("Open in its header");
        expect(text).toContain("no Open link");
        expect(text).toContain("Shift");
    });
});
