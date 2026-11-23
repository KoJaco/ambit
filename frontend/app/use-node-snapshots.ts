import { useEffect, useRef, useState } from "react";
import { getNode, type NodeSnapshot } from "./api";

const TTL_MS = 30_000;

type GetNode = (id: string) => Promise<NodeSnapshot>;
type Entry = { at: number; snap: NodeSnapshot | null };

// Small cache of existing nodes by id. Requests in flight are shared and never aborted, so one
// finishing (or the wanted set changing) cannot restart the others. Entries past the TTL stay
// visible and are refetched in the background (stale-while-revalidate).
export function createSnapshotStore(fetchNode: GetNode, now: () => number = Date.now, ttlMs = TTL_MS) {
    const cache = new Map<string, Entry>();
    const inflight = new Map<string, Promise<void>>();

    const isFresh = (id: string) => {
        const hit = cache.get(id);
        return !!hit && now() - hit.at <= ttlMs;
    };

    // Every node resolved so far, expired or not.
    function known(): Map<string, NodeSnapshot> {
        const out = new Map<string, NodeSnapshot>();
        for (const [id, hit] of cache) if (hit.snap) out.set(id, hit.snap);
        return out;
    }

    // Ids worth fetching: the wanted ones plus any cached node that has expired.
    function needed(wanted: string[]): string[] {
        const ids = new Set(wanted);
        for (const [id, hit] of cache) if (hit.snap) ids.add(id);
        return [...ids].filter((id) => !isFresh(id));
    }

    function load(id: string): Promise<void> {
        let pending = inflight.get(id);
        if (!pending) {
            pending = fetchNode(id)
                .then(
                    (snap) => void cache.set(id, { at: now(), snap }),
                    // A failed refresh keeps the node we already had.
                    () => void cache.set(id, { at: now(), snap: cache.get(id)?.snap ?? null }),
                )
                .finally(() => void inflight.delete(id));
            inflight.set(id, pending);
        }
        return pending;
    }

    // Starts whatever is not fresh or in flight; resolves when all of `ids` have settled.
    function ensure(ids: string[]): Promise<void> {
        return Promise.all(ids.filter((id) => !isFresh(id)).map(load)).then(() => undefined);
    }

    return { known, needed, ensure, isFresh, inflightCount: () => inflight.size };
}

const store = createSnapshotStore(getNode);

// Fetches existing nodes by id (small module cache) so previews can show real names.
// Returns only the nodes resolved so far; failures resolve to nothing so callers fall back to ids.
// `resolve` is called with what is known so far and returns the ids still needed, which lets
// callers chase a fetched node's parent.
export function useNodeSnapshots(resolve: (known: ReadonlyMap<string, NodeSnapshot>) => string[]): ReadonlyMap<string, NodeSnapshot> {
    const [, bump] = useState(0);
    const alive = useRef(true);
    useEffect(() => {
        alive.current = true;
        return () => {
            alive.current = false;
        };
    }, []);

    const known = store.known();
    const key = store.needed(resolve(known)).join("\n");

    useEffect(() => {
        if (!key) return;
        void store.ensure(key.split("\n")).then(() => {
            if (alive.current) bump((n) => n + 1);
        });
    }, [key]);

    return known;
}
