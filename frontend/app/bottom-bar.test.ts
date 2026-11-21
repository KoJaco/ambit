import { describe, expect, it, vi } from "vitest";
import type { StructureRequest } from "./components/structure-panel";
import { createBarStore, escapeAction, focusAfterTransition, resolveBarMode } from "./bottom-bar";

const group: StructureRequest = { kind: "group", ids: ["a", "b"], suggestion: "Group" };
const ungroup: StructureRequest = { kind: "ungroup", id: "g", name: "G" };

describe("resolveBarMode", () => {
    it("shows the toolbar by default and for a single selection", () => {
        expect(resolveBarMode({ request: null, selectedCount: 0 })).toBe("toolbar");
        expect(resolveBarMode({ request: null, selectedCount: 1 })).toBe("toolbar");
    });
    it("swaps to the selection bar from two nodes", () => {
        expect(resolveBarMode({ request: null, selectedCount: 2 })).toBe("selection");
    });
    it("lets an open request win over selection", () => {
        expect(resolveBarMode({ request: group, selectedCount: 2 })).toBe("group");
        expect(resolveBarMode({ request: ungroup, selectedCount: 0 })).toBe("ungroup");
        expect(resolveBarMode({ request: ungroup, selectedCount: 3 })).toBe("ungroup");
    });
});

describe("escapeAction", () => {
    it("maps each mode", () => {
        expect(escapeAction("group")).toBe("cancel-request");
        expect(escapeAction("ungroup")).toBe("cancel-request");
        expect(escapeAction("selection")).toBe("clear-selection");
        expect(escapeAction("toolbar")).toBe("none");
    });
});

describe("focusAfterTransition", () => {
    it("returns to the Group button when the selection bar comes back", () => {
        expect(focusAfterTransition("group", "selection")).toBe("group-button");
        expect(focusAfterTransition("group", "toolbar")).toBe("selected-tool");
    });
    it("leaves other transitions alone", () => {
        expect(focusAfterTransition("ungroup", "toolbar")).toBe("none");
        expect(focusAfterTransition("toolbar", "selection")).toBe("none");
    });
});

describe("bar store", () => {
    it("notifies subscribers, resets to idle and unsubscribes", () => {
        const store = createBarStore();
        const listener = vi.fn();
        const off = store.subscribe(listener);
        const idle = store.get();
        store.set({ ...idle, selectedCount: 2 });
        expect(store.get().selectedCount).toBe(2);
        store.reset();
        expect(store.get()).toBe(idle);
        expect(listener).toHaveBeenCalledTimes(2);
        off();
        store.set({ ...idle, selectedCount: 5 });
        expect(listener).toHaveBeenCalledTimes(2);
    });
});
