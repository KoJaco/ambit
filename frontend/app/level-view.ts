// Pure view-state helpers for decision 0010. The client only decides WHICH groups to ask the
// server to open; roll-up, membership and edges are always computed server-side.

export type ViewMode = "group" | "summary";
export type ViewModes = Record<string, ViewMode>;
export type ViewDefaults = { group_max: number; level_budget: number };

export const fallbackViewDefaults: ViewDefaults = { group_max: 8, level_budget: 24 };

// Level children are depth 1, members of an expanded level child are depth 2. Ids deeper than
// this are ignored by the server, so the client never requests them.
export const maxExpandDepth = 2;

// Initial request plus at most two refinements (depth-1 defaults, then depth-2 defaults).
export const maxLevelFetches = 3;

export type LevelChild = {
    id: string;
    name?: string;
    child_count?: number;
    group_id?: string;
    expanded?: boolean;
};

export function sanitizeViewModes(value: unknown): ViewModes {
    const out: ViewModes = {};
    if (!value || typeof value !== "object") return out;
    for (const [id, mode] of Object.entries(value as Record<string, unknown>)) {
        if (mode === "group" || mode === "summary") out[id] = mode;
    }
    return out;
}

function depthOf(child: LevelChild, byId: Map<string, LevelChild>): number {
    let depth = 1;
    let current = child;
    const seen = new Set<string>();
    while (current.group_id && !seen.has(current.id)) {
        seen.add(current.id);
        const parent = byId.get(current.group_id);
        if (!parent) break;
        depth += 1;
        current = parent;
    }
    return depth;
}

// The expanded set to request next. `children` is the latest flat response (members of open
// groups follow their group). Architect-set modes always win; otherwise a node opens as a group
// when child_count <= group_max and the projected visible count stays <= level_budget.
// Result is sorted so it is order independent and usable in cache keys.
export function resolveExpanded(
    children: LevelChild[],
    viewModes: ViewModes,
    defaults: ViewDefaults = fallbackViewDefaults,
): string[] {
    const byId = new Map(children.map((child) => [child.id, child]));
    const open = new Set<string>();
    let count = children.length;

    // Drop currently open groups the architect has since collapsed, along with their members.
    const dropped = new Set<string>();
    for (const child of children) {
        if (child.group_id && dropped.has(child.group_id)) {
            dropped.add(child.id);
            count -= 1;
            continue;
        }
        if (child.expanded) {
            if (viewModes[child.id] === "summary") dropped.add(child.id);
            else open.add(child.id);
        }
    }

    for (const child of children) {
        if (dropped.has(child.id)) continue;
        const childCount = child.child_count ?? 0;
        if (childCount <= 0 || open.has(child.id)) continue;
        if (child.group_id && !open.has(child.group_id)) continue;
        if (depthOf(child, byId) > maxExpandDepth) continue;
        const mode = viewModes[child.id];
        if (mode === "summary") continue;
        if (mode === "group") {
            open.add(child.id);
            count += childCount;
            continue;
        }
        if (childCount <= defaults.group_max && count + childCount <= defaults.level_budget) {
            open.add(child.id);
            count += childCount;
        }
    }
    return [...open].sort();
}

// Whether to offer the Expand/Collapse control. Open groups can always collapse. A collapsed node
// can open only on a normal level and only within the server's max expand depth.
export function canExpandChild(child: LevelChild, children: LevelChild[], onRelationshipLevel: boolean): boolean {
    if (child.expanded) return true;
    if (onRelationshipLevel || (child.child_count ?? 0) <= 0) return false;
    const byId = new Map(children.map((item) => [item.id, item]));
    return depthOf(child, byId) <= maxExpandDepth;
}

// Signature of the level's membership (sorted `id>group_id` pairs). Cached layouts are only valid
// while it matches: the same ids moved between two open groups keep the same id set.
export function layoutSignature(children: { id: string; group_id?: string }[]): string {
    return children
        .map((child) => `${child.id}>${child.group_id ?? ""}`)
        .sort()
        .join("\n");
}

// A cached layout is trusted only when it covers exactly the nodes on screen and its stored
// membership signature is present and equal to the current one.
export function layoutUsable(
    ids: string[],
    cachedIds: string[],
    storedSig: string | null,
    currentSig: string,
): boolean {
    if (ids.length === 0 || storedSig === null || storedSig !== currentSig) return false;
    if (cachedIds.length !== ids.length) return false;
    const cached = new Set(cachedIds);
    return ids.every((id) => cached.has(id));
}

export type OverBudgetHint = {
    message: string;
    largest: { id: string; name: string; members: number }[];
};

// Non-blocking hint. Never collapses anything itself.
export function overBudgetHint(
    visibleCount: number,
    budget: number,
    groups: { id: string; name: string; members: number }[],
): OverBudgetHint | null {
    if (visibleCount <= budget || groups.length === 0) return null;
    const largest = [...groups]
        .sort((a, b) => b.members - a.members || a.name.localeCompare(b.name) || a.id.localeCompare(b.id))
        .slice(0, 3);
    const names = largest.map((group) => group.name).join(", ");
    return {
        message: `${visibleCount} nodes shown (budget ${budget}). Consider collapsing: ${names}.`,
        largest,
    };
}

// Open groups on the current response with their member counts.
export function expandedGroups(children: LevelChild[]): { id: string; name: string; members: number }[] {
    return children
        .filter((child) => child.expanded)
        .map((child) => ({
            id: child.id,
            name: child.name ?? child.id,
            members: children.filter((member) => member.group_id === child.id).length,
        }));
}

// Stable short hash (FNV-1a, 32 bit) of the sorted set. Empty set is "" so existing keys stay valid.
export function expandedKey(expanded: Iterable<string>): string {
    const sorted = [...new Set(expanded)].sort();
    if (sorted.length === 0) return "";
    let hash = 0x811c9dc5;
    for (const ch of sorted.join("\n")) {
        for (let i = 0; i < ch.length; i += 1) {
            hash ^= ch.charCodeAt(i);
            hash = Math.imul(hash, 0x01000193) >>> 0;
        }
    }
    return hash.toString(16).padStart(8, "0");
}

export function setMode(viewModes: ViewModes, id: string, mode: ViewMode): ViewModes {
    return { ...viewModes, [id]: mode };
}

// Toggle one node. Every other node that can open is frozen at what is on screen right now,
// so a toggle can never make the defaults flip some other group (no auto-collapse after a choice).
export function toggleMode(viewModes: ViewModes, children: LevelChild[], id: string): ViewModes {
    const next: ViewModes = { ...viewModes };
    for (const child of children) {
        if ((child.child_count ?? 0) <= 0) continue;
        next[child.id] = child.expanded ? "group" : "summary";
    }
    const target = children.find((child) => child.id === id);
    const wasOpen = target ? Boolean(target.expanded) : viewModes[id] === "group";
    next[id] = wasOpen ? "summary" : "group";
    return next;
}

// "6 · 3 service, 2 store, 1 db +2" from the server's count-sorted type mix (at most 3 entries).
export function summaryText(childCount: number, typeMix?: { type: string; count: number }[]): string {
    const mix = typeMix ?? [];
    if (mix.length === 0) return `${childCount}`;
    const shown = mix.slice(0, 3).map((entry) => `${entry.count} ${entry.type}`);
    const rest = mix.length - shown.length;
    return `${childCount} · ${shown.join(", ")}${rest > 0 ? ` +${rest}` : ""}`;
}
