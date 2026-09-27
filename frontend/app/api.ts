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

export type EventHandlers = {
    onModelChanged: (nodeIds: string[]) => void;
    onIntegrityChanged: () => void;
};

// Unknown event names are ignored so a later proposals-changed event is a no-op here.
export function dispatchEvent(name: string, data: string, handlers: EventHandlers) {
    if (name === "model-changed") {
        const parsed = JSON.parse(data) as { node_ids?: string[] };
        handlers.onModelChanged(parsed.node_ids ?? []);
        return;
    }
    if (name === "integrity-changed") {
        handlers.onIntegrityChanged();
    }
}

export function subscribeEvents(handlers: EventHandlers): () => void {
    const source = new EventSource("/events");
    const onModel = (event: MessageEvent) => dispatchEvent("model-changed", event.data, handlers);
    const onIntegrity = (event: MessageEvent) => dispatchEvent("integrity-changed", event.data, handlers);
    source.addEventListener("model-changed", onModel);
    source.addEventListener("integrity-changed", onIntegrity);
    return () => {
        source.removeEventListener("model-changed", onModel);
        source.removeEventListener("integrity-changed", onIntegrity);
        source.close();
    };
}
