import type { TypeMixEntry } from "./api";

// Full type mix for a summary card, never shortened: "8 service, 2 module, 1 data store".
export function typeMixText(typeMix?: TypeMixEntry[]): string {
    return (typeMix ?? []).map((entry) => `${entry.count} ${entry.type || "untyped"}`).join(", ");
}

// "11 · 8 service, 2 module, 1 data store"; just the count when there is no mix.
export function fullSummaryText(childCount: number, typeMix?: TypeMixEntry[]): string {
    const mix = typeMixText(typeMix);
    return mix === "" ? `${childCount}` : `${childCount} · ${mix}`;
}
