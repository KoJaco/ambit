// Pure helpers for the Overlay primitive.

// Next index in a Tab cycle over `count` focusables. `current` is -1 when focus is outside the list.
export function nextFocusIndex(count: number, current: number, backwards: boolean): number {
    if (count <= 0) return -1;
    if (current < 0) return backwards ? count - 1 : 0;
    return backwards ? (current - 1 + count) % count : (current + 1) % count;
}

// A backdrop click closes only when both press and release happened on the backdrop itself,
// so dragging a text selection out of the panel does not dismiss it.
export function isBackdropClick(pressTarget: unknown, releaseTarget: unknown, backdrop: unknown): boolean {
    return pressTarget === backdrop && releaseTarget === backdrop;
}

export const focusableSelector = [
    "a[href]",
    "button:not([disabled])",
    "input:not([disabled]):not([type=hidden])",
    "select:not([disabled])",
    "textarea:not([disabled])",
    "[tabindex]:not([tabindex='-1'])",
].join(",");
