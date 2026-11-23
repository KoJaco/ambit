import { useEffect, useId, useRef, useState } from "react";
import { Link } from "react-router";
import { X } from "lucide-react";
import clsx from "clsx";
import { history } from "../history";
import {
    deleteRelationship,
    getNode,
    setRelationship,
    relationshipDrillId,
    type EdgeContract,
    type NodeDetail,
    type Relationship,
} from "../api";
import {
    EDGE_FORMATS,
    EDGE_REF_MAX,
    EDGE_TEXT_MAX,
    edgeContractDirty,
    normalizeEdgeContract,
    validateEdgeContract,
} from "../node-contract";
import { Counter, fieldClass, labelClass } from "./node-contract-fields";
import { specExcerpt } from "../excerpt";
import { edgeKindStyle, type EdgeKindStyle } from "../kind-style";
import { nodeGlyphClass, resolveIconComponent, resolveNodeColor } from "../node-presentation";
import { useMemberProbe } from "./flow/aggregated-members";
import { KindGlyphIcon } from "./flow/kind-glyph";
import { PanelHeaderActions, RightRailPanel } from "./right-rail-panel";
import { Tooltip } from "./tooltip";

type EndpointState = { status: "loading" } | { status: "missing" } | { status: "ready"; node: NodeDetail };

function useEndpoint(id: string): EndpointState {
    const [state, setState] = useState<EndpointState>({ status: "loading" });
    useEffect(() => {
        const ac = new AbortController();
        setState({ status: "loading" });
        getNode(id, ac.signal)
            .then((node) => setState({ status: "ready", node }))
            .catch(() => {
                if (!ac.signal.aborted) setState({ status: "missing" });
            });
        return () => ac.abort();
    }, [id]);
    return state;
}

function useReducedMotion(): boolean {
    const [reduced, setReduced] = useState(false);
    useEffect(() => {
        const query = window.matchMedia("(prefers-reduced-motion: reduce)");
        setReduced(query.matches);
        const onChange = () => setReduced(query.matches);
        query.addEventListener("change", onChange);
        return () => query.removeEventListener("change", onChange);
    }, []);
    return reduced;
}

function MiniNode({ id, state }: { id: string; state: EndpointState }) {
    if (state.status === "loading") {
        return (
            <div
                className="flex animate-pulse items-center gap-3 rounded-xl border border-border bg-card p-3"
                aria-busy="true"
                aria-label="Loading node"
            >
                <div className="size-9 shrink-0 rounded-lg bg-muted" />
                <div className="flex-1 space-y-1.5">
                    <div className="h-3 w-2/3 rounded bg-muted" />
                    <div className="h-2.5 w-1/3 rounded bg-muted" />
                </div>
            </div>
        );
    }
    if (state.status === "missing") {
        return (
            <div className="flex items-center gap-3 rounded-xl border border-dashed border-border bg-card p-3">
                <div className="size-9 shrink-0 rounded-lg bg-muted" aria-hidden />
                <div className="min-w-0">
                    <p className="truncate font-mono text-xs text-foreground" title={id}>
                        {id}
                    </p>
                    <p className="text-xs text-muted-foreground">Node not found</p>
                </div>
            </div>
        );
    }
    const { node } = state;
    const Icon = resolveIconComponent(node.icon);
    return (
        <div className="flex items-center gap-3 rounded-xl border border-border bg-card p-3 shadow-sm">
            <span className={nodeGlyphClass(resolveNodeColor(node.type, node.color), "lg")} aria-hidden>
                <Icon className="size-5" strokeWidth={2} />
            </span>
            <div className="min-w-0">
                <p className="truncate text-sm font-semibold text-foreground" title={node.name}>
                    {node.name}
                </p>
                <p className="truncate text-xs text-muted-foreground">{node.type || "node"}</p>
            </div>
        </div>
    );
}

// Vertical connector: decorative. Style comes from the shared kind table so it matches the canvas.
function FlowArrow({ kind, label }: { kind: EdgeKindStyle; label: string }) {
    const reduced = useReducedMotion();
    const height = 72;
    const dashed = kind.id === "async";
    return (
        <div className="flex items-center gap-3 px-6" aria-hidden>
            <svg width="16" height={height} viewBox={`0 0 16 ${height}`} className="shrink-0 text-muted-foreground">
                <line
                    x1="8"
                    y1="0"
                    x2="8"
                    y2={height - 8}
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap={dashed ? "round" : "butt"}
                    strokeDasharray={dashed ? "0.1 7" : undefined}
                >
                    {dashed && kind.animated && !reduced ? (
                        <animate attributeName="stroke-dashoffset" from="7.1" to="0" dur="0.8s" repeatCount="indefinite" />
                    ) : null}
                </line>
                <path d={`M2 ${height - 12} L8 ${height - 2} L14 ${height - 12} Z`} fill="currentColor" />
                {kind.glyph === "data" ? (
                    <g>
                        <rect x="0" y={height / 2 - 8} width="16" height="16" rx="4" className="fill-card" />
                        <path
                            d={`M3.5 ${height / 2 - 4}h9M3.5 ${height / 2}h9M3.5 ${height / 2 + 4}h5`}
                            stroke="currentColor"
                            strokeWidth="1.5"
                            strokeLinecap="round"
                        />
                    </g>
                ) : null}
            </svg>
            {label ? (
                <span className="max-w-full truncate rounded-full border border-border bg-card px-2.5 py-0.5 text-xs font-medium text-foreground">
                    {label}
                </span>
            ) : null}
        </div>
    );
}

function Excerpt({ name, state }: { name: string; state: EndpointState }) {
    const text = state.status === "ready" ? specExcerpt(state.node.markdown) : "";
    return (
        <div>
            <h4 className="text-xs font-semibold text-foreground">{name}</h4>
            {state.status === "loading" ? (
                <div className="mt-1 h-3 w-full animate-pulse rounded bg-muted" aria-hidden />
            ) : (
                <p className={clsx("mt-1 text-xs leading-relaxed", text ? "text-muted-foreground" : "italic text-muted-foreground/80")}>
                    {text || "No spec yet"}
                </p>
            )}
        </div>
    );
}

function KindRow({ kind }: { kind: EdgeKindStyle }) {
    return (
        <div>
            <span className="inline-flex items-center gap-1.5 text-foreground">
                <KindGlyphIcon glyph={kind.glyph} className="size-4 text-muted-foreground" />
                {kind.label}
            </span>
            <p className="mt-0.5 text-xs text-muted-foreground">{kind.description}</p>
        </div>
    );
}

function Members({ edge }: { edge: Relationship }) {
    const members = edge.members ?? [];
    const probed = useMemberProbe(members, members.length > 0);
    if (members.length === 0) return null;
    return (
        <section aria-labelledby="rel-members" className="space-y-2">
            <h3 id="rel-members" className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                Members ({members.length})
            </h3>
            <ul className="divide-y divide-border rounded-xl border border-border bg-card text-xs">
                {members.map((id) => {
                    const member = probed[id];
                    return (
                        <li key={id} className="flex min-h-9 items-center justify-between gap-2 px-3 py-1.5">
                            <span className="min-w-0">
                                <span className="block truncate text-foreground" title={member?.label || id}>
                                    {member?.label || id}
                                </span>
                                <span className="block truncate font-mono text-[10px] text-muted-foreground" title={id}>
                                    {member?.kind ? `${member.kind} · ` : ""}
                                    {id}
                                </span>
                            </span>
                            {member ? (
                                <Link
                                    to={`/relationship/${id}`}
                                    aria-label={`Open relationship ${member.label || id}`}
                                    className="shrink-0 font-medium text-primary underline focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary"
                                >
                                    Open
                                </Link>
                            ) : null}
                        </li>
                    );
                })}
            </ul>
        </section>
    );
}

// The edge contract: only for edges that are real relationships (aggregated edges carry none).
function EdgeContractSection({ edge, onSaved }: { edge: Relationship; onSaved: () => void }) {
    const uid = useId();
    // The selected edge is a snapshot taken when it was clicked, so the saved contract is tracked
    // here and updated from the server's response.
    const [saved, setSaved] = useState<EdgeContract | undefined>(edge.contract);
    const [draft, setDraft] = useState<EdgeContract>(edge.contract ?? {});
    const [error, setError] = useState<string | null>(null);
    const [busy, setBusy] = useState(false);
    useEffect(() => {
        setSaved(edge.contract);
        setDraft(edge.contract ?? {});
        setError(null);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [edge.id]);
    const problems = validateEdgeContract(draft);
    const hasProblems = Boolean(problems.ref || problems.text);
    const dirty = edgeContractDirty(saved, draft);
    const hasSaved = Object.keys(normalizeEdgeContract(saved)).length > 0;

    async function save(contract: EdgeContract) {
        setBusy(true);
        setError(null);
        try {
            // label and kind are sent as they are: an update with an id rewrites them.
            const result = await setRelationship({ id: edge.id, from: edge.from, to: edge.to, label: edge.label, kind: edge.kind, contract });
            setSaved(result.contract);
            setDraft(result.contract ?? {});
            onSaved();
        } catch (err: unknown) {
            setError(err instanceof Error ? err.message : "save failed");
        } finally {
            setBusy(false);
        }
    }

    return (
        <section aria-labelledby={`${uid}-heading`} className="space-y-3">
            <h3 id={`${uid}-heading`} className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                Contract
            </h3>
            {!hasSaved ? <p className="text-xs text-muted-foreground">No contract</p> : null}
            <div className="flex flex-col gap-1.5">
                <label htmlFor={`${uid}-format`} className={labelClass}>
                    Format
                </label>
                <select
                    id={`${uid}-format`}
                    className={fieldClass}
                    value={draft.format ?? ""}
                    onChange={(event) => setDraft({ ...draft, format: (event.target.value || undefined) as EdgeContract["format"] })}
                >
                    <option value="">None</option>
                    {EDGE_FORMATS.map((f) => (
                        <option key={f.value} value={f.value}>
                            {f.label}
                        </option>
                    ))}
                </select>
            </div>
            <div className="flex flex-col gap-1.5">
                <label htmlFor={`${uid}-ref`} className={labelClass}>
                    Reference
                </label>
                <input
                    id={`${uid}-ref`}
                    className={fieldClass}
                    aria-describedby={`${uid}-ref-hint ${uid}-ref-count`}
                    aria-invalid={problems.ref ? true : undefined}
                    placeholder="docs/api/orders.yaml"
                    value={draft.ref ?? ""}
                    onChange={(event) => setDraft({ ...draft, ref: event.target.value })}
                />
                <p id={`${uid}-ref-hint`} className="text-xs text-muted-foreground">
                    A repo-relative path, without '..'.
                </p>
                <Counter id={`${uid}-ref-count`} length={(draft.ref ?? "").trim().length} max={EDGE_REF_MAX} />
                {problems.ref ? <p className="text-xs text-destructive">{problems.ref}</p> : null}
            </div>
            <div className="flex flex-col gap-1.5">
                <label htmlFor={`${uid}-text`} className={labelClass}>
                    Text
                </label>
                <textarea
                    id={`${uid}-text`}
                    rows={4}
                    className={`${fieldClass} min-h-20 resize-y text-sm`}
                    aria-describedby={`${uid}-text-count`}
                    aria-invalid={problems.text ? true : undefined}
                    value={draft.text ?? ""}
                    onChange={(event) => setDraft({ ...draft, text: event.target.value })}
                />
                <Counter id={`${uid}-text-count`} length={(draft.text ?? "").trim().length} max={EDGE_TEXT_MAX} />
                {problems.text ? <p className="text-xs text-destructive">{problems.text}</p> : null}
            </div>
            <div className="flex flex-wrap gap-2">
                <button
                    type="button"
                    disabled={busy || !dirty || hasProblems}
                    className="inline-flex min-h-7 items-center rounded-full bg-primary px-4 py-1 text-sm font-medium text-primary-foreground hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-60"
                    onClick={() => void save(normalizeEdgeContract(draft))}
                >
                    {busy ? "Saving" : "Save contract"}
                </button>
                {hasSaved ? (
                    <button
                        type="button"
                        disabled={busy}
                        className="inline-flex min-h-7 items-center rounded-full bg-accent px-3 py-1 text-sm font-medium hover:bg-accent/70 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-60"
                        onClick={() => void save({})}
                    >
                        Clear contract
                    </button>
                ) : null}
            </div>
            {error ? (
                <p role="alert" className="text-xs text-destructive">
                    {error}
                </p>
            ) : null}
        </section>
    );
}

export function EdgeActions({
    edge,
    onClose,
    onMutated,
    onDeleted,
    onFocus,
    deleteRequest = 0,
}: {
    // Bumped by the Delete key: opens the same confirmation as the button.
    deleteRequest?: number;
    edge: Relationship;
    onClose: () => void;
    onMutated: () => void;
    onDeleted?: () => void;
    onFocus?: () => void;
}) {
    const [confirmDelete, setConfirmDelete] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [busy, setBusy] = useState(false);
    const seenDeleteRequest = useRef(deleteRequest);
    useEffect(() => {
        if (deleteRequest === seenDeleteRequest.current) return;
        seenDeleteRequest.current = deleteRequest;
        setConfirmDelete(true);
    }, [deleteRequest]);
    const from = useEndpoint(edge.from);
    const to = useEndpoint(edge.to);
    const fromName = from.status === "ready" ? from.node.name : edge.from;
    const toName = to.status === "ready" ? to.node.name : edge.to;
    const kind = edgeKindStyle(edge.kind);
    const extraKinds = edge.aggregated ? (edge.kinds ?? []).map((k) => edgeKindStyle(k)) : [];
    const drillId = relationshipDrillId(edge);
    const multi = Boolean(edge.aggregated && !edge.drillable && (edge.members?.length ?? 0) > 0);

    async function run(action: () => Promise<void>) {
        setBusy(true);
        setError(null);
        try {
            await action();
            onMutated();
        } catch (err: unknown) {
            setError(err instanceof Error ? err.message : "request failed");
        } finally {
            setBusy(false);
        }
    }

    return (
        <RightRailPanel label="Inspector" onPointerDown={onFocus}>
            <div className="flex items-center justify-between gap-2">
                <h2 className="text-sm font-semibold text-foreground">Relationship</h2>
                <PanelHeaderActions onClose={onClose} closeLabel="Close relationship panel" />
            </div>

            <p className="sr-only">
                From {fromName} to {toName}
                {edge.label ? `, ${edge.label}` : ""}, {kind.label}.
            </p>
            <div className="flex flex-col gap-1" role="group" aria-label="Data flow">
                <MiniNode id={edge.from} state={from} />
                <FlowArrow kind={multi ? edgeKindStyle(edge.kinds?.length === 1 ? edge.kinds[0] : "") : kind} label={edge.label} />
                <MiniNode id={edge.to} state={to} />
            </div>

            <section aria-labelledby="rel-details" className="space-y-3">
                <h3 id="rel-details" className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                    Details
                </h3>
                <dl className="space-y-3 text-sm">
                    <div>
                        <dt className="text-xs text-muted-foreground">Kind</dt>
                        <dd className="mt-0.5 space-y-2">
                            {multi && extraKinds.length > 0 ? (
                                extraKinds.map((k) => <KindRow key={k.label} kind={k} />)
                            ) : (
                                <KindRow kind={kind} />
                            )}
                        </dd>
                    </div>
                    <div>
                        <dt className="text-xs text-muted-foreground">Label</dt>
                        <dd className="mt-0.5 text-foreground">{edge.label || "No label"}</dd>
                    </div>
                    <div>
                        <dt className="text-xs text-muted-foreground">ID</dt>
                        <dd className="mt-0.5 break-all font-mono text-xs text-foreground">{edge.id}</dd>
                    </div>
                </dl>
            </section>

            <section aria-labelledby="rel-context" className="space-y-3">
                <h3 id="rel-context" className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                    Context
                </h3>
                <Excerpt name={fromName} state={from} />
                <Excerpt name={toName} state={to} />
            </section>

            {multi ? <Members edge={edge} /> : null}

            {!edge.aggregated ? <EdgeContractSection edge={edge} onSaved={onMutated} /> : null}

            {drillId ? (
                <Link
                    to={`/relationship/${drillId}`}
                    className="inline-flex h-9 items-center justify-center rounded-lg bg-primary px-3 text-sm font-medium text-primary-foreground hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
                >
                    Open relationship
                </Link>
            ) : null}

            <div className="mt-auto">
                {!confirmDelete ? (
                    <button
                        type="button"
                        disabled={busy}
                        className="rounded-lg border border-destructive/40 px-3 py-1.5 text-sm text-destructive hover:bg-destructive/5 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary disabled:opacity-50"
                        onClick={() => setConfirmDelete(true)}
                    >
                        Delete relationship…
                    </button>
                ) : (
                    <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm">
                        <p className="text-destructive">
                            Deletes this edge and all interior members (including nested interiors).
                        </p>
                        <div className="mt-2 flex gap-2">
                            <button
                                type="button"
                                disabled={busy}
                                className="rounded-lg bg-destructive px-3 py-1.5 text-destructive-foreground focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary disabled:opacity-50"
                                onClick={() =>
                                    void run(async () => {
                                        await deleteRelationship(edge.id);
                                        // An edge with an interior takes its members with it; undo cannot
                                        // restore those, so only plain edges are undoable.
                                        if (edge.drillable || edge.aggregated) history.clear();
                                        else {
                                            const { id, from, to, label, kind } = edge;
                                            history.record({
                                                label: `delete ${label || "relationship"}`,
                                                undo: async () => void (await setRelationship({ id, from, to, label, kind })),
                                                redo: () => deleteRelationship(id),
                                            });
                                        }
                                        onClose();
                                        onDeleted?.();
                                    })
                                }
                            >
                                Confirm delete
                            </button>
                            <button
                                type="button"
                                className="rounded-lg border border-border px-3 py-1.5 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary"
                                onClick={() => setConfirmDelete(false)}
                            >
                                Cancel
                            </button>
                        </div>
                    </div>
                )}
                {error ? (
                    <p role="alert" className="mt-2 text-xs text-destructive">
                        {error}
                    </p>
                ) : null}
            </div>
        </RightRailPanel>
    );
}
