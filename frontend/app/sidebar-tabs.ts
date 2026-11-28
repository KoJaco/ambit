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
