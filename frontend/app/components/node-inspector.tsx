import { useEffect, useState } from "react";
import { getNode, updateNode, type NodeDetail } from "../api";
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

    return (
        <RightRailPanel className="min-h-[75vh] overflow-hidden" onPointerDown={() => onFocus?.()}>
            <div className="flex items-center justify-between shrink-0">
                <h2 className="font-semibold">Inspector</h2>
                <button type="button" className="text-xs" onClick={onClose}>
                    Close
                </button>
            </div>
            {error ? <p className="shrink-0 text-red-600">{error}</p> : null}
            {node && preview ? (
                <>
                    <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto">
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
                                <label className="flex flex-col gap-1">
                                    Name
                                    <input
                                        className="rounded border border-foreground/20 bg-background px-2 py-1"
                                        value={node.name}
                                        onChange={(event) => setNode({ ...node, name: event.target.value })}
                                    />
                                </label>
                                <label className="flex flex-col gap-1">
                                    Type
                                    <input
                                        className="rounded border border-foreground/20 bg-background px-2 py-1"
                                        value={node.type}
                                        onChange={(event) => setNode({ ...node, type: event.target.value })}
                                    />
                                </label>
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
                                <label className="flex flex-col gap-1">
                                    Status
                                    <select
                                        className="rounded border border-foreground/20 bg-background px-2 py-1"
                                        value={node.status}
                                        onChange={(event) => setNode({ ...node, status: event.target.value })}
                                    >
                                        {statusOptions(node.status).map((status) => (
                                            <option key={status} value={status}>
                                                {status}
                                            </option>
                                        ))}
                                    </select>
                                </label>
                                <label className="flex flex-col gap-1">
                                    Implementation
                                    <textarea
                                        className="min-h-16 rounded border border-foreground/20 bg-background px-2 py-1 font-mono text-xs"
                                        value={node.implementation.join("\n")}
                                        onChange={(event) =>
                                            setNode({ ...node, implementation: event.target.value.split("\n") })
                                        }
                                    />
                                </label>
                                <label className="flex flex-col gap-1">
                                    Scope
                                    <textarea
                                        className="min-h-16 rounded border border-foreground/20 bg-background px-2 py-1 font-mono text-xs"
                                        value={node.scope.join("\n")}
                                        onChange={(event) =>
                                            setNode({ ...node, scope: event.target.value.split("\n") })
                                        }
                                    />
                                </label>
                                {preview.defaultsToImplementation ? (
                                    <p>Empty scope defaults to implementation.</p>
                                ) : (
                                    <p>Scope covers the globs written above.</p>
                                )}
                                <p className="font-mono text-xs text-foreground/70">
                                    {preview.globs.length > 0 ? preview.globs.join(", ") : "(no globs)"}
                                </p>
                                <label className="flex items-center gap-2">
                                    <input
                                        type="checkbox"
                                        className="m-0 size-4 shrink-0"
                                        checked={node.protected}
                                        onChange={(event) => setNode({ ...node, protected: event.target.checked })}
                                    />
                                    <span className="leading-none">Protected applies regardless of assignment.</span>
                                </label>
                                <label className="flex flex-col gap-1">
                                    Markdown
                                    <textarea
                                        className="min-h-32 rounded border border-foreground/20 bg-background px-2 py-1 font-mono text-xs"
                                        value={node.markdown}
                                        onChange={(event) => setNode({ ...node, markdown: event.target.value })}
                                    />
                                </label>
                            </>
                        )}
                    </div>
                    <button
                        type="button"
                        className="shrink-0 rounded bg-primary text-primary-foreground px-3 py-1"
                        disabled={saving}
                        onClick={() => {
                            void save();
                        }}
                    >
                        {saving ? "Saving" : "Save"}
                    </button>
                </>
            ) : null}
        </RightRailPanel>
    );
}

function statusOptions(current: string): readonly string[] {
    if ((STATUSES as readonly string[]).includes(current)) return STATUSES;
    return [current, ...STATUSES];
}

function lines(value: string[]): string[] {
    return value.map((line) => line.trim()).filter((line) => line !== "");
}
