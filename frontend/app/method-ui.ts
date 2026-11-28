import type { CheckModelResult, Level, LintFinding, LintSeverity, MethodRule, PutMethodResult, Relationship, StageInfo } from "./api";

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

// What a finding points at, or null when it is about the whole model.
export type FindingTarget = { kind: "node" | "relationship"; id: string };

export function findingTarget(f: LintFinding): FindingTarget | null {
    if (f.node_id) return { kind: "node", id: f.node_id };
    if (f.relationship_id) return { kind: "relationship", id: f.relationship_id };
    return null;
}

// Route of the level that contains a node (its parent), or "/" for a root node.
export function containingLevelHref(parentId: string | null | undefined): string {
    return parentId ? `/node/${encodeURIComponent(parentId)}` : "/";
}

// Router location state read by the home route to select something after "Go to".
export type GoToSelect =
    | { kind: "node"; id: string; name: string }
    | { kind: "edge"; rel: Relationship; name: string };

export function readGoToSelect(state: unknown): GoToSelect | null {
    const sel = (state as { select?: GoToSelect } | null | undefined)?.select;
    if (!sel || typeof sel !== "object") return null;
    if (sel.kind === "node" && typeof sel.id === "string") return sel;
    if (sel.kind === "edge" && sel.rel && typeof sel.rel.id === "string") return sel;
    return null;
}

export function relationshipDisplay(rel: { label?: string; from: string; to: string } | undefined, id: string, nameOf: (id: string) => string): string {
    if (!rel) return id;
    return rel.label?.trim() ? rel.label.trim() : `${nameOf(rel.from)} \u2192 ${nameOf(rel.to)}`;
}

// Unique, name-based accessible name for a Go-to button.
export function goToLabel(kind: FindingTarget["kind"], display: string, rule: string): string {
    return `Go to ${kind} ${display}, ${rule}`;
}

export type LocatedRelationship = { rel: Relationship; levelId: string | null };

// Walks levels breadth-first from the root to find the level whose edges include each wanted
// relationship id. Stops early once all are found; `maxLevels` bounds the walk.
export async function locateRelationships(
    wanted: Iterable<string>,
    fetchLevel: (nodeId?: string) => Promise<Pick<Level, "children" | "relationships">>,
    maxLevels = 200,
): Promise<Map<string, LocatedRelationship>> {
    const left = new Set(wanted);
    const found = new Map<string, LocatedRelationship>();
    const queue: (string | null)[] = [null];
    for (let i = 0; i < queue.length && i < maxLevels && left.size > 0; i++) {
        const levelId = queue[i];
        let level;
        try {
            level = await fetchLevel(levelId ?? undefined);
        } catch {
            continue;
        }
        for (const rel of level.relationships) {
            if (!rel.aggregated && left.has(rel.id)) {
                found.set(rel.id, { rel, levelId });
                left.delete(rel.id);
            }
        }
        for (const child of level.children) if ((child.child_count ?? 0) > 0) queue.push(child.id);
    }
    return found;
}

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;

export function checkStatus(result: Pick<CheckModelResult, "ok" | "counts">): string {
    const { enforced, advisory, total } = result.counts;
    if (total === 0) return "OK: no issues found.";
    const summary = `${plural(enforced, "enforced issue", "enforced issues")}, ${plural(advisory, "advisory issue", "advisory issues")}`;
    return `${result.ok ? "OK, with advisory issues" : "Issues found"}: ${summary}.`;
}

// The status line of the Model check sheet. A failure replaces any earlier outcome.
export function checkStatusLine(s: { loading: boolean; error: string | null; result: CheckModelResult | null }): string {
    if (s.loading) return "Checking the model.";
    if (s.error) return `Model check failed: ${s.error}`;
    return s.result ? checkStatus(s.result) : "";
}

// Rule params as "key value" pairs for the rules table, or "" when there are none.
export function formatRuleParams(params: MethodRule["params"]): string {
    if (!params) return "";
    return Object.entries(params)
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([k, v]) => `${k} ${typeof v === "string" ? v : JSON.stringify(v)}`)
        .join(", ");
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

// Notice shown after switching method. Switching changes policy only. `res` is the server's
// PUT /method answer: ids as strings, stage "" when the method has none.
export function switchNotice(
    methodName: string,
    res: Pick<PutMethodResult, "stage" | "stage_reset" | "previous_stage">,
    stageName: (id: string) => string = (id) => id,
): string {
    const base = `Switched to ${methodName}.`;
    const note = " Your model was not changed.";
    const prev = res.previous_stage ?? "";
    if (!res.stage) return prev ? `${base} Stage cleared, this method has no stages.${note}` : `${base}${note}`;
    if (prev === res.stage && !res.stage_reset) return `${base} Stage stays ${stageName(res.stage)}.${note}`;
    return `${base} Stage set to ${stageName(res.stage)}.${note}`;
}

export type StageGateRow = { id: string; stage: string; statuses: string[]; current: boolean };

// Every stage of the method in order, each with the statuses agents cannot set there (empty when
// the stage has no gate), and the current stage marked. Gates for ids that are not stages are kept last.
export function stageGateRows(gates: Record<string, string[]>, stages: StageInfo[], currentId: string | null | undefined): StageGateRow[] {
    const known = new Set(stages.map((s) => s.id));
    const rows: StageGateRow[] = stages.map((s) => ({ id: s.id, stage: s.name, statuses: gates[s.id] ?? [], current: s.id === currentId }));
    for (const id of Object.keys(gates).sort()) {
        if (!known.has(id) && gates[id].length > 0) rows.push({ id, stage: id, statuses: gates[id], current: id === currentId });
    }
    return rows;
}

export function gatesSummary(gates: Record<string, string[]>, stages: StageInfo[]): { stage: string; statuses: string[] }[] {
    const names = new Map(stages.map((s) => [s.id, s.name]));
    const order = new Map(stages.map((s, i) => [s.id, i]));
    return Object.entries(gates)
        .filter(([, statuses]) => statuses.length > 0)
        .sort(([a], [b]) => (order.get(a) ?? 99) - (order.get(b) ?? 99) || a.localeCompare(b))
        .map(([id, statuses]) => ({ stage: names.get(id) ?? id, statuses }));
}
