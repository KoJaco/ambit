export const rootLayoutKey = "_root";

export function layoutCacheKey(nodeId: string | undefined): string {
    return nodeId ? nodeId : rootLayoutKey;
}

export type Summary = {
    id: string;
    name: string;
    type: string;
    status: string;
    protected: boolean;
};

export type Crossing = {
    node_id: string;
    direction: string;
    label: string;
    kind: string;
    other_id: string;
};

export type Relationship = {
    from: string;
    to: string;
    label: string;
    kind: string;
};

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
};

export type NodeDetail = {
    id: string;
    name: string;
    type: string;
    status: string;
    parent_id?: string;
    implementation: string[];
    scope: string[];
    protected: boolean;
    markdown: string;
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
};

export type Position = { id: string; x: number; y: number };

export type Layout = { positions: Position[] };

async function send<T>(path: string, init?: RequestInit): Promise<T> {
    const res = await fetch(path, init);
    if (!res.ok) {
        const body = (await res.json().catch(() => null)) as { error?: string } | null;
        throw new Error(body?.error ?? `request failed (${res.status})`);
    }
    if (res.status === 204) return undefined as T;
    return res.json() as Promise<T>;
}

export function getLevel(nodeId?: string, signal?: AbortSignal): Promise<Level> {
    const path = nodeId ? `/levels/${encodeURIComponent(nodeId)}` : "/levels";
    return send<Level>(path, { signal });
}

export function getNode(id: string, signal?: AbortSignal): Promise<NodeDetail> {
    return send<NodeDetail>(`/nodes/${encodeURIComponent(id)}`, { signal });
}

export function getIntegrity(): Promise<{ warnings: Warning[] }> {
    return send(`/integrity`);
}

export function createNode(input: {
    name: string;
    type: string;
    parent_id?: string;
    markdown?: string;
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

export function setRelationship(input: Relationship): Promise<Relationship> {
    return send<Relationship>("/relationships", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(input),
    });
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
    implementation: string[];
    scope: string[];
    protected: boolean;
    markdown: string;
};

export type RelSnapshot = {
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

export type EventHandlers = {
    onModelChanged: (nodeIds: string[]) => void;
    onIntegrityChanged: () => void;
    onProposalsChanged: () => void;
};

// Unknown event names are ignored. proposals-changed refreshes the review surface only.
export function dispatchEvent(name: string, data: string, handlers: EventHandlers) {
    if (name === "model-changed") {
        const parsed = JSON.parse(data) as { node_ids?: string[] };
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
