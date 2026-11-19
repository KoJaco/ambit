import { useEffect, useState } from "react";
import { Link } from "react-router";
import clsx from "clsx";
import { createNode as postNode, getLevel, getRelationshipLevel, type SchemaColumn, type Summary } from "../api";
import {
    PresentationControls,
    type PresentationSubview,
} from "./node-inspector-presentation";
import styles from "./Sidebar.module.css";

type CreatePresentation = {
    icon?: string;
    color?: string;
    renderer?: string;
    schema?: SchemaColumn[];
};

export function Sidebar({
    nodeId,
    relationshipId,
    refreshKey,
    onMutated,
}: {
    nodeId?: string;
    relationshipId?: string;
    refreshKey: number;
    onMutated: () => void;
}) {
    const [collapsed, setCollapsed] = useState(false);
    const [roots, setRoots] = useState<Summary[]>([]);
    const [children, setChildren] = useState<Summary[]>([]);
    const [current, setCurrent] = useState<Summary | null>(null);
    const [name, setName] = useState("");
    const [type, setType] = useState("");
    const [presentation, setPresentation] = useState<CreatePresentation>({});
    const [presentationSubview, setPresentationSubview] = useState<PresentationSubview>(null);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        const ac = new AbortController();
        getLevel(undefined, ac.signal)
            .then((level) => setRoots(level.children ?? []))
            .catch((err: unknown) => {
                if (err instanceof DOMException && err.name === "AbortError") return;
                setError(err instanceof Error ? err.message : "level request failed");
            });
        return () => ac.abort();
    }, [nodeId, refreshKey]);

    useEffect(() => {
        if (relationshipId) {
            const ac = new AbortController();
            getRelationshipLevel(relationshipId, ac.signal)
                .then((level) => {
                    setCurrent({ id: relationshipId, name: level.relationship.label || "Relationship", type: "", status: "", protected: false });
                    setChildren(level.children ?? []);
                })
                .catch((err: unknown) => {
                    if (err instanceof DOMException && err.name === "AbortError") return;
                    setError(err instanceof Error ? err.message : "level request failed");
                });
            return () => ac.abort();
        }
        if (!nodeId) {
            setChildren([]);
            setCurrent(null);
            return;
        }
        const ac = new AbortController();
        getLevel(nodeId, ac.signal)
            .then((level) => {
                setCurrent(level.node);
                setChildren(level.children ?? []);
            })
            .catch((err: unknown) => {
                if (err instanceof DOMException && err.name === "AbortError") return;
                setError(err instanceof Error ? err.message : "level request failed");
            });
        return () => ac.abort();
    }, [nodeId, relationshipId, refreshKey]);

    function resetCreateForm() {
        setName("");
        setType("");
        setPresentation({});
        setPresentationSubview(null);
    }

    async function createNode() {
        setError(null);
        const body: Parameters<typeof postNode>[0] = { name, type };
        if (relationshipId) {
            body.relationship_id = relationshipId;
        } else {
            body.parent_id = nodeId ?? "";
        }
        if (presentation.icon !== undefined) {
            body.icon = presentation.icon;
        }
        if (presentation.color !== undefined) {
            body.color = presentation.color;
        }
        if (presentation.renderer !== undefined) {
            body.renderer = presentation.renderer;
        }
        if (presentation.schema !== undefined) {
            body.schema = presentation.schema.filter((row) => row.title.trim() !== "" && row.type.trim() !== "");
        }
        try {
            await postNode(body);
        } catch (err) {
            setError(err instanceof Error ? err.message : "create failed");
            return;
        }
        resetCreateForm();
        onMutated();
    }

    return (
        <aside
            className={clsx(
                "pointer-events-auto absolute top-0 left-0 flex flex-col overflow-hidden rounded-xl border border-border bg-card shadow-md",
                collapsed ? "w-10" : "max-h-full min-h-[25vh] w-64"
            )}
        >
            <button
                type="button"
                className="ml-2 my-2 w-fit rounded-full border border-border bg-primary/10 text-primary px-2 py-1 text-xs"
                onClick={() => setCollapsed((value) => !value)}
            >
                {collapsed ? ">" : "<"}
            </button>
            {collapsed ? null : (
                <div className={clsx("flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-3 pb-4 text-sm", styles.sidebarScroll)}>
                    <section>
                        <h2 className="mb-1 text-xs uppercase tracking-wide text-foreground/60">Roots</h2>
                        <NodeLinks items={roots} currentId={nodeId} />
                    </section>
                    {nodeId || relationshipId ? (
                        <section>
                            <h2 className="mb-1 text-xs uppercase tracking-wide text-foreground/60">
                                {current?.name ?? nodeId}
                            </h2>
                            <NodeLinks items={children} currentId={nodeId} />
                        </section>
                    ) : null}
                    <form
                        className="mt-auto flex min-h-0 flex-col gap-2"
                        onSubmit={(event) => {
                            event.preventDefault();
                            void createNode();
                        }}
                    >
                        {presentationSubview ? (
                            <div className="flex min-h-0 flex-1 flex-col">
                                <PresentationControls
                                    subview={presentationSubview}
                                    onSubview={setPresentationSubview}
                                    backLabel="New node"
                                    icon={presentation.icon}
                                    color={presentation.color}
                                    type={type}
                                    renderer={presentation.renderer}
                                    schema={presentation.schema}
                                    onPickIcon={(id) => setPresentation((prev) => ({ ...prev, icon: id }))}
                                    onPickColor={(token) => setPresentation((prev) => ({ ...prev, color: token }))}
                                    onPickRenderer={(id) => setPresentation((prev) => ({ ...prev, renderer: id }))}
                                    onChangeSchema={(rows) => setPresentation((prev) => ({ ...prev, schema: rows }))}
                                />
                            </div>
                        ) : (
                            <div className="border p-1.5 rounded-md space-y-1.5">
                                <h2 className="text-xs uppercase tracking-wide text-foreground/60">New node</h2>
                                <input
                                    className="rounded border border-border bg-accent px-2 w-full py-1"
                                    placeholder="Name"
                                    value={name}
                                    onChange={(event) => setName(event.target.value)}
                                />
                                <input
                                    className="rounded border border-border w-full bg-accent px-2 py-1"
                                    placeholder="Type"
                                    value={type}
                                    onChange={(event) => setType(event.target.value)}
                                />
                                <PresentationControls
                                    subview={null}
                                    onSubview={setPresentationSubview}
                                    backLabel="New node"
                                    icon={presentation.icon}
                                    color={presentation.color}
                                    type={type}
                                    renderer={presentation.renderer}
                                    schema={presentation.schema}
                                    onPickIcon={() => { }}
                                    onPickColor={() => { }}
                                    onPickRenderer={() => { }}
                                    onChangeSchema={() => { }}
                                />
                                {error ? <p className="text-xs text-red-600">{error}</p> : null}
                                <button type="submit" className="rounded bg-primary text-primary-foreground px-2 py-1 w-full">
                                    Create
                                </button>
                            </div>
                        )}
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
                        className={clsx("block rounded px-1 py-0.5", item.id === currentId && "bg-accent")}
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
