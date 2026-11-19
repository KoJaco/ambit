import type { SchemaColumn } from "./api";

export const RENDERER_IDS = ["card", "schema", "placeholder", "group"] as const;

export type CanvasNodeType = "card" | "schema" | "placeholder" | "group" | "container";
export type CanvasEdgeType = "arch" | "button" | "animated" | "data";

// An open group is always a container. A collapsed node with children is a summary card, even
// when its renderer is `group` (0010): the static group frame stays for leaves only.
export function canvasNodeType(renderer?: string, childCount = 0, expanded = false): CanvasNodeType {
    if (expanded) return "container";
    if (renderer === "group" && childCount > 0) return "card";
    if (renderer === "schema" || renderer === "placeholder" || renderer === "group") return renderer;
    return "card";
}

export function canvasEdgeType(kind?: string): CanvasEdgeType {
    switch (kind) {
        case "sync":
            return "button";
        case "async":
            return "animated";
        case "data":
            return "data";
        default:
            return "arch";
    }
}

export function nodeLayoutSize(renderer?: string, schemaLength = 0): { width: number; height: number } {
    switch (canvasNodeType(renderer)) {
        case "schema":
            return { width: 280, height: Math.max(120, 56 + Math.max(schemaLength, 1) * 28) };
        case "group":
            return { width: 280, height: 140 };
        case "placeholder":
            return { width: 160, height: 72 };
        default:
            return { width: 220, height: 96 };
    }
}

export function schemaRows(schema?: SchemaColumn[]): SchemaColumn[] {
    return (schema ?? []).filter((row) => row.title.trim() !== "" || row.type.trim() !== "");
}
