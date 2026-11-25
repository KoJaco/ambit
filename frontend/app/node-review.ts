import type { NodeDetail, NodePatch, NodeReview, NodeRisk } from "./api";
import { normalizeContract, normalizeDependsOn, saveErrorTarget } from "./node-contract";

export const VERIFY_MAX_ITEMS = 8;
export const VERIFY_ITEM_MAX = 300;

export const RISK_OPTIONS: { value: NodeRisk; label: string }[] = [
    { value: "", label: "Not set" },
    { value: "low", label: "Low" },
    { value: "medium", label: "Medium" },
    { value: "high", label: "High" },
];

export const REVIEW_OPTIONS: { value: NodeReview; label: string }[] = [
    { value: "", label: "Default" },
    { value: "none", label: "Not required" },
    { value: "required", label: "Required" },
];

type ReviewFields = { risk?: NodeRisk; review?: NodeReview; verify?: string[] };

// Blank rows are dropped and commands trimmed, as on save.
export function normalizeVerify(verify: string[] | undefined): string[] {
    return (verify ?? []).map((v) => v.trim()).filter((v) => v !== "");
}

export type VerifyProblems = { list?: string; items: Record<number, string> };

// Messages for the draft as typed (indexes are rows as shown).
export function verifyProblems(verify: string[] | undefined): VerifyProblems {
    const items = verify ?? [];
    const out: VerifyProblems = { items: {} };
    if (items.filter((i) => i.trim() !== "").length > VERIFY_MAX_ITEMS) out.list = `At most ${VERIFY_MAX_ITEMS} commands.`;
    items.forEach((item, i) => {
        if (item.trim().length > VERIFY_ITEM_MAX) out.items[i] = `At most ${VERIFY_ITEM_MAX} characters.`;
    });
    return out;
}

export function validateVerify(verify: string[] | undefined): string[] {
    const p = verifyProblems(verify);
    return [...(p.list ? [p.list] : []), ...Object.values(p.items)];
}

// "risk high, review required, 2 commands"; "none" when nothing is set.
export function reviewSummary(node: ReviewFields): string {
    const parts: string[] = [];
    if (node.risk) parts.push(`risk ${node.risk}`);
    if (node.review) parts.push(node.review === "required" ? "review required" : "review not required");
    const n = normalizeVerify(node.verify).length;
    if (n > 0) parts.push(`${n} ${n === 1 ? "command" : "commands"}`);
    return parts.length ? parts.join(", ") : "none";
}

export function reviewChanged(a: ReviewFields | null | undefined, b: ReviewFields | null | undefined): boolean {
    return (
        (a?.risk ?? "") !== (b?.risk ?? "") ||
        (a?.review ?? "") !== (b?.review ?? "") ||
        JSON.stringify(normalizeVerify(a?.verify)) !== JSON.stringify(normalizeVerify(b?.verify))
    );
}

// Where a save error is shown: review failures sit next to their fields.
export function isReviewError(message: string): boolean {
    return /\bnode (risk|review)\b|verify command/i.test(message);
}

// Review failures are checked first: Go wraps them as "invalid contract: ...", which would
// otherwise be routed to the Contract section.
export function resolveSaveErrorTarget(message: string): "depends" | "contract" | "review" | "general" {
    return isReviewError(message) ? "review" : saveErrorTarget(message);
}

// Blank contract rows and ordering do not count as edits.
export function comparable(node: NodeDetail | null): unknown {
    return (
        node && {
            ...node,
            depends_on: normalizeDependsOn(node.depends_on),
            contract: normalizeContract(node.contract),
            risk: node.risk ?? "",
            review: node.review ?? "",
            verify: normalizeVerify(node.verify),
        }
    );
}

function lines(value: string[]): string[] {
    return value.map((line) => line.trim()).filter((line) => line !== "");
}

// presentation=false leaves icon/colour/renderer/schema untouched, as a save without appearance edits does.
// `compare` is the node as it was before the edit: depends_on, contract and each of risk/review/verify
// are sent only when they differ from it (or, with `only`, when that patch sent them), so a save
// never overwrites a change made elsewhere.
export function toPatch(node: NodeDetail, presentation: boolean, compare?: NodeDetail | null, only?: NodePatch): NodePatch {
    const patch: NodePatch = {
        name: node.name,
        type: node.type,
        status: node.status,
        implementation: lines(node.implementation),
        scope: lines(node.scope),
        protected: node.protected,
        markdown: node.markdown,
    };
    const deps = normalizeDependsOn(node.depends_on);
    const contract = normalizeContract(node.contract);
    const sendDeps = only
        ? only.depends_on !== undefined
        : !!compare && JSON.stringify(deps) !== JSON.stringify(normalizeDependsOn(compare.depends_on));
    const sendContract = only
        ? only.contract !== undefined
        : !!compare && JSON.stringify(contract) !== JSON.stringify(normalizeContract(compare.contract));
    if (sendDeps) patch.depends_on = deps;
    if (sendContract) patch.contract = contract;
    const verify = normalizeVerify(node.verify);
    const sendRisk = only ? only.risk !== undefined : !!compare && (node.risk ?? "") !== (compare.risk ?? "");
    const sendReview = only ? only.review !== undefined : !!compare && (node.review ?? "") !== (compare.review ?? "");
    const sendVerify = only
        ? only.verify !== undefined
        : !!compare && JSON.stringify(verify) !== JSON.stringify(normalizeVerify(compare.verify));
    if (sendRisk) patch.risk = node.risk ?? "";
    if (sendReview) patch.review = node.review ?? "";
    if (sendVerify) patch.verify = verify;
    if (presentation) {
        patch.icon = node.icon ?? "";
        patch.color = node.color ?? "";
        patch.renderer = node.renderer ?? "";
        patch.schema = (node.schema ?? []).filter((row) => row.title.trim() !== "" && row.type.trim() !== "");
    }
    return patch;
}
