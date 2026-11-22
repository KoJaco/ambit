// Shared visual vocabulary for relationship kinds. The canvas edges and the relationship panel both
// read this table so a kind looks the same everywhere.

export type KindGlyphId = "arrow" | "dots" | "data";
export type EdgeKindId = "sync" | "async" | "data" | "unknown";

export type EdgeKindStyle = {
    id: EdgeKindId;
    // Short human name, e.g. "Synchronous". Unknown kinds echo the raw kind when there is one.
    label: string;
    // SVG stroke-dasharray, undefined for a solid line.
    dash?: string;
    animated: boolean;
    glyph: KindGlyphId;
    description: string;
};

const STYLES: Record<Exclude<EdgeKindId, "unknown">, EdgeKindStyle> = {
    sync: {
        id: "sync",
        label: "Synchronous",
        animated: false,
        glyph: "arrow",
        description: "The caller waits for a response before it continues.",
    },
    async: {
        id: "async",
        label: "Asynchronous",
        dash: "6 6",
        animated: true,
        glyph: "dots",
        description: "The caller hands work off, such as an event or a message, and does not wait for the result.",
    },
    data: {
        id: "data",
        label: "Data flow",
        animated: false,
        glyph: "data",
        description: "Data moves from the source to the target, for example reads, writes, or replication.",
    },
};

export function edgeKindStyle(kind?: string | null): EdgeKindStyle {
    const key = (kind ?? "").trim().toLowerCase();
    if (key === "sync" || key === "async" || key === "data") return STYLES[key];
    return {
        id: "unknown",
        label: (kind ?? "").trim() || "Unspecified",
        animated: false,
        glyph: "arrow",
        description: "No interaction style is recorded for this relationship.",
    };
}
