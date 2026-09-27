import { useEffect, useState } from "react";
import { Link } from "react-router";
import clsx from "clsx";
import styles from "./Sidebar.module.css";

type Summary = { id: string; name: string; type: string };

type Level = { node: Summary | null; children: Summary[] };

export function Sidebar({
    nodeId,
    refreshKey,
    onMutated,
}: {
    nodeId?: string;
    refreshKey: number;
    onMutated: () => void;
}) {
    const [collapsed, setCollapsed] = useState(false);
    const [roots, setRoots] = useState<Summary[]>([]);
    const [children, setChildren] = useState<Summary[]>([]);
    const [current, setCurrent] = useState<Summary | null>(null);
    const [name, setName] = useState("");
    const [type, setType] = useState("");
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        const ac = new AbortController();
        fetch("/levels", { signal: ac.signal })
            .then((res) => res.json() as Promise<Level>)
            .then((level) => setRoots(level.children ?? []))
            .catch(() => {});
        return () => ac.abort();
    }, [nodeId, refreshKey]);

    useEffect(() => {
        if (!nodeId) {
            setChildren([]);
            setCurrent(null);
            return;
        }
        const ac = new AbortController();
        fetch(`/levels/${encodeURIComponent(nodeId)}`, { signal: ac.signal })
            .then((res) => res.json() as Promise<Level>)
            .then((level) => {
                setCurrent(level.node);
                setChildren(level.children ?? []);
            })
            .catch(() => {});
        return () => ac.abort();
    }, [nodeId, refreshKey]);

    async function createNode() {
        setError(null);
        const res = await fetch("/nodes", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                name,
                type,
                parent_id: nodeId ?? "",
            }),
        });
        if (!res.ok) {
            const body = (await res.json().catch(() => null)) as { error?: string } | null;
            setError(body?.error ?? `create failed (${res.status})`);
            return;
        }
        setName("");
        setType("");
        onMutated();
    }

    return (
        <aside
            className={clsx(
                "absolute top-0 left-0 z-40 h-screen border-r border-foreground/20 bg-background",
                collapsed ? "w-12" : "w-64"
            )}
        >
            <button
                type="button"
                className="m-2 rounded-md border border-foreground/20 px-2 py-1 text-xs"
                onClick={() => setCollapsed((value) => !value)}
            >
                {collapsed ? ">" : "<"}
            </button>
            {collapsed ? null : (
                <div className={clsx("flex h-[calc(100%-3rem)] flex-col gap-4 px-3 pb-4 text-sm", styles.sidebarScroll)}>
                    <section>
                        <h2 className="mb-1 text-xs uppercase tracking-wide text-foreground/60">Roots</h2>
                        <NodeLinks items={roots} currentId={nodeId} />
                    </section>
                    {nodeId ? (
                        <section>
                            <h2 className="mb-1 text-xs uppercase tracking-wide text-foreground/60">
                                {current?.name ?? nodeId}
                            </h2>
                            <NodeLinks items={children} currentId={nodeId} />
                        </section>
                    ) : null}
                    <form
                        className="mt-auto flex flex-col gap-2"
                        onSubmit={(event) => {
                            event.preventDefault();
                            void createNode();
                        }}
                    >
                        <h2 className="text-xs uppercase tracking-wide text-foreground/60">New node</h2>
                        <input
                            className="rounded border border-foreground/20 bg-background px-2 py-1"
                            placeholder="Name"
                            value={name}
                            onChange={(event) => setName(event.target.value)}
                        />
                        <input
                            className="rounded border border-foreground/20 bg-background px-2 py-1"
                            placeholder="Type"
                            value={type}
                            onChange={(event) => setType(event.target.value)}
                        />
                        {error ? <p className="text-xs text-red-600">{error}</p> : null}
                        <button type="submit" className="rounded border border-foreground/30 px-2 py-1">
                            Create
                        </button>
                    </form>
                </div>
            )}
        </aside>
    );
}

function NodeLinks({ items, currentId }: { items: Summary[]; currentId?: string }) {
    if (items.length === 0) return <p className="text-xs text-foreground/50">None</p>;
    return (
        <ul className="flex flex-col gap-1">
            {items.map((item) => (
                <li key={item.id}>
                    <Link
                        to={`/node/${item.id}`}
                        className={clsx("block rounded px-1 py-0.5", item.id === currentId && "bg-foreground/10")}
                    >
                        {item.name}
                        <span className="ml-2 text-xs text-foreground/50">{item.type}</span>
                    </Link>
                </li>
            ))}
        </ul>
    );
}

export default Sidebar;
