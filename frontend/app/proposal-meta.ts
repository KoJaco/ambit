// Identifies a proposal in the Review panel so two with the same summary are distinguishable.

/** Relative time for recent proposals ("3 min ago"), an absolute date otherwise; "" when unknown. */
export function proposalAge(createdAt: string | undefined, now: Date = new Date()): string {
    if (!createdAt) return "";
    const then = new Date(createdAt);
    if (Number.isNaN(then.getTime())) return "";
    const seconds = Math.round((now.getTime() - then.getTime()) / 1000);
    if (seconds < 45) return "just now";
    const minutes = Math.round(seconds / 60);
    if (minutes < 60) return `${minutes} min ago`;
    const hours = Math.round(minutes / 60);
    if (hours < 24) return `${hours} h ago`;
    return then.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

/** Full local timestamp for a tooltip or machine-readable attribute; "" when unknown. */
export function proposalTimestamp(createdAt: string | undefined): string {
    if (!createdAt) return "";
    const then = new Date(createdAt);
    return Number.isNaN(then.getTime()) ? "" : then.toLocaleString();
}

/** The change-set label for a proposal, trimmed; "" when none. Groups related proposals. */
export function changeSetLabel(changeSet: string | undefined): string {
    return changeSet?.trim() ?? "";
}
