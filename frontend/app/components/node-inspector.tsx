import { useEffect, useState } from "react";
import { getNode, updateNode, type NodeDetail } from "../api";
import { previewScope } from "../scope-preview";

const STATUSES = ["draft", "specified", "assigned", "in_progress", "done", "blocked"] as const;

export default function NodeInspector({
    nodeId,
    onClose,
    onSaved,
}: {
    nodeId: string;
    onClose: () => void;
    onSaved: () => void;
}) {
    const [node, setNode] = useState<NodeDetail | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [saving, setSaving] = useState(false);

    useEffect(() => {
        const ac = new AbortController();
        setNode(null);
        setError(null);
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
            const saved = await updateNode(node.id, {
                name: node.name,
                type: node.type,
                status: node.status,
                implementation: lines(node.implementation),
                scope: lines(node.scope),
                protected: node.protected,
                markdown: node.markdown,
            });
            setNode(saved);
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

    return (
        <aside className="pointer-events-auto absolute top-0 right-0 flex max-h-full w-80 flex-col gap-3 overflow-y-auto rounded-xl border border-foreground/25 bg-background/95 p-4 text-sm shadow-sm backdrop-blur">
            <div className="flex items-center justify-between">
                <h2 className="font-semibold">Inspector</h2>
                <button type="button" className="text-xs" onClick={onClose}>
                    Close
                </button>
            </div>
            {error ? <p className="text-red-600">{error}</p> : null}
            {node && preview ? (
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
                            onChange={(event) => setNode({ ...node, scope: event.target.value.split("\n") })}
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
                    <button
                        type="button"
                        className="rounded border border-foreground/30 px-3 py-1"
                        disabled={saving}
                        onClick={() => {
                            void save();
                        }}
                    >
                        {saving ? "Saving" : "Save"}
                    </button>
                </>
            ) : null}
        </aside>
    );
}

function statusOptions(current: string): readonly string[] {
    if ((STATUSES as readonly string[]).includes(current)) return STATUSES;
    return [current, ...STATUSES];
}

function lines(value: string[]): string[] {
    return value.map((line) => line.trim()).filter((line) => line !== "");
}
