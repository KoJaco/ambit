// Keyboard shortcut registry and matching. Pure: no DOM access, so it is unit-tested in node.
// "Mod" is Cmd on macOS and Ctrl elsewhere.

export type ShortcutId =
    | "tool-pointer"
    | "tool-grab"
    | "delete"
    | "undo"
    | "redo"
    | "zoom-in"
    | "zoom-out"
    | "fit-view"
    | "open-shortcuts"
    | "close";

export type ShortcutGroup = "Tools" | "History" | "View" | "Help";

export const shortcutGroups: ShortcutGroup[] = ["Tools", "History", "View", "Help"];

export type Binding = {
    // Compared case-insensitively against KeyboardEvent.key.
    key?: string;
    // Compared against KeyboardEvent.code (layout independent), used for Shift+1.
    code?: string;
    mod?: boolean;
    // true: Shift required. false: Shift must be up. undefined: either.
    shift?: boolean;
};

export type Shortcut = {
    id: ShortcutId;
    label: string;
    group: ShortcutGroup;
    // Display chords, first one is the primary. Parts joined with "+", e.g. "Mod+Shift+Z".
    keys: string[];
    bindings: Binding[];
};

export const shortcuts: Shortcut[] = [
    { id: "tool-pointer", label: "Pointer tool", group: "Tools", keys: ["V"], bindings: [{ key: "v", mod: false, shift: false }] },
    { id: "tool-grab", label: "Grab tool (pan)", group: "Tools", keys: ["H"], bindings: [{ key: "h", mod: false, shift: false }] },
    {
        id: "delete",
        label: "Delete selected edge or node",
        group: "Tools",
        keys: ["Del"],
        bindings: [
            { key: "delete", mod: false, shift: false },
            { key: "backspace", mod: false, shift: false },
        ],
    },
    { id: "undo", label: "Undo", group: "History", keys: ["Mod+Z"], bindings: [{ key: "z", mod: true, shift: false }] },
    {
        id: "redo",
        label: "Redo",
        group: "History",
        keys: ["Mod+Shift+Z", "Mod+Y"],
        bindings: [
            { key: "z", mod: true, shift: true },
            { key: "y", mod: true, shift: false },
        ],
    },
    {
        id: "zoom-in",
        label: "Zoom in",
        group: "View",
        keys: ["+", "="],
        bindings: [
            { key: "+", mod: false },
            { key: "=", mod: false },
        ],
    },
    { id: "zoom-out", label: "Zoom out", group: "View", keys: ["-"], bindings: [{ key: "-", mod: false, shift: false }] },
    {
        id: "fit-view",
        label: "Re-center and fit",
        group: "View",
        keys: ["0", "Shift+1"],
        bindings: [
            { key: "0", mod: false, shift: false },
            { code: "Digit1", mod: false, shift: true },
        ],
    },
    { id: "open-shortcuts", label: "Show keyboard shortcuts", group: "Help", keys: ["?"], bindings: [{ key: "?", mod: false }] },
    { id: "close", label: "Close dialog or panel", group: "Help", keys: ["Esc"], bindings: [{ key: "escape", mod: false }] },
];

export type KeyEventLike = {
    key: string;
    code?: string;
    ctrlKey: boolean;
    metaKey: boolean;
    shiftKey: boolean;
    altKey: boolean;
    isComposing?: boolean;
};

export type Platform = "mac" | "other";

export function detectPlatform(nav: { platform?: string; userAgent?: string } | undefined): Platform {
    const text = `${nav?.platform ?? ""} ${nav?.userAgent ?? ""}`;
    return /mac|iphone|ipad|ipod/i.test(text) ? "mac" : "other";
}

function bindingMatches(binding: Binding, event: KeyEventLike, platform: Platform): boolean {
    const modDown = platform === "mac" ? event.metaKey : event.ctrlKey;
    // The other modifier makes it a different (browser or OS) shortcut.
    const strayMod = platform === "mac" ? event.ctrlKey : event.metaKey;
    if (strayMod || event.altKey) return false;
    if (modDown !== (binding.mod ?? false)) return false;
    if (binding.shift !== undefined && event.shiftKey !== binding.shift) return false;
    if (binding.code !== undefined) return event.code === binding.code;
    return binding.key !== undefined && event.key.toLowerCase() === binding.key;
}

// Returns the shortcut id for a keydown, or null. Does not look at focus: callers use
// isEditableTarget and their own dialog state.
export function matchShortcut(event: KeyEventLike, platform: Platform = "other"): ShortcutId | null {
    if (event.isComposing) return null;
    for (const shortcut of shortcuts) {
        if (shortcut.bindings.some((binding) => bindingMatches(binding, event, platform))) return shortcut.id;
    }
    return null;
}

export type TargetLike = {
    tagName?: string;
    isContentEditable?: boolean;
    getAttribute?: (name: string) => string | null;
} | null;

const editableTags = new Set(["INPUT", "TEXTAREA", "SELECT"]);

export function isEditableTarget(target: TargetLike): boolean {
    if (!target) return false;
    if (target.isContentEditable) return true;
    const role = target.getAttribute?.("role");
    if (role === "listbox" || role === "combobox") return true;
    const attr = target.getAttribute?.("contenteditable");
    if (attr !== null && attr !== undefined && attr !== "false") return true;
    return editableTags.has((target.tagName ?? "").toUpperCase());
}

// What the global handler should do for an event. Esc is handled by the overlays and the bar
// themselves while a dialog is open, so everything is ignored then.
export function resolveShortcut(
    event: KeyEventLike,
    context: { target: TargetLike; dialogOpen: boolean; platform: Platform },
): ShortcutId | null {
    if (context.dialogOpen) return null;
    if (isEditableTarget(context.target)) return null;
    return matchShortcut(event, context.platform);
}

const macSymbols: Record<string, string> = { Mod: "⌘", Shift: "⇧", Alt: "⌥", Esc: "Esc" };
const otherNames: Record<string, string> = { Mod: "Ctrl" };

export function formatChord(chord: string, platform: Platform): string {
    const parts = chord.split("+");
    // "+" itself is a key: "Mod++" splits to trailing empty parts.
    const merged: string[] = [];
    for (let i = 0; i < parts.length; i += 1) {
        if (parts[i] === "" && i > 0) {
            if (merged[merged.length - 1] !== "+") merged.push("+");
        } else if (parts[i] !== "") merged.push(parts[i]);
    }
    if (chord === "+") return "+";
    if (platform === "mac") return merged.map((part) => macSymbols[part] ?? part).join("");
    return merged.map((part) => otherNames[part] ?? part).join("+");
}

export function shortcutById(id: ShortcutId): Shortcut {
    const found = shortcuts.find((shortcut) => shortcut.id === id);
    if (!found) throw new Error(`unknown shortcut ${id}`);
    return found;
}

// Primary chord for tooltips, e.g. "Ctrl+Z".
export function shortcutLabel(id: ShortcutId, platform: Platform): string {
    return formatChord(shortcutById(id).keys[0], platform);
}

// "Undo (Ctrl+Z)"
export function withShortcut(label: string, id: ShortcutId, platform: Platform): string {
    return `${label} (${shortcutLabel(id, platform)})`;
}

export function shortcutsByGroup(): { group: ShortcutGroup; items: Shortcut[] }[] {
    return shortcutGroups.map((group) => ({ group, items: shortcuts.filter((s) => s.group === group) }));
}
