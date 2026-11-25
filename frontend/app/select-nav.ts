export type SelectOption = { value: string; label: string; disabled?: boolean };

/** Next enabled index from `from` in direction `step`, clamped (no wrap). -1 when none enabled. */
export function moveActive(options: SelectOption[], from: number, step: 1 | -1): number {
    for (let i = from + step; i >= 0 && i < options.length; i += step) {
        if (!options[i].disabled) return i;
    }
    return options[from] && !options[from].disabled ? from : -1;
}

export function firstEnabled(options: SelectOption[]): number {
    return options.findIndex((o) => !o.disabled);
}

export function lastEnabled(options: SelectOption[]): number {
    for (let i = options.length - 1; i >= 0; i--) if (!options[i].disabled) return i;
    return -1;
}

/** Typeahead: next enabled option whose label starts with `query`, searching after `from` first and wrapping. */
export function typeaheadIndex(options: SelectOption[], query: string, from: number): number {
    const q = query.toLowerCase();
    if (!q) return -1;
    // Repeating one character cycles through matches of that character.
    const cycling = q.length > 1 && [...q].every((c) => c === q[0]);
    const needle = cycling ? q[0] : q;
    const start = cycling || q.length === 1 ? from + 1 : Math.max(from, 0);
    for (let n = 0; n < options.length; n++) {
        const i = (start + n) % options.length;
        if (!options[i].disabled && options[i].label.toLowerCase().startsWith(needle)) return i;
    }
    return -1;
}

export type ListKeyState = { ctrlKey?: boolean; metaKey?: boolean; altKey?: boolean; shiftKey?: boolean; searching?: boolean };
export type ListKeyAction = {
    kind: "next" | "prev" | "first" | "last" | "choose" | "close" | "tab" | "type" | "none";
    preventDefault: boolean;
    stopPropagation: boolean;
};

/**
 * What an open list does with a key. Every key stops propagation so global shortcuts (delete, zoom,
 * undo...) never fire while a Select is open; typeahead characters and handled keys also preventDefault.
 */
export function listKeyAction(key: string, state: ListKeyState = {}): ListKeyAction {
    const act = (kind: ListKeyAction["kind"], preventDefault: boolean): ListKeyAction => ({
        kind,
        preventDefault,
        stopPropagation: true,
    });
    switch (key) {
        case "ArrowDown":
            return act("next", true);
        case "ArrowUp":
            return act("prev", true);
        case "Home":
            return act("first", true);
        case "End":
            return act("last", true);
        case "Enter":
            return act("choose", true);
        case " ":
            return state.searching ? act("type", true) : act("choose", true);
        case "Escape":
            return act("close", true);
        case "Tab":
            return act("tab", true);
    }
    if (key.length === 1 && !state.ctrlKey && !state.metaKey && !state.altKey) return act("type", true);
    return act("none", false);
}
