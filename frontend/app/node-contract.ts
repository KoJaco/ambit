import type { BuildOrder, Constraint, EdgeContract, NodeContract } from "./api";

// Pure helpers for the node model (decision 0012): contracts, dependencies, project rules and
// the advisory build order. Limits mirror the server; the server stays the authority.

export const PAYLOADS_MAX = 8000;
export const LIST_MAX_ITEMS = 32;
export const LIST_ITEM_MAX = 300;
export const EDGE_TEXT_MAX = 4000;
export const EDGE_REF_MAX = 300;
export const CONSTRAINTS_MAX = 24;
export const CONSTRAINT_TEXT_MAX = 300;

export type ContractListKey = "invariants" | "errors" | "owns_data";

export const CONTRACT_LISTS: { key: ContractListKey; label: string; addLabel: string; itemLabel: string }[] = [
    { key: "invariants", label: "Invariants", addLabel: "Add invariant", itemLabel: "Invariant" },
    { key: "errors", label: "Errors", addLabel: "Add error", itemLabel: "Error" },
    { key: "owns_data", label: "Owned data", addLabel: "Add owned data", itemLabel: "Owned data" },
];

function trimmedList(list: string[] | undefined): string[] {
    return (list ?? []).map((item) => item.trim()).filter((item) => item !== "");
}

// What is sent on save: trimmed, blank rows dropped, empty fields omitted. `{}` clears on PATCH.
export function normalizeContract(contract: NodeContract | undefined): NodeContract {
    const out: NodeContract = {};
    const payloads = (contract?.payloads ?? "").trim();
    if (payloads) out.payloads = payloads;
    for (const { key } of CONTRACT_LISTS) {
        const list = trimmedList(contract?.[key]);
        if (list.length) out[key] = list;
    }
    return out;
}

export function contractIsEmpty(contract: NodeContract | undefined): boolean {
    return Object.keys(normalizeContract(contract)).length === 0;
}

// Dependencies as sent: unique, blank dropped, sorted for a stable comparison.
export function normalizeDependsOn(ids: string[] | undefined): string[] {
    return [...new Set(trimmedList(ids))].sort();
}

// Validation messages for the contract as it would be sent. Empty array: nothing to flag.
export function validateContract(contract: NodeContract | undefined): string[] {
    const normal = normalizeContract(contract);
    const problems: string[] = [];
    if ((normal.payloads ?? "").length > PAYLOADS_MAX) problems.push(`Payloads: at most ${PAYLOADS_MAX} characters.`);
    for (const { key, label } of CONTRACT_LISTS) {
        const list = normal[key] ?? [];
        if (list.length > LIST_MAX_ITEMS) problems.push(`${label}: at most ${LIST_MAX_ITEMS} items.`);
        if (list.some((item) => item.length > LIST_ITEM_MAX)) problems.push(`${label}: each item at most ${LIST_ITEM_MAX} characters.`);
    }
    return problems;
}

export type CounterState = { text: string; near: boolean; over: boolean };

// "7,950 / 8,000"; `near` from 90 percent so the warning does not rely on colour alone.
export function counterState(length: number, max: number): CounterState {
    return {
        text: `${length} / ${max}`,
        near: length >= max * 0.9,
        over: length > max,
    };
}

function plural(n: number, one: string, many = `${one}s`): string {
    return `${n} ${n === 1 ? one : many}`;
}

// "payloads, 2 invariants, 1 error"; "none" for an empty contract.
export function contractSummary(contract: NodeContract | undefined): string {
    const normal = normalizeContract(contract);
    const parts: string[] = [];
    if (normal.payloads) parts.push("payloads");
    if (normal.invariants) parts.push(plural(normal.invariants.length, "invariant"));
    if (normal.errors) parts.push(plural(normal.errors.length, "error"));
    if (normal.owns_data) parts.push(plural(normal.owns_data.length, "owned item", "owned items"));
    return parts.length ? parts.join(", ") : "none";
}

// Short before/after text for a review row: names which parts changed, not their content.
export function contractChangeSummary(before: NodeContract | undefined, after: NodeContract | undefined): string {
    const b = normalizeContract(before);
    const a = normalizeContract(after);
    if (JSON.stringify(b) === JSON.stringify(a)) return "unchanged";
    if (Object.keys(a).length === 0) return "cleared";
    const parts: string[] = [];
    if ((b.payloads ?? "") !== (a.payloads ?? "")) parts.push("payloads");
    for (const { key, label } of CONTRACT_LISTS) {
        const x = b[key] ?? [];
        const y = a[key] ?? [];
        if (JSON.stringify(x) !== JSON.stringify(y)) parts.push(`${label.toLowerCase()} ${x.length} → ${y.length}`);
    }
    return `changed: ${parts.join(", ")}`;
}

export function dependsText(ids: string[] | undefined, nameOf: (id: string) => string = (id) => id): string {
    const list = normalizeDependsOn(ids);
    return list.length ? list.map(nameOf).join(", ") : "(none)";
}

// ---- edge contract ----------------------------------------------------------------------------

export const EDGE_FORMATS: { value: NonNullable<EdgeContract["format"]>; label: string }[] = [
    { value: "openapi", label: "OpenAPI" },
    { value: "jsonschema", label: "JSON Schema" },
    { value: "text", label: "Text" },
];

export function edgeFormatLabel(format: EdgeContract["format"] | undefined): string {
    return EDGE_FORMATS.find((f) => f.value === format)?.label ?? "";
}

// What is sent on save; `{}` clears.
export function normalizeEdgeContract(contract: EdgeContract | undefined): EdgeContract {
    const out: EdgeContract = {};
    if (contract?.format) out.format = contract.format;
    const ref = (contract?.ref ?? "").trim();
    if (ref) out.ref = ref;
    const text = (contract?.text ?? "").trim();
    if (text) out.text = text;
    return out;
}

export function validateEdgeContract(contract: EdgeContract | undefined): { ref?: string; text?: string } {
    const normal = normalizeEdgeContract(contract);
    const errors: { ref?: string; text?: string } = {};
    if (normal.ref !== undefined) {
        if (normal.ref.length > EDGE_REF_MAX) errors.ref = `Reference: at most ${EDGE_REF_MAX} characters.`;
        else if (normal.ref.includes("..")) errors.ref = "Reference must not contain '..'.";
    }
    if ((normal.text ?? "").length > EDGE_TEXT_MAX) errors.text = `Text: at most ${EDGE_TEXT_MAX} characters.`;
    return errors;
}

export function edgeContractDirty(saved: EdgeContract | undefined, draft: EdgeContract | undefined): boolean {
    return JSON.stringify(normalizeEdgeContract(saved)) !== JSON.stringify(normalizeEdgeContract(draft));
}

// One line for review rows, e.g. "OpenAPI · api/orders.yaml"; "" when there is none.
export function edgeContractLine(contract: EdgeContract | undefined): string {
    const normal = normalizeEdgeContract(contract);
    const parts = [edgeFormatLabel(normal.format), normal.ref, normal.text ? "text" : ""].filter(Boolean);
    return parts.join(" · ");
}

// ---- project rules (constraints) ------------------------------------------------------------

export const SEVERITIES: Constraint["severity"][] = ["must", "should"];

export function nextConstraintId(list: Constraint[]): string {
    const used = new Set(list.map((c) => c.id));
    for (let n = list.length + 1; ; n += 1) {
        const id = `rule-${n}`;
        if (!used.has(id)) return id;
    }
}

export function addConstraint(list: Constraint[]): Constraint[] {
    if (list.length >= CONSTRAINTS_MAX) return list;
    return [...list, { id: nextConstraintId(list), text: "", severity: "must" }];
}

export function removeConstraint(list: Constraint[], index: number): Constraint[] {
    return list.filter((_, i) => i !== index);
}

// Moves one row by `delta` positions; out-of-range moves return the list unchanged.
export function moveConstraint(list: Constraint[], index: number, delta: number): Constraint[] {
    const to = index + delta;
    if (index < 0 || index >= list.length || to < 0 || to >= list.length) return list;
    const next = [...list];
    const [item] = next.splice(index, 1);
    next.splice(to, 0, item);
    return next;
}

export function setConstraintText(list: Constraint[], index: number, text: string): Constraint[] {
    return list.map((c, i) => (i === index ? { ...c, text } : c));
}

export function toggleSeverity(list: Constraint[], index: number): Constraint[] {
    return list.map((c, i) => (i === index ? { ...c, severity: c.severity === "must" ? "should" : "must" } : c));
}

export type ConstraintsCheck = {
    // Per-row message, "" when the row is fine.
    rows: string[];
    // List-level message ("" when fine).
    list: string;
    ok: boolean;
};

export function validateConstraints(list: Constraint[]): ConstraintsCheck {
    const rows = list.map((c) => {
        const text = c.text.trim();
        if (text === "") return "Write the rule or remove the row.";
        if (text.length > CONSTRAINT_TEXT_MAX) return `At most ${CONSTRAINT_TEXT_MAX} characters.`;
        return "";
    });
    const listMessage = list.length > CONSTRAINTS_MAX ? `At most ${CONSTRAINTS_MAX} rules.` : "";
    return { rows, list: listMessage, ok: !listMessage && rows.every((m) => m === "") };
}

// What is sent on PUT: text trimmed.
export function normalizeConstraints(list: Constraint[]): Constraint[] {
    return list.map((c) => ({ ...c, text: c.text.trim() }));
}

export function constraintsDirty(saved: Constraint[], draft: Constraint[]): boolean {
    return JSON.stringify(normalizeConstraints(saved)) !== JSON.stringify(normalizeConstraints(draft));
}

export type ConstraintLine = {
    kind: "added" | "removed" | "changed";
    // Text shown for the rule (the proposed text for changed).
    text: string;
    // Previous text and severity for a changed line.
    was?: string;
};

function ruleText(c: Constraint): string {
    return `${c.severity.toUpperCase()}: ${c.text}`;
}

// Added, removed and changed rules, matched by id. Order-only differences are not listed.
export function constraintsDiff(current: Constraint[] | undefined, proposed: Constraint[] | undefined): ConstraintLine[] {
    const cur = current ?? [];
    const prop = proposed ?? [];
    const curById = new Map(cur.map((c) => [c.id, c]));
    const propIds = new Set(prop.map((c) => c.id));
    const lines: ConstraintLine[] = [];
    for (const c of prop) {
        const old = curById.get(c.id);
        if (!old) lines.push({ kind: "added", text: ruleText(c) });
        else if (old.text !== c.text || old.severity !== c.severity) {
            lines.push({ kind: "changed", text: ruleText(c), was: ruleText(old) });
        }
    }
    for (const c of cur) if (!propIds.has(c.id)) lines.push({ kind: "removed", text: ruleText(c) });
    return lines;
}

// "Project rules: 1 added, 2 removed, 1 changed" for a row's change line.
export function constraintsChangeText(current: Constraint[] | undefined, proposed: Constraint[] | undefined): string {
    const lines = constraintsDiff(current, proposed);
    if (lines.length === 0) return (current ?? []).length === (proposed ?? []).length ? "Reorders project rules" : "Updates project rules";
    const count = (kind: ConstraintLine["kind"]) => lines.filter((l) => l.kind === kind).length;
    const parts = (["added", "removed", "changed"] as const).map((k) => (count(k) ? `${count(k)} ${k}` : "")).filter(Boolean);
    return `Project rules: ${parts.join(", ")}`;
}

// ---- build order ------------------------------------------------------------------------------

export type BuildBadge = {
    wave: number | null;
    // Advisory: names of unfinished dependencies the node waits on.
    blockedBy: { id: string; name: string }[];
    done: boolean;
    inCycle: boolean;
    // One line for the screen reader and the badge text.
    text: string;
};

// Null when the node is not part of the order (for example a container) and not in a cycle.
export function buildBadge(order: BuildOrder | null | undefined, id: string): BuildBadge | null {
    if (!order) return null;
    const item = order.order.find((o) => o.id === id);
    const inCycle = order.cycles.some((c) => c.includes(id)) || order.unordered.includes(id);
    if (!item) {
        return inCycle
            ? { wave: null, blockedBy: [], done: false, inCycle, text: "Not ordered: part of a dependency cycle" }
            : null;
    }
    const names = new Map(order.order.map((o) => [o.id, o.name]));
    const blockedBy = item.blocked_by.map((b) => ({ id: b, name: names.get(b) ?? b }));
    const parts = [`wave ${item.wave}`];
    if (item.done) parts.push("done");
    if (blockedBy.length) parts.push(`blocked by ${blockedBy.map((b) => b.name).join(", ")}`);
    if (inCycle) parts.push("in a dependency cycle");
    return { wave: item.wave, blockedBy, done: item.done, inCycle, text: parts.join(" · ") };
}

export type DependsOption = { id: string; name: string };

// Picker options: every other node the build order knows, plus current dependencies it does not
// (so an existing dependency is never silently hidden). Sorted by name.
export function dependsOptions(order: BuildOrder | null | undefined, selfId: string, current: string[] | undefined): DependsOption[] {
    const byId = new Map<string, string>();
    for (const item of order?.order ?? []) byId.set(item.id, item.name);
    for (const id of current ?? []) if (!byId.has(id)) byId.set(id, id);
    byId.delete(selfId);
    return [...byId].map(([id, name]) => ({ id, name })).sort((a, b) => a.name.localeCompare(b.name) || a.id.localeCompare(b.id));
}

export function toggleDependency(current: string[] | undefined, id: string, selfId: string): string[] {
    if (id === selfId) return current ?? [];
    const list = current ?? [];
    return list.includes(id) ? list.filter((d) => d !== id) : [...list, id];
}

// Where a save error is shown: dependency and contract failures sit next to their fields.
export function saveErrorTarget(message: string): "depends" | "contract" | "general" {
    const text = message.toLowerCase();
    if (/depend|cycle/.test(text)) return "depends";
    if (/contract|payload|invariant|owns_data/.test(text)) return "contract";
    return "general";
}
