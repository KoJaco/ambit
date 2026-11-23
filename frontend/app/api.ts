import {
    expandedKey,
    sanitizeGroupSizes,
    sanitizeViewModes,
    type GroupSizes,
    type ViewDefaults,
    type ViewModes,
} from "./level-view";
import { markOwnWrite } from "./history";

export const rootLayoutKey = "_root";

// The expanded-set hash is appended only when a group is open, so flat levels keep their old key.
// The separator cannot appear in a node id (ids are lowercase slugs).
export function layoutCacheKey(opts: {
    nodeId?: string;
    relationshipId?: string;
    expanded?: Iterable<string>;
}): string {
    if (opts.relationshipId) return `_rel_${opts.relationshipId}`;
    const base = opts.nodeId ? opts.nodeId : rootLayoutKey;
    const hash = expandedKey(opts.expanded ?? []);
    return hash ? `${base}~${hash}` : base;
}

// View modes (group or summary per node) are per level, not per expanded set, so they use the base key.
const viewModesStoragePrefix = "ambit:view-modes:";

export function loadViewModes(baseKey: string): ViewModes {
    try {
        const raw = window.localStorage.getItem(viewModesStoragePrefix + baseKey);
        return raw ? sanitizeViewModes(JSON.parse(raw)) : {};
    } catch {
        return {};
    }
}

export function saveViewModes(baseKey: string, modes: ViewModes): void {
    try {
        window.localStorage.setItem(viewModesStoragePrefix + baseKey, JSON.stringify(modes));
    } catch {
        // Private mode or quota: view modes are a convenience, the level still renders.
    }
}

// Sizes the architect gave open groups by resizing. The typed server layout carries positions only,
// so they live here, keyed by the layout cache key (level plus expanded-set hash).
const groupSizesStoragePrefix = "ambit:group-sizes:";

export function loadGroupSizes(cacheKey: string): GroupSizes {
    try {
        const raw = window.localStorage.getItem(groupSizesStoragePrefix + cacheKey);
        return raw ? sanitizeGroupSizes(JSON.parse(raw)) : {};
    } catch {
        return {};
    }
}

export function saveGroupSizes(cacheKey: string, sizes: GroupSizes): void {
    try {
        window.localStorage.setItem(groupSizesStoragePrefix + cacheKey, JSON.stringify(sizes));
    } catch {
        // Without it the group simply returns to its automatic size.
    }
}

// Membership signature of a cached layout (see layoutSignature). The server Layout schema is
// typed and cannot carry it, so it lives next to the view modes, keyed by the cache key.
const layoutSigStoragePrefix = "ambit:layout-sig:";

export function loadLayoutSig(cacheKey: string): string | null {
    try {
        return window.localStorage.getItem(layoutSigStoragePrefix + cacheKey);
    } catch {
        return null;
    }
}

export function saveLayoutSig(cacheKey: string, sig: string): void {
    try {
        window.localStorage.setItem(layoutSigStoragePrefix + cacheKey, sig);
    } catch {
        // Without it the layout is simply recomputed next time.
    }
}

export type SchemaColumn = { title: string; type: string };

export type Summary = {
    id: string;
    name: string;
    type: string;
    status: string;
    protected: boolean;
    icon?: string;
    color?: string;
    renderer?: string;
    schema?: SchemaColumn[];
    child_count?: number;
    group_id?: string;
    expanded?: boolean;
    internal_rel_count?: number;
    type_mix?: TypeMixEntry[];
    assigned?: boolean;
};

export type TypeMixEntry = { type: string; count: number };

export type Crossing = {
    node_id: string;
    direction: string;
    label: string;
    kind: string;
    other_id: string;
    relationship_id?: string;
    drillable?: boolean;
};

export type Relationship = {
    id: string;
    from: string;
    to: string;
    label: string;
    kind: string;
    drillable?: boolean;
    aggregated?: boolean;
    members?: string[];
    kinds?: string[];
};

// Aggregated edges carry a synthetic id; only a single-member one can be drilled, via that member.
export function relationshipDrillId(rel: Relationship): string | undefined {
    if (!rel.drillable) return undefined;
    return rel.aggregated ? rel.members?.[0] : rel.id;
}

export type Warning = {
    severity: string;
    path: string;
    message: string;
};

export type Level = {
    node: Summary | null;
    children: Summary[];
    relationships: Relationship[];
    crossings: Crossing[];
    warnings: Warning[];
    view_defaults?: ViewDefaults;
};

export type RelationshipLevel = {
    relationship: Relationship;
    children: Summary[];
    context: Summary[];
    relationships: Relationship[];
    crossings: Crossing[];
    warnings: Warning[];
};

export type NodeDetail = {
    id: string;
    name: string;
    type: string;
    status: string;
    parent_id?: string;
    relationship_id?: string;
    implementation: string[];
    scope: string[];
    protected: boolean;
    markdown: string;
    icon?: string;
    color?: string;
    renderer?: string;
    schema?: SchemaColumn[];
    assigned?: boolean;
};

export type NodePatch = {
    name?: string;
    type?: string;
    status?: string;
    parent_id?: string;
    implementation?: string[];
    scope?: string[];
    protected?: boolean;
    markdown?: string;
    icon?: string;
    color?: string;
    renderer?: string;
    schema?: SchemaColumn[];
};

export type Position = { id: string; x: number; y: number };

export type NodePorts = { left: number; right: number };
export type EdgeAttachment = { source: string; target: string };

export type Layout = {
    positions: Position[];
    ports?: Record<string, NodePorts>;
    attachments?: Record<string, EdgeAttachment>;
};

async function send<T>(path: string, init?: RequestInit): Promise<T> {
    if (init?.method && init.method !== "GET") markOwnWrite();
    const res = await fetch(path, init);
    if (!res.ok) {
        const body = (await res.json().catch(() => null)) as { error?: string } | null;
        throw new Error(body?.error ?? `request failed (${res.status})`);
    }
    if (res.status === 204) return undefined as T;
    return res.json() as Promise<T>;
}

export function getLevel(nodeId?: string, signal?: AbortSignal, expand: string[] = []): Promise<Level> {
    const base = nodeId ? `/levels/${encodeURIComponent(nodeId)}` : "/levels";
    const query = expand.length > 0 ? `?expand=${expand.map(encodeURIComponent).join(",")}` : "";
    return send<Level>(base + query, { signal });
}

export function getNode(id: string, signal?: AbortSignal): Promise<NodeDetail> {
    return send<NodeDetail>(`/nodes/${encodeURIComponent(id)}`, { signal });
}

export type IconInfo = { id: string; label: string };

export function getIcons(signal?: AbortSignal): Promise<{ icons: IconInfo[] }> {
    return send<{ icons: IconInfo[] }>("/icons", { signal });
}

export function getIntegrity(): Promise<{ warnings: Warning[] }> {
    return send(`/integrity`);
}

export function getRelationshipLevel(relationshipId: string, signal?: AbortSignal): Promise<RelationshipLevel> {
    return send<RelationshipLevel>(`/levels/relationships/${encodeURIComponent(relationshipId)}`, { signal });
}

export function createNode(input: {
    name: string;
    type: string;
    parent_id?: string;
    relationship_id?: string;
    status?: string;
    implementation?: string[];
    scope?: string[];
    protected?: boolean;
    markdown?: string;
    icon?: string;
    color?: string;
    renderer?: string;
    schema?: SchemaColumn[];
}): Promise<NodeDetail> {
    return send<NodeDetail>("/nodes", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(input),
    });
}

export function updateNode(id: string, patch: NodePatch): Promise<NodeDetail> {
    return send<NodeDetail>(`/nodes/${encodeURIComponent(id)}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(patch),
    });
}

export function deleteNode(id: string): Promise<void> {
    return send<void>(`/nodes/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export function setRelationship(input: Partial<Relationship> & { from: string; to: string }): Promise<Relationship> {
    return send<Relationship>("/relationships", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(input),
    });
}

export function deleteRelationship(id: string): Promise<void> {
    return send<void>(`/relationships/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// node_id is null when nothing is assigned. dangling means the node no longer exists.
export type Assignment = { node_id: string | null; assigned_at?: string; dangling?: boolean };

export function getAssignment(signal?: AbortSignal): Promise<Assignment> {
    return send<Assignment>("/assignment", { signal });
}

export function setAssignment(nodeId: string): Promise<void> {
    return send<void>("/assignment", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ node_id: nodeId }),
    });
}

export function clearAssignment(): Promise<void> {
    return send<void>("/assignment", { method: "DELETE" });
}

export function getLayout(key: string): Promise<Layout> {
    return send<Layout>(`/layout/${encodeURIComponent(key)}`);
}

export function putLayout(key: string, layout: Layout): Promise<Layout> {
    return send<Layout>(`/layout/${encodeURIComponent(key)}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(layout),
    });
}

export type ProposalSummary = {
    proposal_id: string;
    created_at?: string;
    source?: string;
    summary?: string;
    unreadable: boolean;
    error?: string;
    operations?: OpSummary[];
};

export type OpSummary = {
    index: number;
    op: string;
    node_id: string;
    status: string;
    stale: boolean;
    stale_reason?: string;
    fields?: string[];
};

export type NodeSnapshot = {
    id: string;
    name: string;
    type: string;
    status: string;
    parent_id?: string;
    relationship_id?: string;
    implementation: string[];
    scope: string[];
    protected: boolean;
    markdown: string;
    icon?: string;
    color?: string;
};

export type RelSnapshot = {
    id?: string;
    from: string;
    to: string;
    label: string;
    kind: string;
};

export type OpDiff = {
    index: number;
    op: string;
    node_id: string;
    status: string;
    stale: boolean;
    stale_reason?: string;
    fields?: string[];
    current: NodeSnapshot | null;
    proposed: NodeSnapshot | null;
    relationship: RelSnapshot | null;
    current_relationship: RelSnapshot | null;
    error?: string;
};

export type ProposalDiff = {
    proposal_id: string;
    created_at: string;
    source: string;
    summary?: string;
    operations: OpDiff[];
};

export type SkippedOp = {
    index: number;
    op: string;
    node_id: string;
    reason: string;
};

export type AcceptAllResult = {
    applied: number[];
    skipped: SkippedOp[];
    failed: { index: number; op: string; node_id: string; error: string }[];
};

export function listProposals(): Promise<{ proposals: ProposalSummary[] }> {
    return send("/proposals");
}

export function getProposal(id: string): Promise<ProposalDiff> {
    return send(`/proposals/${encodeURIComponent(id)}`);
}

export function acceptOperation(id: string, index: number, confirmStale = false): Promise<void> {
    return send(`/proposals/${encodeURIComponent(id)}/operations/${index}/accept`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ confirm_stale: confirmStale }),
    });
}

export function rejectOperation(id: string, index: number): Promise<void> {
    return send(`/proposals/${encodeURIComponent(id)}/operations/${index}/reject`, { method: "POST" });
}

export function acceptAll(id: string): Promise<AcceptAllResult> {
    return send(`/proposals/${encodeURIComponent(id)}/accept`, { method: "POST" });
}

export function rejectAll(id: string): Promise<void> {
    return send(`/proposals/${encodeURIComponent(id)}/reject`, { method: "POST" });
}

export function deleteProposal(id: string): Promise<void> {
    return send(`/proposals/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export type StageGroupInput = {
    node_ids: string[];
    name: string;
    type?: string;
    icon?: string;
    color?: string;
};

function stage(path: string, body: unknown): Promise<ProposalDiff> {
    return send<ProposalDiff>(path, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
    });
}

// Structure edits from the canvas never write the model: each stages a reviewed proposal (201).
// Failures (400/404/409) reject with the server's `{error}` message, see `send`.
export function stageGroup(input: StageGroupInput): Promise<ProposalDiff> {
    return stage("/proposals/group", input);
}

export function stageUngroup(nodeId: string, deleteContainer = false): Promise<ProposalDiff> {
    return stage("/proposals/ungroup", { node_id: nodeId, delete_container: deleteContainer });
}

// newParentId "" moves the node to the root.
export function stageReparent(nodeId: string, newParentId: string): Promise<ProposalDiff> {
    return stage("/proposals/reparent", { node_id: nodeId, new_parent_id: newParentId });
}

export type EventHandlers = {
    onModelChanged: (nodeIds: string[]) => void;
    onIntegrityChanged: () => void;
    onProposalsChanged: () => void;
};

// Unknown event names are ignored. proposals-changed refreshes the review surface only.
export function dispatchEvent(name: string, data: string, handlers: EventHandlers) {
    if (name === "model-changed") {
        const parsed = JSON.parse(data) as { node_ids?: string[]; relationship_ids?: string[] };
        handlers.onModelChanged(parsed.node_ids ?? []);
        return;
    }
    if (name === "integrity-changed") {
        handlers.onIntegrityChanged();
        return;
    }
    if (name === "proposals-changed") {
        handlers.onProposalsChanged();
    }
}

export function subscribeEvents(handlers: EventHandlers): () => void {
    const source = new EventSource("/events");
    const onModel = (event: MessageEvent) => dispatchEvent("model-changed", event.data, handlers);
    const onIntegrity = (event: MessageEvent) => dispatchEvent("integrity-changed", event.data, handlers);
    const onProposals = (event: MessageEvent) => dispatchEvent("proposals-changed", event.data, handlers);
    source.addEventListener("model-changed", onModel);
    source.addEventListener("integrity-changed", onIntegrity);
    source.addEventListener("proposals-changed", onProposals);
    return () => {
        source.removeEventListener("model-changed", onModel);
        source.removeEventListener("integrity-changed", onIntegrity);
        source.removeEventListener("proposals-changed", onProposals);
        source.close();
    };
}
