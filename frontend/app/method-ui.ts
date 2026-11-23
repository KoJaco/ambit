import type { CheckModelResult, LintFinding, LintSeverity, MethodRule, PutMethodResult, StageInfo } from "./api";

// Pure helpers for the Method sheet, stage control and Model check sheet.

export const severityLabel: Record<LintSeverity, string> = {
    enforced: "Enforced",
    advisory: "Advisory",
    off: "Off",
};

const severityOrder: Record<LintSeverity, number> = { enforced: 0, advisory: 1, off: 2 };

// Rules table order: enforced, then advisory, then off; ties by id.
export function orderRules(rules: MethodRule[]): MethodRule[] {
    return [...rules].sort((a, b) => severityOrder[a.severity] - severityOrder[b.severity] || a.id.localeCompare(b.id));
}

export type FindingGroup = { severity: "enforced" | "advisory"; label: string; findings: LintFinding[] };

// Enforced first; within a group by rule, then node/relationship id, then message. Empty groups are dropped.
export function groupFindings(findings: LintFinding[]): FindingGroup[] {
    const key = (f: LintFinding) => f.node_id ?? f.relationship_id ?? "";
    const sorted = [...findings].sort(
        (a, b) => a.rule.localeCompare(b.rule) || key(a).localeCompare(key(b)) || a.message.localeCompare(b.message),
    );
    return (["enforced", "advisory"] as const)
        .map((severity) => ({
            severity,
            label: severityLabel[severity],
            findings: sorted.filter((f) => f.severity === severity),
        }))
        .filter((g) => g.findings.length > 0);
}

// Where "Go to" leads, or null when the finding is about the whole model.
export function findingTarget(f: LintFinding): { href: string; label: string } | null {
    if (f.node_id) return { href: `/node/${encodeURIComponent(f.node_id)}`, label: `node ${f.node_id}` };
    if (f.relationship_id) return { href: `/relationship/${encodeURIComponent(f.relationship_id)}`, label: `relationship ${f.relationship_id}` };
    return null;
}

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;

export function checkStatus(result: Pick<CheckModelResult, "ok" | "counts">): string {
    const { enforced, advisory, total } = result.counts;
    if (total === 0) return "OK: no issues found.";
    const summary = `${plural(enforced, "enforced issue", "enforced issues")}, ${plural(advisory, "advisory issue", "advisory issues")}`;
    return `${result.ok ? "OK, with advisory issues" : "Issues found"}: ${summary}.`;
}

export function driftCount(drift: CheckModelResult["drift"]): number {
    if (!drift) return 0;
    return (drift.unowned_files?.length ?? 0) + (drift.unmatched_globs?.length ?? 0) + (drift.overlaps?.length ?? 0);
}

export type StageMove = "later" | "earlier" | "same" | "unknown";

export function stageMove(stages: StageInfo[], currentId: string | null | undefined, targetId: string): StageMove {
    const to = stages.findIndex((s) => s.id === targetId);
    if (to < 0) return "unknown";
    const from = currentId ? stages.findIndex((s) => s.id === currentId) : -1;
    if (from === to) return "same";
    return to > from ? "later" : "earlier";
}

export function nextStage(stages: StageInfo[], currentId: string | null | undefined): StageInfo | null {
    const from = currentId ? stages.findIndex((s) => s.id === currentId) : -1;
    return stages[from + 1] ?? null;
}

// Moving to a later stage is checked first. Enforced findings ask for confirmation but never block.
export type StageDecision = { action: "apply" } | { action: "confirm"; enforced: number; message: string };

export function enforcedMessage(n: number): string {
    return `${plural(n, "enforced issue", "enforced issues")}. Advance anyway?`;
}

export function decideStageChange(move: StageMove, enforced: number): StageDecision {
    if (move === "later" && enforced > 0) return { action: "confirm", enforced, message: enforcedMessage(enforced) };
    return { action: "apply" };
}

export function stageChipLabel(stage: { name: string } | null | undefined): string {
    return `Stage: ${stage?.name ?? "not set"}`;
}

// Notice shown after switching method. Switching changes policy only.
export function switchNotice(methodName: string, res: Pick<PutMethodResult, "stage" | "stage_reset" | "previous_stage">): string {
    const base = `Switched to ${methodName}. Your model was not changed.`;
    if (!res.stage_reset) return base;
    const prev = res.previous_stage ? (typeof res.previous_stage === "string" ? res.previous_stage : (res.previous_stage.name ?? res.previous_stage.id)) : null;
    const was = prev ? ` The previous stage "${prev}" does not exist in this method.` : "";
    return res.stage
        ? `${base}${was} The stage was reset to ${res.stage.name}.`
        : `${base}${was} This method has no stages, so the stage was cleared.`;
}

export function gatesSummary(gates: Record<string, string[]>, stages: StageInfo[]): { stage: string; statuses: string[] }[] {
    const names = new Map(stages.map((s) => [s.id, s.name]));
    const order = new Map(stages.map((s, i) => [s.id, i]));
    return Object.entries(gates)
        .filter(([, statuses]) => statuses.length > 0)
        .sort(([a], [b]) => (order.get(a) ?? 99) - (order.get(b) ?? 99) || a.localeCompare(b))
        .map(([id, statuses]) => ({ stage: names.get(id) ?? id, statuses }));
}
