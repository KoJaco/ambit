// Pure helpers for canvas structure edits (group, ungroup, reparent). Edits are proposals, so
// nothing here changes the model; it only decides what to ask the server to stage.
import { isStackedCard } from "./canvas-presentation";

export type Rect = { id: string; x: number; y: number; width: number; height: number };

export const defaultGroupBase = "Group";

// "Group", then "Group 2", "Group 3"... skipping names already used on the level (case-insensitive).
export function defaultGroupName(existingNames: string[], base: string = defaultGroupBase): string {
    const used = new Set(existingNames.map((name) => name.trim().toLowerCase()));
    if (!used.has(base.toLowerCase())) return base;
    for (let n = 2; ; n += 1) {
        const candidate = `${base} ${n}`;
        if (!used.has(candidate.toLowerCase())) return candidate;
    }
}

export function pointInRect(point: { x: number; y: number }, rect: Rect): boolean {
    return (
        point.x >= rect.x &&
        point.x <= rect.x + rect.width &&
        point.y >= rect.y &&
        point.y <= rect.y + rect.height
    );
}

// Whether a dragged node can be dropped into this canvas node: an open group, or a collapsed
// summary card (a group shown as a stacked card).
export function acceptsDrop(node: {
    type?: string;
    data?: { renderer?: string; child_count?: number; expanded?: boolean };
}): boolean {
    if (node.type === "container") return true;
    return isStackedCard(node.data?.renderer, node.data?.child_count, node.data?.expanded);
}

// The innermost (smallest) rect under the point, ignoring excluded ids. Null when none.
export function containerAt(
    point: { x: number; y: number },
    rects: Rect[],
    exclude: string[] = [],
): string | null {
    let best: Rect | null = null;
    for (const rect of rects) {
        if (exclude.includes(rect.id) || !pointInRect(point, rect)) continue;
        if (!best || rect.width * rect.height < best.width * best.height) best = rect;
    }
    return best?.id ?? null;
}

// Where a node dragged out of its group lands: the container under the pointer, else the level
// itself. `levelNodeId` is the node whose level is shown ("" at the model root). An empty result
// means the model root, so on a nested level the level's own node id must be used.
export function outsideReparentTarget(found: string | null, levelNodeId: string | undefined): string {
    return found ?? levelNodeId ?? "";
}

export type SelectableNode = { id: string; selected?: boolean; parentId?: string };

// Ids that can be grouped together: at least two selected nodes that share the same parent.
// Returns [] otherwise (the server requires siblings; this keeps the toolbar honest).
export function groupableSelection(nodes: SelectableNode[]): string[] {
    const selected = nodes.filter((node) => node.selected);
    if (selected.length < 2) return [];
    const parent = selected[0].parentId ?? "";
    if (selected.some((node) => (node.parentId ?? "") !== parent)) return [];
    return selected.map((node) => node.id);
}

export function groupButtonLabel(count: number): string {
    return `Group ${count} nodes`;
}

// Trim and fall back to the suggestion so Enter on an emptied field still does something sensible.
export function resolveGroupName(input: string, fallback: string): string {
    const trimmed = input.trim();
    return trimmed === "" ? fallback : trimmed;
}

export function stagedMessage(): string {
    return "Proposal staged. Review it in Proposals.";
}

export function stageErrorMessage(err: unknown): string {
    if (err instanceof Error && err.message) return err.message;
    return "Could not stage the proposal";
}

// Only summary cards and leaves drill on double-click. An expanded group already shows its
// children on this level, so opening it would repeat the same picture (decision 0010).
export function drillsOnDoubleClick(nodeType: string | undefined): boolean {
    return nodeType !== "container";
}
