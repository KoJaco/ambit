import type { AssignmentState } from "./assignment";
import { previewScope } from "./scope-preview";

export type SettingsSummaryItem = {
    kind: "protected" | "assigned" | "scope";
    label: string;
};

// One-line state shown on the collapsed "Node settings" header so key state is never hidden.
export function nodeSettingsSummary(
    node: { implementation: string[]; scope: string[]; protected: boolean },
    assignment: AssignmentState,
): SettingsSummaryItem[] {
    const items: SettingsSummaryItem[] = [];
    if (node.protected) items.push({ kind: "protected", label: "Protected" });
    if (assignment === "this") items.push({ kind: "assigned", label: "Assigned" });
    else if (assignment === "other") items.push({ kind: "assigned", label: "Assigned elsewhere" });
    else if (assignment === "dangling") items.push({ kind: "assigned", label: "Assignment dangling" });
    const globs = previewScope(node).globs.length;
    items.push({ kind: "scope", label: globs === 0 ? "No scope set" : `${globs} ${globs === 1 ? "glob" : "globs"}` });
    return items;
}

export function nodeSettingsSummaryText(items: readonly SettingsSummaryItem[]): string {
    return items.map((item) => item.label).join(", ");
}
