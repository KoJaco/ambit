export type SidebarTab = "nodes" | "parts";

/** Target tab for a key press on a tab, or null when the key is not tab navigation. Wraps around. */
export function nextTab(current: SidebarTab, key: string, tabs: SidebarTab[]): SidebarTab | null {
    if (tabs.length === 0) return null;
    const i = Math.max(0, tabs.indexOf(current));
    switch (key) {
        case "ArrowRight":
        case "ArrowDown":
            return tabs[(i + 1) % tabs.length];
        case "ArrowLeft":
        case "ArrowUp":
            return tabs[(i - 1 + tabs.length) % tabs.length];
        case "Home":
            return tabs[0];
        case "End":
            return tabs[tabs.length - 1];
        default:
            return null;
    }
}

/** The "Work on parts of the model" footer action: Nodes tab, whole scope only (partial scope switches back from the Parts tab). */
export function showEnablePartsAction(tab: SidebarTab, scope: "whole" | "partial" | null): boolean {
    return tab === "nodes" && scope === "whole";
}

/**
 * Where focus goes when the scope changes. Leaving partial scope removes the Parts tab and the
 * "Switch to whole model" button, so focus that was on them (now on the page body) moves to the
 * New node button. Focus that is already somewhere real stays put.
 */
export function focusAfterScopeSwitch(wasPartial: boolean, isPartial: boolean, focusLost: boolean): "new-node" | null {
    return wasPartial && !isPartial && focusLost ? "new-node" : null;
}
