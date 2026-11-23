import { describe, expect, it } from "vitest";
import {
    detectPlatform,
    formatChord,
    isEditableTarget,
    matchShortcut,
    resolveShortcut,
    shortcutLabel,
    shortcutsByGroup,
    withShortcut,
    type KeyEventLike,
} from "./shortcuts";

function key(k: string, extra: Partial<KeyEventLike> = {}): KeyEventLike {
    return { key: k, ctrlKey: false, metaKey: false, shiftKey: false, altKey: false, ...extra };
}

describe("matchShortcut", () => {
    it("matches plain tool keys case-insensitively", () => {
        expect(matchShortcut(key("v"))).toBe("tool-pointer");
        expect(matchShortcut(key("H", { shiftKey: false }))).toBe("tool-grab");
        expect(matchShortcut(key("V", { shiftKey: true }))).toBeNull();
    });

    it("uses Ctrl as Mod on non-mac and Cmd on mac", () => {
        expect(matchShortcut(key("z", { ctrlKey: true }), "other")).toBe("undo");
        expect(matchShortcut(key("z", { metaKey: true }), "other")).toBeNull();
        expect(matchShortcut(key("z", { metaKey: true }), "mac")).toBe("undo");
        expect(matchShortcut(key("z", { ctrlKey: true }), "mac")).toBeNull();
    });

    it("matches both redo bindings", () => {
        expect(matchShortcut(key("Z", { ctrlKey: true, shiftKey: true }))).toBe("redo");
        expect(matchShortcut(key("y", { ctrlKey: true }))).toBe("redo");
    });

    it("matches zoom and fit keys", () => {
        expect(matchShortcut(key("+", { shiftKey: true }))).toBe("zoom-in");
        expect(matchShortcut(key("="))).toBe("zoom-in");
        expect(matchShortcut(key("-"))).toBe("zoom-out");
        expect(matchShortcut(key("0"))).toBe("fit-view");
        expect(matchShortcut(key("!", { code: "Digit1", shiftKey: true }))).toBe("fit-view");
        expect(matchShortcut(key("1", { code: "Digit1" }))).toBeNull();
    });

    it("matches Delete and Backspace, and ignores them in fields", () => {
        expect(matchShortcut(key("Delete"))).toBe("delete");
        expect(matchShortcut(key("Backspace"))).toBe("delete");
        expect(matchShortcut(key("Backspace", { ctrlKey: true }))).toBeNull();
        const field = { tagName: "TEXTAREA" };
        expect(resolveShortcut(key("Backspace"), { target: field, dialogOpen: false, platform: "other" })).toBeNull();
        expect(resolveShortcut(key("Delete"), { target: { tagName: "INPUT" }, dialogOpen: false, platform: "other" })).toBeNull();
    });

    it("matches ? with or without reported shift, and Escape", () => {
        expect(matchShortcut(key("?", { shiftKey: true }))).toBe("open-shortcuts");
        expect(matchShortcut(key("?"))).toBe("open-shortcuts");
        expect(matchShortcut(key("Escape"))).toBe("close");
    });

    it("ignores browser shortcuts and alt combos", () => {
        expect(matchShortcut(key("-", { ctrlKey: true }))).toBeNull();
        expect(matchShortcut(key("0", { ctrlKey: true }))).toBeNull();
        expect(matchShortcut(key("v", { ctrlKey: true }))).toBeNull();
        expect(matchShortcut(key("v", { altKey: true }))).toBeNull();
        expect(matchShortcut(key("v", { metaKey: true }))).toBeNull();
    });

    it("ignores IME composition", () => {
        expect(matchShortcut(key("v", { isComposing: true }))).toBeNull();
    });
});

describe("isEditableTarget / resolveShortcut", () => {
    it("detects form fields and contenteditable", () => {
        expect(isEditableTarget({ tagName: "INPUT" })).toBe(true);
        expect(isEditableTarget({ tagName: "textarea" })).toBe(true);
        expect(isEditableTarget({ tagName: "SELECT" })).toBe(true);
        expect(isEditableTarget({ tagName: "DIV", isContentEditable: true })).toBe(true);
        expect(isEditableTarget({ tagName: "DIV", getAttribute: () => "" })).toBe(true);
        expect(isEditableTarget({ tagName: "DIV", getAttribute: () => "false" })).toBe(false);
        expect(isEditableTarget({ tagName: "BUTTON" })).toBe(false);
        expect(isEditableTarget(null)).toBe(false);
    });

    it("resolves only when not editing and no dialog is open", () => {
        const ctx = { target: { tagName: "DIV" }, dialogOpen: false, platform: "other" as const };
        expect(resolveShortcut(key("v"), ctx)).toBe("tool-pointer");
        expect(resolveShortcut(key("v"), { ...ctx, target: { tagName: "INPUT" } })).toBeNull();
        expect(resolveShortcut(key("v"), { ...ctx, dialogOpen: true })).toBeNull();
        expect(resolveShortcut(key("Escape"), { ...ctx, dialogOpen: true })).toBeNull();
    });
});

describe("labels", () => {
    it("formats chords per platform", () => {
        expect(formatChord("Mod+Shift+Z", "other")).toBe("Ctrl+Shift+Z");
        expect(formatChord("Mod+Shift+Z", "mac")).toBe("⌘⇧Z");
        expect(formatChord("Esc", "other")).toBe("Esc");
        expect(formatChord("+", "other")).toBe("+");
        expect(formatChord("Mod++", "other")).toBe("Ctrl++");
    });

    it("builds tooltip labels", () => {
        expect(shortcutLabel("undo", "other")).toBe("Ctrl+Z");
        expect(withShortcut("Undo", "undo", "mac")).toBe("Undo (⌘Z)");
        expect(withShortcut("Pointer", "tool-pointer", "other")).toBe("Pointer (V)");
    });

    it("detects mac", () => {
        expect(detectPlatform({ platform: "MacIntel" })).toBe("mac");
        expect(detectPlatform({ platform: "Linux x86_64", userAgent: "X11" })).toBe("other");
        expect(detectPlatform(undefined)).toBe("other");
    });

    it("groups every shortcut once", () => {
        const groups = shortcutsByGroup();
        expect(groups.map((g) => g.group)).toEqual(["Tools", "History", "View", "Help"]);
        expect(groups.flatMap((g) => g.items).length).toBe(10);
    });
});
