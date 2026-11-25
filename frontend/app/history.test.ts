import { describe, expect, it, vi } from "vitest";
import { createHistory, isOutsideChange, markOwnWrite, StaleEntryError, stepNote, truncateLabel } from "./history";

function entry(label: string, log: string[] = []) {
    return {
        label,
        undo: async () => void log.push(`undo ${label}`),
        redo: async () => void log.push(`redo ${label}`),
    };
}

describe("history", () => {
    it("peek returns the entry undo or redo will step", async () => {
        const h = createHistory();
        expect(h.peek("undo")).toBeUndefined();
        h.record(entry("a"));
        h.record(entry("b"));
        expect(h.peek("undo")?.label).toBe("b");
        expect(h.peek("redo")).toBeUndefined();
        await h.undo();
        expect(h.peek("undo")?.label).toBe("a");
        expect(h.peek("redo")?.label).toBe("b");
    });

    it("undoes and redoes in order", async () => {
        const log: string[] = [];
        const h = createHistory();
        h.record(entry("a", log));
        h.record(entry("b", log));
        expect(await h.undo()).toBe("b");
        expect(await h.undo()).toBe("a");
        expect(await h.undo()).toBeNull();
        expect(await h.redo()).toBe("a");
        expect(log).toEqual(["undo b", "undo a", "redo a"]);
        expect(h.get().canRedo).toBe(true);
    });

    it("drops redo entries when a new edit is recorded", async () => {
        const h = createHistory();
        h.record(entry("a"));
        await h.undo();
        h.record(entry("b"));
        expect(h.get().canRedo).toBe(false);
    });

    it("keeps only the most recent entries up to the limit", async () => {
        const h = createHistory(3);
        for (const name of ["a", "b", "c", "d"]) h.record(entry(name));
        expect(await h.undo()).toBe("d");
        expect(await h.undo()).toBe("c");
        expect(await h.undo()).toBe("b");
        expect(await h.undo()).toBeNull();
    });

    it("leaves the stack intact when a step fails", async () => {
        const h = createHistory();
        h.record({ label: "x", undo: async () => Promise.reject(new Error("gone")), redo: async () => {} });
        await expect(h.undo()).rejects.toThrow("gone");
        expect(h.get().busy).toBe(false);
        expect(h.get().canUndo).toBe(true);
    });

    it("ignores a second step while one is running", async () => {
        const h = createHistory();
        let release = () => {};
        h.record({ label: "slow", undo: () => new Promise<void>((r) => (release = r)), redo: async () => {} });
        h.record(entry("fast"));
        const first = h.undo();
        expect(await h.undo()).toBeNull();
        release();
        await first;
    });

    it("notifies subscribers and clears", () => {
        const h = createHistory();
        const fn = vi.fn();
        h.subscribe(fn);
        h.record(entry("a"));
        h.clear();
        expect(fn).toHaveBeenCalledTimes(2);
        expect(h.get().canUndo).toBe(false);
    });
});

describe("own-write window", () => {
    it("treats changes shortly after a local write as ours", () => {
        markOwnWrite(1000);
        expect(isOutsideChange(2000)).toBe(false);
        expect(isOutsideChange(4000)).toBe(true);
    });
});

describe("truncateLabel / stepNote", () => {
    it("truncates long labels with an ellipsis and keeps short ones", () => {
        expect(truncateLabel("short")).toBe("short");
        const t = truncateLabel("x".repeat(300));
        expect(t.length).toBe(60);
        expect(t.endsWith("…")).toBe(true);
    });
    it("does not double the period or leave stray spaces", () => {
        expect(stepNote("undo", "Group nodes.", "3 changes back.")).toBe("Undid: Group nodes. 3 changes back.");
        expect(stepNote("undo", "Group nodes", "3 changes back.")).toBe("Undid: Group nodes. 3 changes back.");
        expect(stepNote("redo", "Group nodes")).toBe("Redid: Group nodes");
        expect(stepNote("undo", "a ".repeat(200), "x").includes(". .")).toBe(false);
    });
});

describe("stale entry", () => {
    it("drops only the refused redo entry and keeps the undo stack", async () => {
        const h = createHistory();
        h.record(entry("keep"));
        h.record({
            label: "restaged",
            undo: async () => {},
            redo: async () => {
                throw new StaleEntryError("Cannot redo");
            },
        });
        await h.undo();
        expect(h.get().canRedo).toBe(true);
        await expect(h.redo()).rejects.toBeInstanceOf(StaleEntryError);
        expect(h.get().canRedo).toBe(false);
        expect(h.get().canUndo).toBe(true);
    });
});
