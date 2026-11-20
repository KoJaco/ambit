import clsx from "clsx";
import { X } from "lucide-react";
import { useEffect, useId, useState } from "react";
import { Tooltip } from "./tooltip";
import { getNode, updateNode, type NodeDetail } from "../api";
import {
    nodeDotClass,
    nodeGlyphClass,
    resolveIconComponent,
    resolveNodeColor,
    statusLabel,
    statusTone,
} from "../node-presentation";
import { previewScope } from "../scope-preview";
import {
    PresentationControls,
    type PresentationSubview,
} from "./node-inspector-presentation";
import { RightRailPanel } from "./right-rail-panel";

const STATUSES = ["draft", "specified", "assigned", "in_progress", "done", "blocked"] as const;

export default function NodeInspector({
    nodeId,
    onClose,
    onSaved,
    onFocus,
}: {
    nodeId: string;
    onClose: () => void;
    onSaved: () => void;
    onFocus?: () => void;
}) {
    const uid = useId();
    const [node, setNode] = useState<NodeDetail | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [saving, setSaving] = useState(false);
    const [presentationSubview, setPresentationSubview] = useState<PresentationSubview>(null);
    const [presentationTouched, setPresentationTouched] = useState(false);

    useEffect(() => {
        const ac = new AbortController();
        setNode(null);
        setError(null);
        setPresentationSubview(null);
        setPresentationTouched(false);
        getNode(nodeId, ac.signal)
            .then(setNode)
            .catch((err: unknown) => {
                if (err instanceof DOMException && err.name === "AbortError") return;
                setError(err instanceof Error ? err.message : "node request failed");
            });
        return () => ac.abort();
    }, [nodeId]);

    async function save() {
        if (!node) return;
        setSaving(true);
        setError(null);
        try {
            const patch: Parameters<typeof updateNode>[1] = {
                name: node.name,
                type: node.type,
                status: node.status,
                implementation: lines(node.implementation),
                scope: lines(node.scope),
                protected: node.protected,
                markdown: node.markdown,
            };
            if (presentationTouched) {
                patch.icon = node.icon ?? "";
                patch.color = node.color ?? "";
                patch.renderer = node.renderer ?? "";
                patch.schema = (node.schema ?? []).filter((row) => row.title.trim() !== "" && row.type.trim() !== "");
            }
            const saved = await updateNode(node.id, patch);
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

    const closeButton = (
        <Tooltip label="Close inspector">
            <button
                type="button"
                aria-label="Close inspector"
                className="inline-flex size-8 shrink-0 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring"
                onClick={onClose}
            >
                <X className="size-4" aria-hidden />
            </button>
        </Tooltip>
    );

    return (
        <RightRailPanel className="min-h-[75vh] overflow-hidden" onPointerDown={() => onFocus?.()}>
            {!node || !preview ? (
                <div className="flex shrink-0 items-center justify-between">
                    <h2 className="font-semibold">Inspector</h2>
                    {closeButton}
                </div>
            ) : null}
            {error ? <p className="shrink-0 text-destructive">{error}</p> : null}
            {node && preview ? (
                <>
                    <div className="flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto p-1">
                        {presentationSubview ? (
                            <PresentationControls
                                subview={presentationSubview}
                                onSubview={setPresentationSubview}
                                backLabel="Inspector"
                                icon={node.icon}
                                color={node.color}
                                type={node.type}
                                renderer={node.renderer}
                                schema={node.schema}
                                onPickIcon={(id) => {
                                    markPresentation();
                                    setNode({ ...node, icon: id });
                                }}
                                onPickColor={(token) => {
                                    markPresentation();
                                    setNode({ ...node, color: token });
                                }}
                                onPickRenderer={(id) => {
                                    markPresentation();
                                    setNode({ ...node, renderer: id });
                                }}
                                onChangeSchema={(rows) => {
                                    markPresentation();
                                    setNode({ ...node, schema: rows });
                                }}
                            />
                        ) : (
                            <>
                                <div className="flex items-start gap-3">
                                    <span
                                        className={nodeGlyphClass(resolveNodeColor(node.type, node.color), "lg")}
                                        aria-hidden
                                    >
                                        {(() => {
                                            const Icon = resolveIconComponent(node.icon);
                                            return <Icon className="size-5" strokeWidth={2} />;
                                        })()}
                                    </span>
                                    <div className="flex min-w-0 flex-1 flex-col gap-1">
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
                                    {closeButton}
                                </div>
                                <PresentationControls
                                    subview={null}
                                    onSubview={setPresentationSubview}
                                    backLabel="Inspector"
                                    icon={node.icon}
                                    color={node.color}
                                    type={node.type}
                                    renderer={node.renderer}
                                    schema={node.schema}
                                    onPickIcon={() => { }}
                                    onPickColor={() => { }}
                                    onPickRenderer={() => { }}
                                    onChangeSchema={() => { }}
                                />
                                <div className="flex flex-col gap-2">
                                    <label htmlFor={`${uid}-status`} className={sectionLabelClass}>
                                        Status
                                    </label>
                                    <div className="flex items-center gap-2">
                                        <span
                                            className={nodeDotClass(statusTone(node.status))}
                                            aria-hidden
                                        />
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
                                <div className="flex flex-col gap-2">
                                    <label htmlFor={`${uid}-spec`} className={sectionLabelClass}>
                                        Spec
                                    </label>
                                    <textarea
                                        id={`${uid}-spec`}
                                        className={clsx(inputClass, "min-h-32 text-sm")}
                                        placeholder="What this node is responsible for."
                                        value={node.markdown}
                                        onChange={(event) => setNode({ ...node, markdown: event.target.value })}
                                    />
                                </div>
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
                            </>
                        )}
                    </div>
                    <div className="flex shrink-0 items-center justify-between gap-3">
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

function lines(value: string[]): string[] {
    return value.map((line) => line.trim()).filter((line) => line !== "");
}
