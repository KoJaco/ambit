import { useEffect, useId, useRef, useState } from "react";
import { Link } from "react-router";
import clsx from "clsx";
import { PanelLeftClose, PanelLeftOpen, Plus } from "lucide-react";
import { history } from "../history";
import { createNode as postNode, deleteNode, getLevel, getRelationshipLevel, type SchemaColumn, type Summary } from "../api";
import { AppearanceFields, AppearanceSummary } from "./appearance-editor";
import { CollapsibleSection } from "./collapsible-section";
import { Collapsible } from "./collapsible";
import { groupNodesByType } from "../node-groups";
import { nodeGlyphClass, resolveIconComponent, resolveNodeColor } from "../node-presentation";
import { Tooltip } from "./tooltip";
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
    const [creating, setCreating] = useState(false);
    const [appearanceOpen, setAppearanceOpen] = useState(false);
    const creatorId = useId();
    const nameRef = useRef<HTMLInputElement>(null);
    const newButtonRef = useRef<HTMLButtonElement>(null);
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
        setAppearanceOpen(false);
    }

    function openCreator() {
        setCreating(true);
        // The region turns visible as it animates in; focus once it can take it.
        window.setTimeout(() => nameRef.current?.focus(), 50);
    }

    function closeCreator() {
        resetCreateForm();
        setError(null);
        setCreating(false);
        newButtonRef.current?.focus();
    }

    async function createNode() {
        if (name.trim() === "") return;
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
            let created = await postNode(body);
            history.record({
                label: `create ${created.name}`,
                undo: () => deleteNode(created.id),
                redo: async () => {
                    created = await postNode(body);
                },
            });
        } catch (err) {
            setError(err instanceof Error ? err.message : "create failed");
            return;
        }
        resetCreateForm();
        setCreating(false);
        newButtonRef.current?.focus();
        onMutated();
    }

    return (
        <aside
            className={clsx(
                "pointer-events-auto absolute top-0 left-0 flex flex-col overflow-hidden border border-border bg-card shadow-md",
                collapsed ? "size-10 rounded-full" : "relative max-h-full min-h-[25vh] w-64 rounded-xl"
            )}
        >
            <Tooltip label={collapsed ? "Expand sidebar" : "Collapse sidebar"} prefer="bottom">
                <button
                    type="button"
                    aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
                    aria-expanded={!collapsed}
                    className={clsx(
                        "inline-flex size-8 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary",
                        collapsed ? "m-auto" : "absolute top-2 right-2 z-10"
                    )}
                    onClick={() => setCollapsed((value) => !value)}
                >
                    {collapsed ? <PanelLeftOpen className="size-4" aria-hidden /> : <PanelLeftClose className="size-4" aria-hidden />}
                </button>
            </Tooltip>
            {collapsed ? null : (
                <div className={clsx("flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-3 pt-3 pb-4 text-sm", styles.sidebarScroll)}>
                    <section>
                        <h2 className="mb-1 pr-8 text-xs uppercase tracking-wide text-foreground/60">Roots</h2>
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
                </div>
            )}
            {collapsed ? null : (
                <div className="shrink-0 border-t border-border px-3 py-2.5">
                    <Collapsible open={creating} id={creatorId}>
                        <form
                            className="flex flex-col gap-2 pb-2.5"
                            aria-label="New node"
                            onSubmit={(event) => {
                                event.preventDefault();
                                void createNode();
                            }}
                            onKeyDown={(event) => {
                                if (event.key !== "Escape") return;
                                event.stopPropagation();
                                closeCreator();
                            }}
                        >
                            <input
                                ref={nameRef}
                                aria-label="Name"
                                className="w-full rounded-[10px] border border-transparent bg-accent px-2.5 py-1.5 hover:border-border focus:border-primary focus:bg-card focus:outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
                                placeholder="Name"
                                value={name}
                                onChange={(event) => setName(event.target.value)}
                            />
                            <input
                                aria-label="Type"
                                className="w-full rounded-[10px] border border-transparent bg-accent px-2.5 py-1.5 hover:border-border focus:border-primary focus:bg-card focus:outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
                                placeholder="Type"
                                value={type}
                                onChange={(event) => setType(event.target.value)}
                            />
                            <CollapsibleSection
                                title="Appearance"
                                id={`${creatorId}-appearance`}
                                open={appearanceOpen}
                                onToggle={() => setAppearanceOpen((open) => !open)}
                                maxHeightClass="max-h-[40vh]"
                                summary={<AppearanceSummary type={type} value={presentation} />}
                            >
                                <AppearanceFields
                                    type={type}
                                    value={presentation}
                                    onChange={(next) => setPresentation((prev) => ({ ...prev, ...next }))}
                                    schema={presentation.schema}
                                    onChangeSchema={(rows) => setPresentation((prev) => ({ ...prev, schema: rows }))}
                                />
                            </CollapsibleSection>
                            {error ? <p className="text-xs text-destructive">{error}</p> : null}
                            <div className="flex gap-2">
                                <button
                                    type="submit"
                                    disabled={name.trim() === ""}
                                    className="flex-1 rounded-full bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-60"
                                >
                                    Create
                                </button>
                                <button
                                    type="button"
                                    className="rounded-full border border-border px-3 py-1.5 text-sm hover:bg-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
                                    onClick={closeCreator}
                                >
                                    Cancel
                                </button>
                            </div>
                        </form>
                    </Collapsible>
                    <button
                        ref={newButtonRef}
                        type="button"
                        aria-expanded={creating}
                        aria-controls={creatorId}
                        className="flex min-h-8 w-full items-center justify-center gap-1.5 rounded-full border border-border bg-card px-3 py-1.5 text-sm font-medium hover:bg-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
                        onClick={() => (creating ? closeCreator() : openCreator())}
                    >
                        <Plus className="size-4" aria-hidden />
                        New node
                    </button>
                </div>
            )}
        </aside>
    );
}

function NodeLinks({ items, currentId }: { items: Summary[]; currentId?: string }) {
    if (items.length === 0) return <p className="text-xs text-foreground/50">None</p>;
    return (
        <div className="flex flex-col gap-3">
            {groupNodesByType(items).map((group) => (
                <div key={group.type} role="group" aria-label={group.label}>
                    <h3 className="mb-1 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
                        {group.label}
                    </h3>
                    <ul className="flex flex-col gap-1">
                        {group.items.map((item) => {
                            const Icon = resolveIconComponent(item.icon);
                            const current = item.id === currentId;
                            return (
                                <li key={item.id}>
                                    <Link
                                        to={`/node/${item.id}`}
                                        aria-current={current ? "page" : undefined}
                                        className={clsx(
                                            "flex min-h-8 items-center gap-2 rounded-[10px] border bg-card px-2 py-1.5 text-sm hover:bg-accent focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring",
                                            current ? "border-primary bg-accent ring-1 ring-primary" : "border-border",
                                        )}
                                    >
                                        <span
                                            className={nodeGlyphClass(resolveNodeColor(item.type, item.color))}
                                            style={{ width: "1.5rem", height: "1.5rem", borderRadius: "0.375rem" }}
                                            aria-hidden
                                        >
                                            <Icon className="size-3.5" strokeWidth={2} />
                                        </span>
                                        <span className="min-w-0 flex-1 truncate">{item.name}</span>
                                    </Link>
                                </li>
                            );
                        })}
                    </ul>
                </div>
            ))}
        </div>
    );
}

export default Sidebar;
