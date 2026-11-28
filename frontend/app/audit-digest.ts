// Formatting and grouping for the "Architect accepted" digest.
import type { AuditDigest, AuditEntry } from "./api";
import { proposalAge, proposalTimestamp } from "./proposal-meta";

export function shortProposalId(id: string): string {
    return id.length > 8 ? id.slice(0, 8) : id;
}

export function digestSummary(d: Pick<AuditDigest, "proposals" | "ops">): string {
    const p = `${d.proposals} ${d.proposals === 1 ? "proposal" : "proposals"}`;
    const o = `${d.ops} ${d.ops === 1 ? "change" : "changes"}`;
    return `${p} accepted automatically, ${o}.`;
}

/** Rules by descending count, ties alphabetical. Never mutates the input. */
export function ruleRows(byRule: AuditDigest["by_rule"] | undefined): { rule: string; count: number }[] {
    return [...(byRule ?? [])].sort((a, b) => b.count - a.count || a.rule.localeCompare(b.rule));
}

export type EntryView = {
    key: string;
    shortId: string;
    age: string;
    timestamp: string;
    actor: string;
    ops: { key: string; text: string; rule: string }[];
};

export function opText(op: { op: string; node: string; diff_summary: string }): string {
    const head = op.node ? `${op.op} ${op.node}` : op.op;
    return op.diff_summary ? `${head}: ${op.diff_summary}` : head;
}

/** Newest first. */
export function entryViews(entries: AuditEntry[] | undefined, now: Date = new Date()): EntryView[] {
    return [...(entries ?? [])]
        .sort((a, b) => Date.parse(b.ts) - Date.parse(a.ts) || 0)
        .map((e, i) => ({
            key: `${e.proposal_id}-${e.ts}-${i}`,
            shortId: shortProposalId(e.proposal_id),
            age: proposalAge(e.ts, now),
            timestamp: proposalTimestamp(e.ts),
            actor: e.actor === "human" ? "Human" : "Architect agent",
            ops: (e.ops ?? []).map((o) => ({ key: `${e.proposal_id}-${o.index}`, text: opText(o), rule: o.rule })),
        }));
}

export function ruleLabel(rule: string): string {
    return rule === "human" ? "human accept" : rule;
}
