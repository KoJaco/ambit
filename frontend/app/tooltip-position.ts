// Pure placement for the tooltip. Prefers above the anchor, flips below when there is no room,
// and clamps horizontally so the tip never leaves the viewport.

export type BoxLike = { left: number; top: number; width: number; height: number };
export type Placement = "top" | "bottom";

export function placeTooltip(
    anchor: BoxLike,
    tip: { width: number; height: number },
    viewport: { width: number; height: number },
    options: { gap?: number; margin?: number; prefer?: Placement } = {},
): { left: number; top: number; placement: Placement } {
    const gap = options.gap ?? 6;
    const margin = options.margin ?? 8;
    const prefer = options.prefer ?? "top";
    const above = anchor.top - gap - tip.height;
    const below = anchor.top + anchor.height + gap;
    const fitsAbove = above >= margin;
    const fitsBelow = below + tip.height <= viewport.height - margin;
    let placement: Placement = prefer;
    if (prefer === "top" && !fitsAbove && fitsBelow) placement = "bottom";
    if (prefer === "bottom" && !fitsBelow && fitsAbove) placement = "top";
    const top = placement === "top" ? above : below;
    const centred = anchor.left + anchor.width / 2 - tip.width / 2;
    const maxLeft = Math.max(margin, viewport.width - margin - tip.width);
    const left = Math.min(Math.max(centred, margin), maxLeft);
    return { left, top, placement };
}
