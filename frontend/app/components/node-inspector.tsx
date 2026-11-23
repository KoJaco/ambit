import clsx from "clsx";
import { useEffect, useId, useRef, useState } from "react";
import { Tooltip } from "./tooltip";
import {
    clearAssignment,
    createNode,
    deleteNode,
    getAssignment,
    getLevel,
    getNode,
    getRelationshipLevel,
    setAssignment,
    updateNode,
    type Assignment,
    type NodeDetail,
} from "../api";
import { history } from "../history";
import { checkNodeDelete, deleteNodeUndoable, nodeDeleteMessage } from "../delete-actions";
import { assignmentLabel, assignmentState } from "../assignment";
import {
    nodeDotClass,
    nodeGlyphClass,
    resolveIconComponent,
    resolveNodeColor,
    statusLabel,
    statusTone,
} from "../node-presentation";
import { previewScope } from "../scope-preview";
import { AppearanceFields, AppearanceSummary } from "./appearance-editor";
import { CollapsibleSection } from "./collapsible-section";
import { PanelHeaderActions, RightRailPanel } from "./right-rail-panel";
import { nodeSettingsSummary } from "../node-settings";
import { setInspectorDirty } from "../right-rail-state";

const STATUSES = ["draft", "specified", "assigned", "in_progress", "done", "blocked"] as const;

export default function NodeInspector({
    nodeId,
    onClose,
    onSaved,
    onFocus,
    onDeleted,
    deleteRequest = 0,
    refreshKey = 0,
}: {
    refreshKey?: number;
    onDeleted?: () => void;
    // Bumped by the Delete key: opens the same confirmation as the button.
    deleteRequest?: number;
    nodeId: string;
    onClose: () => void;
    onSaved: () => void;
    onFocus?: () => void;
}) {
    const uid = useId();
    const settingsId = `${uid}-settings`;
    const appearanceId = `${uid}-appearance`;
    const [node, setNode] = useState<NodeDetail | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [saving, setSaving] = useState(false);
    const [appearanceOpen, setAppearanceOpen] = useState(false);
    const [settingsOpen, setSettingsOpen] = useState(false);
    const [presentationTouched, setPresentationTouched] = useState(false);
    const [deletePlan, setDeletePlan] = useState<{ count: number } | null>(null);
    const [deleteBusy, setDeleteBusy] = useState(false);
    const seenDeleteRequest = useRef(deleteRequest);
    // The node as last loaded or saved: what undo restores.
    const baseline = useRef<NodeDetail | null>(null);

    useEffect(() => {
        const ac = new AbortController();
        setNode(null);
        setError(null);
        setAppearanceOpen(false);
        setPresentationTouched(false);
        getNode(nodeId, ac.signal)
            .then((loaded) => {
                baseline.current = loaded;
                setNode(loaded);
            })
            .catch((err: unknown) => {
                if (err instanceof DOMException && err.name === "AbortError") return;
                setError(err instanceof Error ? err.message : "node request failed");
            });
        return () => ac.abort();
    }, [nodeId]);

    // Tell the page whether unsaved edits are open, so a new proposal does not cover them.
    useEffect(() => {
        const dirty = !!node && (presentationTouched || JSON.stringify(node) !== JSON.stringify(baseline.current));
        setInspectorDirty(dirty);
    }, [node, presentationTouched]);
    useEffect(() => () => setInspectorDirty(false), []);

    const [assignment, setAssignmentState] = useState<Assignment | null>(null);
    const [otherName, setOtherName] = useState<string | null>(null);
    const [assignError, setAssignError] = useState<string | null>(null);
    const [assignBusy, setAssignBusy] = useState(false);

    // Refetched on mount, after set/clear, and on every model-changed event (refreshKey),
    // which the server also sends when local.json changes.
    useEffect(() => {
        const ac = new AbortController();
        getAssignment(ac.signal)
            .then((value) => {
                setAssignmentState(value);
                setAssignError(null);
            })
            .catch((err: unknown) => {
                if (err instanceof DOMException && err.name === "AbortError") return;
                setAssignError(err instanceof Error ? err.message : "assignment request failed");
            });
        return () => ac.abort();
    }, [nodeId, refreshKey]);

    const otherId = assignmentState(assignment, nodeId) === "other" ? assignment?.node_id ?? null : null;
    useEffect(() => {
        setOtherName(null);
        if (!otherId) return;
        const ac = new AbortController();
        getNode(otherId, ac.signal)
            .then((other) => setOtherName(other.name))
            .catch(() => undefined);
        return () => ac.abort();
    }, [otherId, refreshKey]);

    async function changeAssignment(action: () => Promise<void>) {
        setAssignBusy(true);
        setAssignError(null);
        try {
            await action();
            setAssignmentState(await getAssignment());
        } catch (err) {
            setAssignError(err instanceof Error ? err.message : "assignment change failed");
        } finally {
            setAssignBusy(false);
        }
    }

    // Children and relationship count come from the server each time they are needed.
    async function checkDelete(target: NodeDetail) {
        const own = await getLevel(target.id);
        const parentLevel = target.relationship_id
            ? await getRelationshipLevel(target.relationship_id)
            : await getLevel(target.parent_id || undefined);
        return checkNodeDelete(target.id, own.children?.length ?? 0, parentLevel);
    }

    async function startDelete() {
        if (!node || deleteBusy) return;
        setDeleteBusy(true);
        setError(null);
        try {
            const check = await checkDelete(node);
            if (check.ok) setDeletePlan({ count: check.relationshipCount });
            else setError(check.reason);
        } catch (err) {
            setError(err instanceof Error ? err.message : "delete check failed");
        } finally {
            setDeleteBusy(false);
        }
    }

    async function confirmDelete() {
        if (!node || !deletePlan) return;
        setDeleteBusy(true);
        setError(null);
        try {
            // The model may have changed since the prompt; re-check so undo is only offered when
            // the delete took nothing but the node (a changed count clears history instead).
            const fresh = await checkDelete(node);
            if (!fresh.ok) {
                setDeletePlan(null);
                setError(fresh.reason);
                return;
            }
            const count = fresh.relationshipCount === deletePlan.count ? deletePlan.count : Math.max(1, fresh.relationshipCount);
            await deleteNodeUndoable(node, count, { history, deleteNode, createNode });
            onClose();
            onDeleted?.();
        } catch (err) {
            setError(err instanceof Error ? err.message : "delete failed");
        } finally {
            setDeleteBusy(false);
        }
    }

    // A request that arrives while the node is still loading is held and run once it has loaded.
    const deleteWaiting = useRef(false);
    useEffect(() => {
        if (deleteRequest === seenDeleteRequest.current) return;
        seenDeleteRequest.current = deleteRequest;
        if (node) void startDelete();
        else deleteWaiting.current = true;
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [deleteRequest]);
    useEffect(() => {
        if (!node || !deleteWaiting.current) return;
        deleteWaiting.current = false;
        void startDelete();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [node]);

    async function save() {
        if (!node) return;
        setSaving(true);
        setError(null);
        try {
            const before = baseline.current;
            const patch = toPatch(node, presentationTouched);
            const saved = await updateNode(node.id, patch);
            if (before) {
                const undoPatch = toPatch(before, true);
                history.record({
                    label: `edit ${before.name}`,
                    undo: async () => void (await updateNode(node.id, undoPatch)),
                    redo: async () => void (await updateNode(node.id, patch)),
                });
            }
            baseline.current = saved;
            setNode(saved);
            setPresentationTouched(false);
            onSaved();
        } catch (err) {
            setError(err instanceof Error ? err.message : "save failed");
        } finally {
            setSaving(false);
        }
    }

    const preview = node
        ? previewScope({
            implementation: node.implementation,
            scope: node.scope,
            protected: node.protected,
        })
        : null;

    const markPresentation = () => setPresentationTouched(true);

    const state = assignmentState(assignment, nodeId);
    const summary = node ? nodeSettingsSummary(node, state) : [];
    // The header holds the actions outside the scrolling body, so the body's scrollbar never runs
    // behind the theme toggle or the close button.
    const header = (
        <div className="-mb-4 flex shrink-0 items-center justify-between gap-3">
            {!node || !preview ? (
                <h2 className="font-semibold">Inspector</h2>
            ) : (
                <span
                    className={nodeGlyphClass(resolveNodeColor(node.type, node.color), "lg")}
                    aria-hidden
                >
                    {(() => {
                        const Icon = resolveIconComponent(node.icon);
                        return <Icon className="size-5" strokeWidth={2} />;
                    })()}
                </span>
            )}
            <PanelHeaderActions onClose={onClose} closeLabel="Close inspector" />
        </div>
    );

    return (
        <RightRailPanel label="Inspector" className="overflow-hidden" onPointerDown={() => onFocus?.()}>
            {header}
            {error ? <p className="shrink-0 text-destructive">{error}</p> : null}
            {node && preview ? (
                <>
                    <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto p-1 [scrollbar-gutter:stable]">
                        <div className="flex flex-col gap-2">
                            <input
                                aria-label="Name"
                                className={clsx(inputClass, "text-lg font-semibold")}
                                value={node.name}
                                onChange={(event) => setNode({ ...node, name: event.target.value })}
                            />
                            <input
                                aria-label="Type"
                                className={clsx(inputClass, "text-xs text-muted-foreground")}
                                value={node.type}
                                onChange={(event) => setNode({ ...node, type: event.target.value })}
                            />
                        </div>
                        <div className="flex flex-col gap-2">
                            <label htmlFor={`${uid}-spec`} className={sectionLabelClass}>
                                Spec
                            </label>
                            <textarea
                                id={`${uid}-spec`}
                                rows={12}
                                className={clsx(inputClass, "min-h-56 resize-y text-sm leading-relaxed")}
                                placeholder="What this node is responsible for."
                                value={node.markdown}
                                onChange={(event) => setNode({ ...node, markdown: event.target.value })}
                            />
                        </div>
                        <div className="flex flex-col gap-2">
                            <label htmlFor={`${uid}-status`} className={sectionLabelClass}>
                                Status
                            </label>
                            <div className="flex items-center gap-2">
                                <span className={nodeDotClass(statusTone(node.status))} aria-hidden />
                                <select
                                    id={`${uid}-status`}
                                    className={clsx(inputClass, "flex-1")}
                                    value={node.status}
                                    onChange={(event) => setNode({ ...node, status: event.target.value })}
                                >
                                    {statusOptions(node.status).map((status) => (
                                        <option key={status} value={status}>
                                            {statusLabel(status)}
                                        </option>
                                    ))}
                                </select>
                            </div>
                        </div>
                        <CollapsibleSection
                            title="Appearance"
                            id={appearanceId}
                            open={appearanceOpen}
                            onToggle={() => setAppearanceOpen((open) => !open)}
                            summary={<AppearanceSummary type={node.type} value={{ icon: node.icon, color: node.color, renderer: node.renderer }} />}
                        >
                            <AppearanceFields
                                type={node.type}
                                value={{ icon: node.icon, color: node.color, renderer: node.renderer }}
                                onChange={(next) => {
                                    markPresentation();
                                    setNode({ ...node, ...next });
                                }}
                                schema={node.schema}
                                onChangeSchema={(rows) => {
                                    markPresentation();
                                    setNode({ ...node, schema: rows });
                                }}
                            />
                        </CollapsibleSection>
                        <CollapsibleSection
                            title="Node settings"
                            id={settingsId}
                            open={settingsOpen}
                            onToggle={() => setSettingsOpen((open) => !open)}
                            summary={
                                <span className="flex flex-wrap gap-1.5">
                                    {summary.map((item) => (
                                        <span
                                            key={item.kind}
                                            className={clsx(
                                                "rounded-full px-2 py-0.5 text-[11px]",
                                                item.kind === "scope"
                                                    ? "bg-muted text-muted-foreground"
                                                    : "bg-primary/10 font-medium text-primary",
                                            )}
                                        >
                                            {item.label}
                                        </span>
                                    ))}
                                </span>
                            }
                        >
                            <div className="flex flex-col gap-5">
                                            <div className="flex flex-col gap-2">
                                                <label htmlFor={`${uid}-impl`} className={sectionLabelClass}>
                                                    Implemented in
                                                </label>
                                                <p id={`${uid}-impl-hint`} className="text-xs text-muted-foreground">
                                                    One glob per line
                                                </p>
                                                <textarea
                                                    id={`${uid}-impl`}
                                                    aria-describedby={`${uid}-impl-hint`}
                                                    className={clsx(inputClass, "min-h-16 font-mono text-xs")}
                                                    value={node.implementation.join("\n")}
                                                    onChange={(event) =>
                                                        setNode({ ...node, implementation: event.target.value.split("\n") })
                                                    }
                                                />
                                            </div>
                                            <div className="flex flex-col gap-2">
                                                <label htmlFor={`${uid}-scope`} className={sectionLabelClass}>
                                                    Agent may edit
                                                </label>
                                                <textarea
                                                    id={`${uid}-scope`}
                                                    className={clsx(inputClass, "min-h-16 font-mono text-xs")}
                                                    placeholder="Empty: same as Implemented in"
                                                    value={node.scope.join("\n")}
                                                    onChange={(event) =>
                                                        setNode({ ...node, scope: event.target.value.split("\n") })
                                                    }
                                                />
                                                <p className="text-xs text-muted-foreground">
                                                    {preview.defaultsToImplementation
                                                        ? "Empty scope defaults to implementation."
                                                        : "Scope covers the globs written above."}
                                                </p>
                                                <ul className="flex flex-wrap gap-1.5" aria-label="Effective globs">
                                                    {preview.globs.length > 0 ? (
                                                        preview.globs.map((glob) => (
                                                            <li
                                                                key={glob}
                                                                className="rounded-full bg-muted px-2 py-0.5 font-mono text-[11px] text-muted-foreground"
                                                            >
                                                                {glob}
                                                            </li>
                                                        ))
                                                    ) : (
                                                        <li className="font-mono text-xs text-muted-foreground">(no globs)</li>
                                                    )}
                                                </ul>
                                            </div>
                                            <div className="flex items-start gap-3">
                                                <button
                                                    type="button"
                                                    role="switch"
                                                    aria-checked={node.protected}
                                                    aria-labelledby={`${uid}-protected`}
                                                    aria-describedby={`${uid}-protected-help`}
                                                    className={clsx(
                                                        "relative mt-0.5 inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring",
                                                        node.protected ? "bg-primary" : "bg-border",
                                                    )}
                                                    onClick={() => setNode({ ...node, protected: !node.protected })}
                                                >
                                                    <span
                                                        aria-hidden
                                                        className={clsx(
                                                            "inline-block size-4 rounded-full bg-card transition-transform",
                                                            node.protected ? "translate-x-[18px]" : "translate-x-0.5",
                                                        )}
                                                    />
                                                </button>
                                                <div className="flex flex-col gap-0.5">
                                                    <span id={`${uid}-protected`} className="text-sm font-medium">
                                                        Protected
                                                    </span>
                                                    <span id={`${uid}-protected-help`} className="text-xs text-muted-foreground">
                                                        Agents cannot edit this node's files, regardless of assignment.
                                                    </span>
                                                </div>
                                            </div>
                                            <div className="flex flex-col gap-1.5 border-t border-border pt-4">
                                                <span className={sectionLabelClass}>Agent assignment</span>
                                                <p className="text-sm" role="status">
                                                    {assignmentLabel(
                                                        assignment,
                                                        node.id,
                                                        otherId && otherName ? [{ id: otherId, name: otherName }] : [],
                                                    )}
                                                </p>
                                                <p className="text-xs text-muted-foreground">
                                                    Agents are limited to this node's 'Agent may edit' scope (ambit check, MCP check_scope)
                                                </p>
                                                {assignError ? <p className="text-xs text-destructive">{assignError}</p> : null}
                                                <div className="flex flex-wrap gap-2">
                                                    <button
                                                        type="button"
                                                        className="rounded-full bg-accent px-3 py-1 text-xs font-medium hover:bg-accent/70 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-60"
                                                        disabled={assignBusy || state === "this"}
                                                        onClick={() => {
                                                            void changeAssignment(() => setAssignment(node.id));
                                                        }}
                                                    >
                                                        Assign agent to this node
                                                    </button>
                                                    <button
                                                        type="button"
                                                        className="rounded-full bg-accent px-3 py-1 text-xs font-medium hover:bg-accent/70 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-60"
                                                        disabled={assignBusy || state === "none"}
                                                        onClick={() => {
                                                            void changeAssignment(clearAssignment);
                                                        }}
                                                    >
                                                        Clear assignment
                                                    </button>
                                                </div>
                                            </div>
                                        </div>
                        </CollapsibleSection>
                        <div className="flex flex-col gap-2">
                            {deletePlan ? (
                                <div role="alertdialog" aria-label="Confirm delete" className="rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm">
                                    <p className="text-destructive">{nodeDeleteMessage(node.name, deletePlan.count)}</p>
                                    <div className="mt-2 flex gap-2">
                                        <button
                                            type="button"
                                            autoFocus
                                            disabled={deleteBusy}
                                            className="rounded-lg bg-destructive px-3 py-1.5 text-destructive-foreground focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary disabled:opacity-50"
                                            onClick={() => void confirmDelete()}
                                        >
                                            Confirm delete
                                        </button>
                                        <button
                                            type="button"
                                            className="rounded-lg border border-border px-3 py-1.5 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary"
                                            onClick={() => setDeletePlan(null)}
                                        >
                                            Cancel
                                        </button>
                                    </div>
                                </div>
                            ) : (
                                <button
                                    type="button"
                                    disabled={deleteBusy}
                                    className="self-start rounded-lg border border-destructive/40 px-3 py-1.5 text-sm text-destructive hover:bg-destructive/5 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary disabled:opacity-50"
                                    onClick={() => void startDelete()}
                                >
                                    Delete node…
                                </button>
                            )}
                        </div>
                    </div>
                    <div className="flex shrink-0 items-center justify-between gap-3 border-t border-border pt-3">
                        <span className="min-w-0 truncate font-mono text-[11px] text-muted-foreground">
                            .arch/nodes/{node.id}.json
                        </span>
                        <button
                            type="button"
                            className="shrink-0 rounded-full bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-60"
                            disabled={saving}
                            onClick={() => {
                                void save();
                            }}
                        >
                            {saving ? "Saving" : "Save"}
                        </button>
                    </div>
                </>
            ) : null}
        </RightRailPanel>
    );
}

const inputClass =
    "w-full rounded-[10px] border border-transparent bg-accent px-2.5 py-1.5 hover:border-border focus:border-primary focus:bg-card focus:outline-none focus-visible:ring-2 focus-visible:ring-ring/40";
const sectionLabelClass = "text-xs font-medium uppercase tracking-wide text-muted-foreground";

function statusOptions(current: string): readonly string[] {
    if ((STATUSES as readonly string[]).includes(current)) return STATUSES;
    return [current, ...STATUSES];
}

// presentation=false leaves icon/colour/renderer/schema untouched, as a save without appearance edits does.
function toPatch(node: NodeDetail, presentation: boolean): Parameters<typeof updateNode>[1] {
    const patch: Parameters<typeof updateNode>[1] = {
        name: node.name,
        type: node.type,
        status: node.status,
        implementation: lines(node.implementation),
        scope: lines(node.scope),
        protected: node.protected,
        markdown: node.markdown,
    };
    if (presentation) {
        patch.icon = node.icon ?? "";
        patch.color = node.color ?? "";
        patch.renderer = node.renderer ?? "";
        patch.schema = (node.schema ?? []).filter((row) => row.title.trim() !== "" && row.type.trim() !== "");
    }
    return patch;
}

function lines(value: string[]): string[] {
    return value.map((line) => line.trim()).filter((line) => line !== "");
}
