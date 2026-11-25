// Session undo/redo for edits the canvas makes directly (node edits, new nodes, edge deletes,
// node drags). It is client-only and in-memory: a reload starts with no history.
//
// Not covered, on purpose: proposals (they have their own review gate), assignments, and edits
// made outside this tab (agent, CLI, editor). An outside edit clears the stack so undo can never
// overwrite work it did not make.

import { useSyncExternalStore } from "react";

// 50 steps: the common editor default (Figma, VS Code and Photoshop sit between 50 and 100).
export const HISTORY_LIMIT = 50;

export type HistoryEntry = {
    label: string;
    undo: () => Promise<void>;
    redo: () => Promise<void>;
    // Shown after a successful undo, e.g. when the undone changes came back as a proposal.
    undoNote?: () => string;
};

// Thrown by an entry that can no longer run and should be dropped without clearing the rest.
export class StaleEntryError extends Error {}

const NOTE_LABEL_MAX = 60;

// Short form of a (possibly very long) label for status notes and aria-labels.
export function truncateLabel(label: string, max = NOTE_LABEL_MAX): string {
    const text = label.replace(/\s+/g, " ").trim();
    if (text.length <= max) return text;
    return `${text.slice(0, max - 1).trimEnd()}\u2026`;
}

// "Undid: label. extra" without doubled punctuation.
export function stepNote(dir: "undo" | "redo", label: string, extra?: string): string {
    let head = `${dir === "undo" ? "Undid" : "Redid"}: ${truncateLabel(label)}`;
    if (!extra) return head;
    if (!/[.!?\u2026]$/.test(head)) head += ".";
    return `${head} ${extra}`;
}

export type HistoryView = {
    canUndo: boolean;
    canRedo: boolean;
    undoLabel: string | null;
    redoLabel: string | null;
    busy: boolean;
};

export type History = ReturnType<typeof createHistory>;

export function createHistory(limit = HISTORY_LIMIT) {
    let past: HistoryEntry[] = [];
    let future: HistoryEntry[] = [];
    let busy = false;
    let view = compute();
    const listeners = new Set<() => void>();

    function compute(): HistoryView {
        return {
            canUndo: past.length > 0,
            canRedo: future.length > 0,
            undoLabel: past.at(-1)?.label ?? null,
            redoLabel: future.at(-1)?.label ?? null,
            busy,
        };
    }
    function emit() {
        view = compute();
        listeners.forEach((listener) => listener());
    }

    // Apply one step. A failed step leaves the entry where it was and rethrows; the caller decides
    // whether to clear (the model has usually moved under us).
    async function step(from: HistoryEntry[], to: HistoryEntry[], dir: "undo" | "redo"): Promise<string | null> {
        const entry = from.at(-1);
        if (!entry || busy) return null;
        busy = true;
        emit();
        try {
            await entry[dir]();
            from.pop();
            to.push(entry);
            if (to.length > limit) to.shift();
            return entry.label;
        } catch (err) {
            if (err instanceof StaleEntryError) from.pop();
            throw err;
        } finally {
            busy = false;
            emit();
        }
    }

    return {
        record(entry: HistoryEntry) {
            past.push(entry);
            if (past.length > limit) past.shift();
            future = [];
            emit();
        },
        peek: (dir: "undo" | "redo") => (dir === "undo" ? past : future).at(-1),
        undo: () => step(past, future, "undo"),
        redo: () => step(future, past, "redo"),
        clear() {
            if (past.length === 0 && future.length === 0) return;
            past = [];
            future = [];
            emit();
        },
        subscribe(listener: () => void) {
            listeners.add(listener);
            return () => {
                listeners.delete(listener);
            };
        },
        get: () => view,
    };
}

export const history = createHistory();

export function useHistory(): HistoryView {
    return useSyncExternalStore(history.subscribe, history.get, history.get);
}

// SSE `model-changed` also fires for our own writes. Stamp each local write so only changes that
// did not come from us clear the stack.
const OWN_WRITE_WINDOW_MS = 2500;
let lastOwnWrite = 0;

export function markOwnWrite(now = Date.now()) {
    lastOwnWrite = now;
}

export function isOutsideChange(now = Date.now()): boolean {
    return now - lastOwnWrite > OWN_WRITE_WINDOW_MS;
}
