// The selected part, shared by the sidebar and the canvas. Persisted in localStorage.
import type { Part } from "./api";

const key = "ambit:selected-part";

type State = { id: string | null; part: Part | null };
let state: State = { id: read(), part: null };
const listeners = new Set<() => void>();

function read(): string | null {
    try {
        return typeof window === "undefined" ? null : window.localStorage.getItem(key);
    } catch {
        return null;
    }
}

function emit() {
    listeners.forEach((l) => l());
}

export const partSelection = {
    subscribe(l: () => void) {
        listeners.add(l);
        return () => listeners.delete(l);
    },
    get: () => state,
    getServerSnapshot: (): State => ({ id: null, part: null }),
    /** Selects a part (or null for All) and persists the id. */
    select(part: Part | null) {
        state = { id: part?.id ?? null, part };
        try {
            if (part) window.localStorage.setItem(key, part.id);
            else window.localStorage.removeItem(key);
        } catch {
            // storage unavailable; the selection still works for this session
        }
        emit();
    },
    /**
     * Refreshes the part's data after a fetch. In whole scope no part is applied (the stored id is kept);
     * in partial scope a stored id that no longer exists is cleared.
     */
    sync(parts: Part[], partial: boolean) {
        if (!partial) {
            if (state.part !== null) {
                state = { id: state.id, part: null };
                emit();
            }
            return;
        }
        const next = state.id ? (parts.find((p) => p.id === state.id) ?? null) : null;
        if (state.id && !next) {
            partSelection.select(null);
            return;
        }
        if (JSON.stringify(next) === JSON.stringify(state.part)) return;
        state = { id: state.id, part: next };
        emit();
    },
};
