export const uid = (p = "node") =>
    `${p}_${Math.random().toString(36).slice(2, 9)}`;

import type { Flow } from "./types";
import { PortMatrix } from "./ports";

export function validateEdges(flow: Flow): { ok: boolean; errors: string[] } {
    const errors: string[] = [];
    const nodeById = Object.fromEntries(flow.nodes.map((n) => [n.id, n]));
    const onEnd = flow.nodes.filter(
        (n) => n.kind === "OnEnd" || n.special === "OnEnd"
    );

    for (const e of flow.edges) {
        const src = nodeById[e.from.nodeId];
        const dst = nodeById[e.to.nodeId];
        if (!src || !dst) {
            errors.push(`Dangling edge ${e.id}`);
            continue;
        }
        const srcType = e.from.port ?? src.out[0];
        const dstType = e.to.port ?? dst.in[0];
        const allowed = PortMatrix[srcType] ?? [];
        if (!allowed.includes(dstType)) {
            errors.push(`Edge ${e.id}: ${srcType} → ${dstType} not allowed`);
        }
        if (src.kind === "OnEnd")
            errors.push(`Edge ${e.id}: OnEnd cannot have outputs`);
    }

    // Ensure Ingest exists and has no inbound edges
    const ingest = flow.nodes.find(
        (n) => n.special === "Ingest" || n.kind === "Ingest"
    );
    if (!ingest) errors.push("Missing special Ingest node");
    else {
        const inboundToIngest = flow.edges.some(
            (e) => e.to.nodeId === ingest.id
        );
        if (inboundToIngest) errors.push("Ingest cannot have inbound edges");
    }

    // OnEnd cannot have outbound edges
    for (const end of onEnd) {
        const out = flow.edges.some((e) => e.from.nodeId === end.id);
        if (out) errors.push("OnEnd cannot have outbound edges");
    }

    return { ok: errors.length === 0, errors };
}
