import { useEffect, useState } from "react";
import { getNode, type NodeSnapshot } from "./api";

const TTL_MS = 30_000;
const cache = new Map<string, { at: number; snap: NodeSnapshot | null }>();

function fresh(id: string): NodeSnapshot | null | undefined {
    const hit = cache.get(id);
    if (!hit || Date.now() - hit.at > TTL_MS) return undefined;
    return hit.snap;
}

// Fetches existing nodes by id (small module cache) so previews can show real names.
// Returns only the nodes resolved so far; failures resolve to nothing so callers fall back to ids.
// `resolve` is called with what is known so far and returns the ids still needed, which lets
// callers chase a fetched node's parent.
export function useNodeSnapshots(resolve: (known: ReadonlyMap<string, NodeSnapshot>) => string[]): ReadonlyMap<string, NodeSnapshot> {
    const [, bump] = useState(0);
    const known = new Map<string, NodeSnapshot>();
    for (const [id] of cache) {
        const snap = fresh(id);
        if (snap) known.set(id, snap);
    }
    const wanted = resolve(known).filter((id) => fresh(id) === undefined);
    const key = wanted.join("\n");

    useEffect(() => {
        if (!key) return;
        const ctl = new AbortController();
        for (const id of key.split("\n")) {
            getNode(id, ctl.signal)
                .then((snap) => cache.set(id, { at: Date.now(), snap }))
                .catch(() => {
                    if (!ctl.signal.aborted) cache.set(id, { at: Date.now(), snap: null });
                })
                .then(() => {
                    if (!ctl.signal.aborted) bump((n) => n + 1);
                });
        }
        return () => ctl.abort();
    }, [key]);

    return known;
}
