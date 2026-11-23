import clsx from "clsx";
import { ArrowRight, Lock, Minus, Pencil, Plus, Trash2 } from "lucide-react";
import type { ReactNode } from "react";
import { edgeKindStyle } from "../kind-style";
import { nodeDotClass, nodeGlyphClass, resolveIconComponent } from "../node-presentation";
import type { DetailView, EdgePreview, NodePreview, PreviewField } from "../proposal-detail";
import { KindGlyphIcon } from "./flow/kind-glyph";

type Tone = "normal" | "removed" | "new";

// Static look-alike of the canvas NodeCard header: glyph, name, type and status.
export function NodePreviewCard({
    node,
    tone = "normal",
    compact,
    muted,
    caption,
}: {
    node: NodePreview;
    tone?: Tone;
    compact?: boolean;
    muted?: boolean;
    caption?: ReactNode;
}) {
    const changed = (field: PreviewField) => node.changed.includes(field);
    const Icon = node.icon ? resolveIconComponent(node.icon) : null;
    const meta = [node.type, node.statusText].filter(Boolean).join(" · ");
    return (
        <div
            className={clsx(
                "relative min-w-0 rounded-[14px] border bg-card",
                tone === "removed" ? "border-destructive/60 bg-destructive/10" : tone === "new" ? "border-emerald-600/60" : "border-border",
                muted && "opacity-90",
            )}
        >
            <div className={clsx("flex items-center gap-2.5", compact ? "px-2.5 py-2" : "px-3 py-2.5")}>
                {Icon ? (
                    <span className={nodeGlyphClass(node.color)} aria-hidden>
                        <Icon className="size-4" strokeWidth={2} />
                    </span>
                ) : (
                    <span className={nodeDotClass(node.color)} aria-hidden />
                )}
                <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5">
                        <span
                            className={clsx(
                                "truncate text-sm font-medium text-foreground",
                                tone === "removed" && "line-through decoration-destructive",
                                changed("name") && "rounded bg-sky-500/20 px-1",
                            )}
                            title={node.name}
                        >
                            {node.name}
                        </span>
                        {node.protected ? <Lock className="size-3 shrink-0 text-muted-foreground" aria-hidden /> : null}
                    </div>
                    {meta ? (
                        <div className="truncate text-xs text-muted-foreground">
                            <span className={clsx(changed("type") && "rounded bg-sky-500/20 px-1 text-foreground")}>{node.type}</span>
                            {node.type && node.statusText ? " · " : ""}
                            <span className={clsx(changed("status") && "rounded bg-sky-500/20 px-1 text-foreground")}>{node.statusText}</span>
                        </div>
                    ) : !node.known ? (
                        <div className="text-xs text-muted-foreground">existing node</div>
                    ) : null}
                </div>
            </div>
            {caption ? <div className="border-t border-border px-3 py-1.5 text-xs text-muted-foreground">{caption}</div> : null}
        </div>
    );
}

function InsideLine({ node, highlight }: { node: NodePreview; highlight?: boolean }) {
    return (
        <p className="text-xs text-muted-foreground">
            {node.parentId ? (
                <>
                    inside{" "}
                    <span className={clsx("font-medium text-foreground", highlight && "rounded bg-sky-500/20 px-1")}>{node.parentName}</span>
                </>
            ) : (
                <span className={clsx(highlight && "rounded bg-sky-500/20 px-1 text-foreground")}>at the top level</span>
            )}
        </p>
    );
}

function Badge({ children, tone }: { children: ReactNode; tone: "new" | "removed" | "changed" }) {
    return (
        <span
            className={clsx(
                "inline-flex w-fit items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide",
                tone === "new" && "bg-emerald-500/15 text-emerald-800 dark:text-emerald-300",
                tone === "removed" && "bg-red-500/15 text-red-800 dark:text-red-300",
                tone === "changed" && "bg-sky-500/15 text-sky-800 dark:text-sky-300",
            )}
        >
            {children}
        </span>
    );
}

// Vertical edge between two stacked cards: line in the canvas edge colour, dashed for async.
export function EdgePreviewView({ edge, removed }: { edge: EdgePreview; removed?: boolean }) {
    const style = edgeKindStyle(edge.kind);
    return (
        <div className="flex items-center justify-center gap-2 py-1" aria-hidden>
            <svg width="16" height="44" viewBox="0 0 16 44" className={clsx(removed ? "text-destructive" : "text-[var(--edge)]")}>
                <line
                    x1="8"
                    y1="0"
                    x2="8"
                    y2="36"
                    stroke="currentColor"
                    strokeWidth="1.5"
                    strokeDasharray={style.dash}
                    opacity={removed ? 0.7 : 1}
                />
                <path d="M3 32l5 8 5-8z" fill="currentColor" />
            </svg>
            <span
                className={clsx(
                    "inline-flex max-w-full items-center gap-1.5 rounded-full border bg-card px-2 py-0.5 text-xs",
                    removed ? "border-destructive/60 text-foreground line-through decoration-destructive" : "border-border text-foreground",
                )}
            >
                <KindGlyphIcon glyph={style.glyph} className="size-3.5 shrink-0" />
                {edge.label ? <span className="min-w-0 break-words">{edge.label}</span> : null}
                <span className="shrink-0 text-muted-foreground">{style.label}</span>
            </span>
        </div>
    );
}

function Preview({ description, children }: { description: string; children: ReactNode }) {
    return (
        <figure className="m-0 flex flex-col gap-2 rounded-xl border border-border bg-background/40 p-3">
            <figcaption className="sr-only">{description}</figcaption>
            <div className="flex flex-col gap-2" aria-hidden>
                {children}
            </div>
        </figure>
    );
}

export function ProposalVisual({ view }: { view: DetailView }) {
    switch (view.kind) {
        case "create":
            return (
                <Preview description={view.description}>
                    <Badge tone="new">New node</Badge>
                    <NodePreviewCard node={view.node} tone="new" />
                    <InsideLine node={view.node} />
                    {view.notes.length ? (
                        <ul className="flex flex-col gap-0.5 text-xs text-muted-foreground">
                            {view.notes.map((note) => (
                                <li key={note}>{note}</li>
                            ))}
                        </ul>
                    ) : null}
                </Preview>
            );
        case "rules":
            return (
                <Preview description={view.description}>
                    <Badge tone="changed">Project rules</Badge>
                    {view.lines.length === 0 ? <p className="text-xs text-muted-foreground">{view.summary}</p> : null}
                    <ul className="flex flex-col gap-1.5 text-xs">
                        {view.lines.map((line, index) => (
                            <li key={index} className="flex items-start gap-1.5">
                                {line.kind === "added" ? (
                                    <Plus className="mt-0.5 size-3 shrink-0 text-emerald-700 dark:text-emerald-300" aria-hidden />
                                ) : line.kind === "removed" ? (
                                    <Minus className="mt-0.5 size-3 shrink-0 text-red-700 dark:text-red-300" aria-hidden />
                                ) : (
                                    <Pencil className="mt-0.5 size-3 shrink-0 text-sky-700 dark:text-sky-300" aria-hidden />
                                )}
                                <span className="min-w-0">
                                    <span className="font-medium text-foreground">
                                        {line.kind === "added" ? "Added" : line.kind === "removed" ? "Removed" : "Changed"}:
                                    </span>{" "}
                                    <span className={clsx("text-foreground", line.kind === "removed" && "line-through decoration-destructive")}>{line.text}</span>
                                    {line.was ? <span className="block text-muted-foreground">was {line.was}</span> : null}
                                </span>
                            </li>
                        ))}
                    </ul>
                </Preview>
            );
        case "connect":
            return (
                <Preview description={view.description}>
                    <Badge tone={view.previous ? "changed" : "new"}>{view.previous ? "Changed connection" : "New connection"}</Badge>
                    <NodePreviewCard node={view.from} compact />
                    <EdgePreviewView edge={view.edge} />
                    <NodePreviewCard node={view.to} compact />
                    {view.edge.contract ? <p className="text-xs text-foreground">Contract: {view.edge.contract}</p> : null}
                    {view.previous ? (
                        <p className="text-xs text-muted-foreground">
                            Was: {view.previous.label || "(no label)"} · {edgeKindStyle(view.previous.kind).label}
                            {view.previous.contract ? ` · contract ${view.previous.contract}` : ""}
                        </p>
                    ) : null}
                </Preview>
            );
        case "update":
            return (
                <Preview description={view.description}>
                    <Badge tone="changed">Update</Badge>
                    <div className="flex flex-col gap-3">
                        <div className="flex min-w-0 flex-col gap-1">
                            <p className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">Current</p>
                            <NodePreviewCard node={{ ...view.before, changed: [] }} muted />
                            <InsideLine node={view.before} />
                        </div>
                        <div className="flex min-w-0 flex-col gap-1">
                            <p className="flex items-center gap-1 text-[11px] font-semibold uppercase tracking-wide text-foreground">
                                <ArrowRight className="size-3" aria-hidden /> Proposed
                            </p>
                            <NodePreviewCard node={view.after} />
                            <InsideLine node={view.after} highlight={view.after.changed.includes("parent")} />
                        </div>
                    </div>
                    <ul className="flex flex-col gap-1 text-xs">
                        {view.changes.map((c) => (
                            <li key={c.field} className="flex flex-wrap gap-x-1.5">
                                <span className="font-medium text-foreground">{c.label}</span>
                                <span className="text-muted-foreground line-through">{c.before}</span>
                                <span aria-hidden>→</span>
                                <span className="sr-only">changed to</span>
                                <span className="text-foreground">{c.after}</span>
                            </li>
                        ))}
                        {view.specSummary ? (
                            <li className="text-foreground">{view.specSummary}</li>
                        ) : (
                            <li className="text-muted-foreground">Spec unchanged</li>
                        )}
                    </ul>
                </Preview>
            );
        case "remove-node":
            return (
                <Preview description={view.description}>
                    <Badge tone="removed">
                        <Trash2 className="size-3" aria-hidden /> Removed
                    </Badge>
                    <NodePreviewCard node={view.node} tone="removed" />
                    <InsideLine node={view.node} />
                </Preview>
            );
        case "remove-connection":
            return (
                <Preview description={view.description}>
                    <Badge tone="removed">
                        <Trash2 className="size-3" aria-hidden /> Removed connection
                    </Badge>
                    <NodePreviewCard node={view.from} compact />
                    <EdgePreviewView edge={view.edge} removed />
                    <NodePreviewCard node={view.to} compact />
                    {view.interior.length ? (
                        <div className="flex flex-col gap-1.5">
                            <p className="text-xs font-medium text-foreground">Also removed (inside this connection)</p>
                            {view.interior.map((n) => (
                                <NodePreviewCard key={n.id} node={n} tone="removed" compact />
                            ))}
                        </div>
                    ) : null}
                </Preview>
            );
        default:
            return null;
    }
}
