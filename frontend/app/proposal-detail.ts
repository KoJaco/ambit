import type { NodeContract, NodeSnapshot, OpDiff, ProposalDiff, RelSnapshot } from "./api";
import {
    CONTRACT_LISTS,
    constraintsChangeText,
    constraintsDiff,
    contractChangeSummary,
    contractIsEmpty,
    contractSummary,
    dependsText,
    edgeContractLine,
    normalizeContract,
    normalizeDependsOn,
    type ConstraintLine,
} from "./node-contract";
import { resolveNodeColor, statusLabel, type ColorToken } from "./node-presentation";

export type DetailBlock =
    | { kind: "heading"; text: string; level?: 3 | 4 }
    | { kind: "line"; text: string }
    | { kind: "pair"; label: string; before: string; after: string }
    | { kind: "prose"; label: string; before: string; after: string }
    // Contract lists (invariants, errors, owned data): the items themselves, not only counts.
    | { kind: "list"; label: string; before: string[]; after: string[] };

const missingNode = "node is missing";
const noDependents = "node now has dependents";
// Reasons the server refuses even with confirm_stale.
const unconfirmable = (reason?: string) => reason === missingNode || reason === noDependents;

// Heading, status and any error or stale notice: always shown above the visual.
// `names` (id -> name) replaces raw ids once resolved.
export function proposalDetailHead(op: OpDiff, names?: ReadonlyMap<string, string>): DetailBlock[] {
    const blocks: DetailBlock[] = [];
    blocks.push({ kind: "heading", text: summarizeOp(op, names) });
    blocks.push({ kind: "line", text: `Status: ${op.status}` });
    if (op.error) {
        blocks.push({ kind: "line", text: op.error });
    }
    if (op.stale) {
        blocks.push({
            kind: "line",
            text: staleMessage(op),
        });
    }
    return blocks;
}

// The full text form of an operation: head plus every field.
export function proposalDetailBlocks(op: OpDiff, names?: ReadonlyMap<string, string>): DetailBlock[] {
    const blocks: DetailBlock[] = proposalDetailHead(op, names);
    const nm = (id: string) => names?.get(id) ?? id;

    switch (op.op) {
        case "create_node":
            if (op.proposed) {
                blocks.push(...nodeSnapshotBlocks("Proposed", op.proposed, nm));
                if (op.proposed.relationship_id) {
                    blocks.push({ kind: "line", text: `Interior: ${op.proposed.relationship_id}` });
                }
            }
            break;
        case "update_node":
            for (const field of op.fields ?? []) {
                if (field === "spec") {
                    blocks.push({
                        kind: "prose",
                        label: "Spec",
                        before: op.current?.markdown ?? "—",
                        after: op.proposed?.markdown ?? "—",
                    });
                } else if (field === "depends_on") {
                    blocks.push({
                        kind: "pair",
                        label: "Depends on",
                        before: dependsText(op.current?.depends_on, nm),
                        after: dependsText(op.proposed?.depends_on, nm),
                    });
                } else if (field === "contract") {
                    blocks.push({
                        kind: "pair",
                        label: "Contract",
                        before: contractSummary(op.current?.contract),
                        after: `${contractSummary(op.proposed?.contract)} (${contractChangeSummary(op.current?.contract, op.proposed?.contract)})`,
                    });
                    const before = normalizeContract(op.current?.contract).payloads ?? "";
                    const after = normalizeContract(op.proposed?.contract).payloads ?? "";
                    if (before !== after) blocks.push({ kind: "prose", label: "Contract payloads", before, after: after || "(empty)" });
                    blocks.push(...contractListBlocks(op.current?.contract, op.proposed?.contract));
                } else {
                    blocks.push({
                        kind: "pair",
                        label: field,
                        before: fieldText(op.current, field),
                        after: fieldText(op.proposed, field),
                    });
                }
            }
            break;
        case "set_constraints":
            blocks.push({ kind: "heading", text: "Project rules" });
            for (const line of constraintsDiff(op.current_constraints, op.proposed_constraints)) {
                blocks.push({ kind: "line", text: constraintLineText(line) });
            }
            if (constraintsDiff(op.current_constraints, op.proposed_constraints).length === 0) {
                blocks.push({ kind: "line", text: constraintsChangeText(op.current_constraints, op.proposed_constraints) });
            }
            break;
        case "delete_node":
            if (op.current) {
                blocks.push(...nodeSnapshotBlocks("Current", op.current));
            }
            break;
        case "set_relationship":
            if (op.relationship) {
                blocks.push(...relationshipBlocks(op.relationship, op.current_relationship, nm));
            }
            break;
        case "delete_relationship":
            if (op.relationship) {
                blocks.push(...relationshipBlocks(op.relationship, op.current_relationship, nm));
            }
            if (op.removes_nodes?.length) {
                blocks.push({ kind: "heading", text: "Also removes" });
                for (const n of op.removes_nodes) blocks.push({ kind: "line", text: `${n.name || n.id} (${n.id})` });
            }
            break;
        default:
            blocks.push({ kind: "line", text: `${op.op} ${nm(op.node_id)}` });
    }

    return blocks;
}

export function detailCanAccept(op: OpDiff): boolean {
    const missing = unconfirmable(op.stale_reason);
    return op.status === "pending" && !op.error && !missing;
}

export function detailNeedsConfirm(op: OpDiff): boolean {
    const missing = unconfirmable(op.stale_reason);
    return op.stale && !missing && op.status === "pending" && !op.error;
}

function staleMessage(op: OpDiff): string {
    if (op.stale_reason === missingNode) {
        return "Stale: the node is missing on disk. Reject this operation; it cannot be applied.";
    }
    if (op.stale_reason === noDependents) {
        return "Stale: the node now has dependents. Reject this operation; it cannot be applied.";
    }
    if (op.stale_reason === "base_hash mismatch") {
        return "Stale: the node changed after this was staged. Accept applies the proposed version over your newer edit.";
    }
    return `Stale: ${op.stale_reason ?? "unknown reason"}`;
}

function summarizeOp(op: OpDiff, names?: ReadonlyMap<string, string>): string {
    const nm = (id: string) => names?.get(id) ?? id;
    if (op.op === "create_node") {
        return `Create ${op.proposed?.name ?? op.node_id}`;
    }
    if (op.op === "delete_node") {
        return `Delete ${nm(op.node_id)}`;
    }
    if (op.op === "update_node") {
        const fields = op.fields?.length ? ` (${op.fields.join(", ")})` : "";
        return `Update ${nm(op.node_id)}${fields}`;
    }
    if (op.op === "set_constraints") return "Update project rules";
    if (op.op === "set_relationship" && op.relationship) {
        return `Set relationship · ${nm(op.relationship.from)} → ${nm(op.relationship.to)}`;
    }
    if (op.op === "delete_relationship" && op.relationship) {
        return `Remove connection · ${nm(op.relationship.from)} → ${nm(op.relationship.to)}`;
    }
    return `${op.op} ${nm(op.node_id)}`;
}

function constraintLineText(line: ConstraintLine): string {
    const verb = line.kind === "added" ? "Added" : line.kind === "removed" ? "Removed" : "Changed";
    return line.was ? `${verb}: ${line.text} (was ${line.was})` : `${verb}: ${line.text}`;
}

// One list block per contract list that differs, so the actual items are reviewable.
function contractListBlocks(before: NodeContract | undefined, after: NodeContract | undefined): DetailBlock[] {
    const b = normalizeContract(before);
    const a = normalizeContract(after);
    const blocks: DetailBlock[] = [];
    for (const { key, label } of CONTRACT_LISTS) {
        const x = b[key] ?? [];
        const y = a[key] ?? [];
        if (JSON.stringify(x) !== JSON.stringify(y)) blocks.push({ kind: "list", label, before: x, after: y });
    }
    return blocks;
}

function nodeSnapshotBlocks(title: string, node: NodeSnapshot, nm: (id: string) => string = (id) => id): DetailBlock[] {
    const extras: DetailBlock[] = [];
    if (normalizeDependsOn(node.depends_on).length) {
        extras.push({ kind: "pair", label: "Depends on", before: "—", after: dependsText(node.depends_on, nm) });
    }
    if (!contractIsEmpty(node.contract)) {
        extras.push({ kind: "pair", label: "Contract", before: "—", after: contractSummary(node.contract) });
        const payloads = normalizeContract(node.contract).payloads;
        if (payloads) extras.push({ kind: "prose", label: "Contract payloads", before: "", after: payloads });
        extras.push(...contractListBlocks(undefined, node.contract));
    }
    return [
        { kind: "heading", text: title },
        { kind: "pair", label: "name", before: "—", after: node.name },
        { kind: "pair", label: "type", before: "—", after: node.type },
        ...(node.icon
            ? [{ kind: "pair" as const, label: "icon", before: "—", after: node.icon }]
            : []),
        ...(node.color
            ? [{ kind: "pair" as const, label: "color", before: "—", after: node.color }]
            : []),
        { kind: "pair", label: "status", before: "—", after: node.status },
        {
            kind: "pair",
            label: "parent",
            before: "—",
            after: node.parent_id ? node.parent_id : "(root)",
        },
        {
            kind: "pair",
            label: "implementation",
            before: "—",
            after: node.implementation.join(", ") || "(empty)",
        },
        {
            kind: "pair",
            label: "scope",
            before: "—",
            after: node.scope.join(", ") || "(empty)",
        },
        {
            kind: "pair",
            label: "protected",
            before: "—",
            after: node.protected ? "yes" : "no",
        },
        { kind: "prose", label: "Spec", before: "", after: node.markdown || "(empty)" },
        ...extras,
    ];
}

function relationshipBlocks(proposed: RelSnapshot, current: RelSnapshot | null, nm: (id: string) => string = (id) => id): DetailBlock[] {
    const blocks: DetailBlock[] = [
        { kind: "heading", text: "Relationship", level: 4 },
        { kind: "line", text: `${nm(proposed.from)} → ${nm(proposed.to)}` },
    ];
    if (proposed.id) {
        blocks.push({ kind: "line", text: `Id: ${proposed.id}` });
    }
    if (current) {
        blocks.push({
            kind: "pair",
            label: "label",
            before: current.label || "(empty)",
            after: proposed.label || "(empty)",
        });
        blocks.push({
            kind: "pair",
            label: "kind",
            before: current.kind || "(empty)",
            after: proposed.kind || "(empty)",
        });
    } else {
        blocks.push({ kind: "pair", label: "label", before: "—", after: proposed.label || "(empty)" });
        blocks.push({ kind: "pair", label: "kind", before: "—", after: proposed.kind || "(empty)" });
    }
    const was = edgeContractLine(current?.contract);
    const now = edgeContractLine(proposed.contract);
    const cleared = proposed.contract_cleared && !now;
    if (was || now || cleared) {
        const after = now || (cleared ? (was ? `Contract cleared (was: ${was})` : "Contract cleared") : "(unchanged)");
        blocks.push({ kind: "pair", label: "contract", before: was || (current ? "(none)" : "—"), after });
        if (proposed.contract?.text) blocks.push({ kind: "prose", label: "Contract text", before: current?.contract?.text ?? "", after: proposed.contract.text });
    }
    return blocks;
}

function fieldText(node: NodeSnapshot | null, field: string): string {
    if (!node) return "—";
    switch (field) {
        case "name":
            return node.name;
        case "type":
            return node.type;
        case "status":
            return node.status;
        case "parent_id":
            return node.parent_id || "(root)";
        case "implementation":
            return node.implementation.join(", ") || "(empty)";
        case "scope":
            return node.scope.join(", ") || "(empty)";
        case "protected":
            return node.protected ? "protected" : "not protected";
        case "icon":
            return node.icon || "(default)";
        case "color":
            return node.color || "(from type)";
        case "depends_on":
            return dependsText(node.depends_on);
        case "contract":
            return contractSummary(node.contract);
        default:
            return field;
    }
}

// ---- Visual detail view-models ------------------------------------------------------------

// A node as the static preview draws it. `known` is false when the node is not in the proposal's
// snapshots (an endpoint that already exists on the canvas), so only its id is available.
export type NodePreview = {
    id: string;
    name: string;
    type: string;
    status: string;
    statusText: string;
    icon?: string;
    color: ColorToken;
    protected: boolean;
    parentId: string;
    // Name of the parent ("" for the root).
    parentName: string;
    known: boolean;
    // Preview-visible fields that differ from the other side of an update.
    changed: PreviewField[];
};

export type PreviewField = "name" | "type" | "status" | "icon" | "color" | "parent";

export type FieldChange = { field: string; label: string; before: string; after: string };

export type EdgePreview = { label: string; kind: string; contract?: string };

export type DetailView =
    | { kind: "create"; node: NodePreview; description: string; notes: string[] }
    | { kind: "rules"; lines: ConstraintLine[]; summary: string; description: string }
    | { kind: "connect"; from: NodePreview; to: NodePreview; edge: EdgePreview; previous: EdgePreview | null; description: string }
    | { kind: "update"; before: NodePreview; after: NodePreview; changes: FieldChange[]; specSummary: string; description: string }
    | { kind: "remove-node"; node: NodePreview; description: string }
    | { kind: "remove-connection"; from: NodePreview; to: NodePreview; edge: EdgePreview; interior: NodePreview[]; description: string }
    | { kind: "other"; description: string };

// id -> name for every node `proposalDetailView` can name: proposal snapshots over fetched ones.
export function detailNames(diff: ProposalDiff, fetched?: ReadonlyMap<string, NodeSnapshot>): Map<string, string> {
    return new Map([...mergedLookup(diff, fetched)].map(([id, snap]) => [id, snap.name || id]));
}

// Names for every node the proposal mentions, from the snapshots the API provides.
export function snapshotLookup(diff: ProposalDiff): Map<string, NodeSnapshot> {
    const found = new Map<string, NodeSnapshot>();
    for (const op of diff.operations) {
        // Current first so a later proposed snapshot of the same node wins for new names.
        for (const snap of [op.current, op.proposed]) if (snap) found.set(snap.id || op.node_id, snap);
    }
    return found;
}

function preview(snap: NodeSnapshot | null | undefined, id: string, lookup: Map<string, NodeSnapshot>): NodePreview {
    const parentId = snap?.parent_id ?? "";
    const parentName = parentId ? (lookup.get(parentId)?.name ?? parentId) : "";
    if (!snap) {
        return {
            id,
            name: id,
            type: "",
            status: "",
            statusText: "",
            color: resolveNodeColor("", undefined),
            protected: false,
            parentId: "",
            parentName: "",
            known: false,
            changed: [],
        };
    }
    return {
        id: snap.id || id,
        name: snap.name || id,
        type: snap.type,
        status: snap.status,
        statusText: snap.status ? statusLabel(snap.status) : "",
        icon: snap.icon || undefined,
        color: resolveNodeColor(snap.type, snap.color),
        protected: snap.protected,
        parentId,
        parentName,
        known: true,
        changed: [],
    };
}

// Line-level summary of a spec change, e.g. "Spec: 3 lines added, 1 removed".
export function specDiffSummary(before: string, after: string): string {
    if (before === after) return "Spec unchanged";
    const lines = (text: string) => (text.trim() === "" ? [] : text.split("\n"));
    const count = (list: string[]) => {
        const map = new Map<string, number>();
        for (const line of list) map.set(line, (map.get(line) ?? 0) + 1);
        return map;
    };
    const b = count(lines(before));
    const a = count(lines(after));
    let added = 0;
    let removed = 0;
    for (const [line, n] of a) added += Math.max(0, n - (b.get(line) ?? 0));
    for (const [line, n] of b) removed += Math.max(0, n - (a.get(line) ?? 0));
    const part = (n: number, word: string) => `${n} ${n === 1 ? "line" : "lines"} ${word}`;
    if (added && removed) return `Spec: ${part(added, "added")}, ${removed} removed`;
    if (added) return `Spec: ${part(added, "added")}`;
    if (removed) return `Spec: ${part(removed, "removed")}`;
    return "Spec reordered";
}

function insideText(node: NodePreview): string {
    return node.parentId ? `inside ${node.parentName}` : "at the top level";
}

function edgeText(from: NodePreview, to: NodePreview, edge: EdgePreview): string {
    const label = edge.label ? ` labelled ${edge.label}` : "";
    const kind = edge.kind ? `, ${edge.kind}` : "";
    return `from ${from.name} to ${to.name}${label}${kind}`;
}

const PREVIEW_FIELDS: Record<string, PreviewField> = {
    name: "name",
    type: "type",
    status: "status",
    icon: "icon",
    color: "color",
    parent_id: "parent",
};

// Proposal snapshots merged over `fetched` ones (existing nodes outside the proposal); the
// proposal's own snapshots win because they describe the state this operation sees.
export function mergedLookup(diff: ProposalDiff, fetched?: ReadonlyMap<string, NodeSnapshot>): Map<string, NodeSnapshot> {
    const merged = new Map<string, NodeSnapshot>(fetched ?? []);
    for (const [id, snap] of snapshotLookup(diff)) merged.set(id, snap);
    return merged;
}

// Ids of nodes this operation's visual names (endpoints, parents) that no known snapshot covers.
export function unresolvedNodeIds(diff: ProposalDiff, op: OpDiff, fetched?: ReadonlyMap<string, NodeSnapshot>): string[] {
    const lookup = mergedLookup(diff, fetched);
    const rel = op.op === "set_relationship" ? op.relationship : op.op === "delete_relationship" ? (op.relationship ?? op.current_relationship) : null;
    const shown = new Set<string>();
    if (rel) {
        shown.add(rel.from);
        shown.add(rel.to);
    }
    for (const n of op.removes_nodes ?? []) shown.add(n.id);
    for (const snap of [op.current, op.proposed]) for (const id of snap?.depends_on ?? []) shown.add(id);
    const missing = new Set<string>();
    const snaps: (NodeSnapshot | null | undefined)[] = [op.current, op.proposed];
    for (const id of shown) {
        const snap = lookup.get(id);
        if (snap) snaps.push(snap);
        else missing.add(id);
    }
    for (const snap of snaps) {
        if (snap?.parent_id && !lookup.has(snap.parent_id)) missing.add(snap.parent_id);
    }
    return [...missing];
}

export function proposalDetailView(diff: ProposalDiff, op: OpDiff, fetched?: ReadonlyMap<string, NodeSnapshot>): DetailView {
    const lookup = mergedLookup(diff, fetched);
    switch (op.op) {
        case "create_node": {
            const node = preview(op.proposed, op.node_id, lookup);
            const nm = (id: string) => lookup.get(id)?.name ?? id;
            const notes = [
                normalizeDependsOn(op.proposed?.depends_on).length ? `Depends on ${dependsText(op.proposed?.depends_on, nm)}` : "",
                contractIsEmpty(op.proposed?.contract) ? "" : `Contract: ${contractSummary(op.proposed?.contract)}`,
            ].filter(Boolean);
            return {
                kind: "create",
                notes,
                node,
                description: `New node ${node.name}, type ${node.type || "unspecified"}, status ${node.statusText || "unspecified"}, ${insideText(node)}.`,
            };
        }
        case "update_node": {
            const before = preview(op.current, op.node_id, lookup);
            const after = preview(op.proposed, op.node_id, lookup);
            const fields = op.fields ?? [];
            const changed = fields.map((f) => PREVIEW_FIELDS[f]).filter((f): f is PreviewField => !!f);
            before.changed = changed;
            after.changed = changed;
            const changes: FieldChange[] = fields
                .filter((f) => f !== "spec")
                .map((field) => ({
                    field,
                    label: field === "parent_id" ? "parent" : field === "depends_on" ? "depends on" : field,
                    before: fieldValue(op.current, field, lookup),
                    after: fieldValue(op.proposed, field, lookup),
                }));
            const specSummary = fields.includes("spec")
                ? specDiffSummary(op.current?.markdown ?? "", op.proposed?.markdown ?? "")
                : "";
            const what = fields.length ? fields.map((f) => (f === "parent_id" ? "parent" : f)).join(", ") : "nothing";
            return {
                kind: "update",
                before,
                after,
                changes,
                specSummary,
                description: `Update ${before.name}: changes ${what}.`,
            };
        }
        case "delete_node": {
            const node = preview(op.current, op.node_id, lookup);
            return { kind: "remove-node", node, description: `Removed node ${node.name}, ${insideText(node)}.` };
        }
        case "set_relationship": {
            if (!op.relationship) break;
            const from = preview(lookup.get(op.relationship.from), op.relationship.from, lookup);
            const to = preview(lookup.get(op.relationship.to), op.relationship.to, lookup);
            const edge = { label: op.relationship.label, kind: op.relationship.kind, contract: edgeContractLine(op.relationship.contract) };
            const previous = op.current_relationship
                ? { label: op.current_relationship.label, kind: op.current_relationship.kind, contract: edgeContractLine(op.current_relationship.contract) }
                : null;
            return {
                kind: "connect",
                from,
                to,
                edge,
                previous,
                description: `${previous ? "Changed connection" : "New connection"} ${edgeText(from, to, edge)}.`,
            };
        }
        case "delete_relationship": {
            const rel = op.relationship ?? op.current_relationship;
            if (!rel) break;
            const from = preview(lookup.get(rel.from), rel.from, lookup);
            const to = preview(lookup.get(rel.to), rel.to, lookup);
            const edge = { label: rel.label, kind: rel.kind };
            const interior = (op.removes_nodes ?? []).map((n) => {
                const snap = lookup.get(n.id);
                const view = preview(snap, n.id, lookup);
                return snap ? view : { ...view, name: n.name || n.id };
            });
            const also = interior.length ? ` Also removes ${interior.map((n) => n.name).join(", ")}.` : "";
            return {
                kind: "remove-connection",
                from,
                to,
                edge,
                interior,
                description: `Removed connection ${edgeText(from, to, edge)}.${also}`,
            };
        }
        case "set_constraints": {
            const lines = constraintsDiff(op.current_constraints, op.proposed_constraints);
            const summary = constraintsChangeText(op.current_constraints, op.proposed_constraints);
            return { kind: "rules", lines, summary, description: `${summary}.` };
        }
    }
    return { kind: "other", description: `${op.op} ${op.node_id}` };
}

function fieldValue(node: NodeSnapshot | null, field: string, lookup: Map<string, NodeSnapshot>): string {
    if (field === "parent_id") {
        if (!node) return "—";
        return node.parent_id ? (lookup.get(node.parent_id)?.name ?? node.parent_id) : "(root)";
    }
    if (field === "depends_on") return dependsText(node?.depends_on, (id) => lookup.get(id)?.name ?? id);
    return fieldText(node, field);
}
