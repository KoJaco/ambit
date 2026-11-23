import { describe, expect, it, vi } from "vitest";
import type { NodeSnapshot } from "./api";
import { createSnapshotStore } from "./use-node-snapshots";

const snap = (id: string, parent = ""): NodeSnapshot => ({ id, name: `name-${id}`, parent_id: parent }) as NodeSnapshot;

function deferred() {
    let resolve!: () => void;
    const promise = new Promise<void>((r) => (resolve = r));
    return { promise, resolve };
}

describe("createSnapshotStore", () => {
    it("dedupes in-flight requests", async () => {
        const gate = deferred();
        const get = vi.fn(async (id: string) => (await gate.promise, snap(id)));
        const store = createSnapshotStore(get);
        const a = store.ensure(["a"]);
        const b = store.ensure(["a", "b"]);
        gate.resolve();
        await Promise.all([a, b]);
        expect(get).toHaveBeenCalledTimes(2);
        expect([...store.known().keys()].sort()).toEqual(["a", "b"]);
        expect(store.inflightCount()).toBe(0);
    });

    it("does not restart pending fetches when another finishes", async () => {
        const gates = { a: deferred(), b: deferred() };
        const get = vi.fn(async (id: string) => (await gates[id as "a" | "b"].promise, snap(id)));
        const store = createSnapshotStore(get);
        const first = store.ensure(["a", "b"]);
        gates.a.resolve();
        await Promise.resolve();
        await Promise.resolve();
        void store.ensure(store.needed(["a", "b"])); // the wanted key changed mid-flight
        expect(get).toHaveBeenCalledTimes(2);
        gates.b.resolve();
        await first;
        expect(get).toHaveBeenCalledTimes(2);
        expect(store.known().size).toBe(2);
    });

    it("caches a failed fetch as null", async () => {
        const get = vi.fn(async () => {
            throw new Error("nope");
        });
        const store = createSnapshotStore(get);
        await store.ensure(["x"]);
        expect(store.known().size).toBe(0);
        expect(store.isFresh("x")).toBe(true);
        await store.ensure(["x"]);
        expect(get).toHaveBeenCalledTimes(1);
    });

    it("chases parents through needed()", async () => {
        const nodes: Record<string, NodeSnapshot> = { child: snap("child", "parent"), parent: snap("parent") };
        const store = createSnapshotStore(async (id) => nodes[id]);
        const resolve = (known: ReadonlyMap<string, NodeSnapshot>) => {
            const c = known.get("child");
            return !c ? ["child"] : c.parent_id && !known.has(c.parent_id) ? [c.parent_id] : [];
        };
        for (let i = 0; i < 4; i++) await store.ensure(store.needed(resolve(store.known())));
        expect([...store.known().keys()].sort()).toEqual(["child", "parent"]);
    });

    it("keeps expired entries visible and refetches them", async () => {
        let t = 0;
        let version = 1;
        const get = vi.fn(async (id: string) => ({ ...snap(id), name: `v${version}` }));
        const store = createSnapshotStore(get, () => t, 100);
        await store.ensure(["a"]);
        t = 500;
        expect(store.known().get("a")?.name).toBe("v1");
        expect(store.needed([])).toEqual(["a"]);
        version = 2;
        const gate = deferred();
        get.mockImplementationOnce(async (id: string) => (await gate.promise, { ...snap(id), name: "v2" }));
        const refresh = store.ensure(store.needed([]));
        expect(store.known().get("a")?.name).toBe("v1");
        gate.resolve();
        await refresh;
        expect(store.known().get("a")?.name).toBe("v2");
        expect(store.needed([])).toEqual([]);
    });

    it("keeps the old node when a refresh fails", async () => {
        let t = 0;
        const get = vi.fn<(id: string) => Promise<NodeSnapshot>>().mockResolvedValueOnce(snap("a")).mockRejectedValueOnce(new Error("x"));
        const store = createSnapshotStore(get, () => t, 100);
        await store.ensure(["a"]);
        t = 500;
        await store.ensure(store.needed([]));
        expect(store.known().get("a")?.name).toBe("name-a");
    });
});
