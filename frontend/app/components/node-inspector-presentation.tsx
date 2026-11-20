import type { ReactNode } from "react";
import { Tooltip } from "./tooltip";
import { ChevronLeft } from "lucide-react";
import clsx from "clsx";
import type { SchemaColumn } from "../api";
import { RENDERER_IDS } from "../canvas-presentation";
import {
    COLOR_TOKENS,
    ICON_GROUPS,
    defaultColorForType,
    nodeSwatchClass,
    resolveIconComponent,
} from "../node-presentation";

export type PresentationSubview = "icon" | "color" | "renderer" | null;

const RENDERER_LABELS: Record<(typeof RENDERER_IDS)[number], string> = {
    card: "Card",
    schema: "Database schema",
    placeholder: "Placeholder",
    group: "Labeled group",
};

export function PresentationControls({
    subview,
    onSubview,
    backLabel = "Back",
    icon,
    color,
    type,
    renderer,
    schema,
    onPickIcon,
    onPickColor,
    onPickRenderer,
    onChangeSchema,
}: {
    subview: PresentationSubview;
    onSubview: (next: PresentationSubview) => void;
    backLabel?: string;
    icon?: string;
    color?: string;
    type: string;
    renderer?: string;
    schema?: SchemaColumn[];
    onPickIcon: (id: string) => void;
    onPickColor: (token: string) => void;
    onPickRenderer: (id: string) => void;
    onChangeSchema: (rows: SchemaColumn[]) => void;
}) {
    if (subview === "icon") {
        return (
            <PresentationSubviewPanel title="Icon" backLabel={backLabel} onBack={() => onSubview(null)}>
                <button
                    type="button"
                    className="rounded border border-border px-2 py-1 text-left text-xs"
                    onClick={() => onPickIcon("")}
                >
                    Default (colour dot)
                </button>
                {ICON_GROUPS.map((group) => (
                    <div key={group.label} className="flex flex-col gap-1">
                        <span className="text-[10px] font-medium uppercase tracking-wide text-foreground/60">
                            {group.label}
                        </span>
                        <div className="grid grid-cols-5 gap-1">
                            {group.ids.map((id) => {
                                const Icon = resolveIconComponent(id);
                                return (
                                    <Tooltip key={id} label={id}>
                                        <button
                                            type="button"
                                            aria-label={id}
                                            aria-pressed={icon === id}
                                            className={clsx(
                                                "flex items-center justify-center rounded border p-2",
                                                icon === id
                                                    ? "border-primary bg-accent"
                                                    : "border-border",
                                            )}
                                            onClick={() => onPickIcon(id)}
                                        >
                                            <Icon className="size-4" aria-hidden />
                                        </button>
                                    </Tooltip>
                                );
                            })}
                        </div>
                    </div>
                ))}
            </PresentationSubviewPanel>
        );
    }

    if (subview === "color") {
        const typeDefault = defaultColorForType(type);
        return (
            <PresentationSubviewPanel title="Colour" backLabel={backLabel} onBack={() => onSubview(null)}>
                <button
                    type="button"
                    className="rounded border border-border px-2 py-1 text-left text-xs"
                    onClick={() => onPickColor("")}
                >
                    Default (from type: {typeDefault})
                </button>
                <div className="grid grid-cols-2 gap-1.5">
                    {COLOR_TOKENS.map((token) => (
                        <button
                            key={token}
                            type="button"
                            className={clsx(
                                "flex items-center gap-2 rounded border px-2 py-1.5 text-xs capitalize",
                                color === token
                                    ? "border-primary bg-accent"
                                    : "border-border",
                            )}
                            onClick={() => onPickColor(token)}
                        >
                            <span className={nodeSwatchClass(token)} aria-hidden />
                            {token}
                        </button>
                    ))}
                </div>
            </PresentationSubviewPanel>
        );
    }

    if (subview === "renderer") {
        const rows = schema ?? [];
        const selected = renderer || "card";
        return (
            <PresentationSubviewPanel title="Canvas" backLabel={backLabel} onBack={() => onSubview(null)}>
                <div className="flex flex-col gap-1">
                    {RENDERER_IDS.map((id) => (
                        <button
                            key={id}
                            type="button"
                            className={clsx(
                                "rounded border px-2 py-1 text-left text-xs",
                                selected === id ? "border-primary bg-accent" : "border-border",
                            )}
                            onClick={() => onPickRenderer(id === "card" ? "" : id)}
                        >
                            {RENDERER_LABELS[id]}
                        </button>
                    ))}
                </div>
                {selected === "schema" ? (
                    <div className="flex flex-col gap-1.5">
                        <span className="text-[10px] font-medium uppercase tracking-wide text-foreground/60">
                            Columns
                        </span>
                        {rows.map((row, index) => (
                            <div key={index} className="flex gap-1">
                                <input
                                    className="min-w-0 flex-1 rounded border border-border bg-background px-1.5 py-1 text-xs"
                                    placeholder="title"
                                    value={row.title}
                                    onChange={(event) => {
                                        const next = rows.slice();
                                        next[index] = { ...row, title: event.target.value };
                                        onChangeSchema(next);
                                    }}
                                />
                                <input
                                    className="min-w-0 flex-1 rounded border border-border bg-background px-1.5 py-1 text-xs"
                                    placeholder="type"
                                    value={row.type}
                                    onChange={(event) => {
                                        const next = rows.slice();
                                        next[index] = { ...row, type: event.target.value };
                                        onChangeSchema(next);
                                    }}
                                />
                                <button
                                    type="button"
                                    className="rounded border border-border px-1.5 text-xs"
                                    onClick={() => onChangeSchema(rows.filter((_, rowIndex) => rowIndex !== index))}
                                >
                                    ×
                                </button>
                            </div>
                        ))}
                        <button
                            type="button"
                            className="rounded border border-border px-2 py-1 text-left text-xs"
                            onClick={() => onChangeSchema([...rows, { title: "", type: "" }])}
                        >
                            Add column
                        </button>
                    </div>
                ) : null}
            </PresentationSubviewPanel>
        );
    }

    const rendererLabel =
        !renderer || renderer === "card"
            ? ""
            : (RENDERER_LABELS[renderer as keyof typeof RENDERER_LABELS] ?? renderer);

    return (
        <div className="grid grid-cols-2 gap-1.5">
            <button
                type="button"
                className="rounded border border-border px-2 py-1 text-xs w-full"
                onClick={() => onSubview("icon")}
            >
                Icon{icon ? `: ${icon}` : ""}
            </button>
            <button
                type="button"
                className="rounded border border-border px-2 py-1 text-xs w-full"
                onClick={() => onSubview("color")}
            >
                Colour{color ? `: ${color}` : ""}
            </button>
            <button
                type="button"
                className="col-span-2 rounded border border-border px-2 py-1 text-xs w-full"
                onClick={() => onSubview("renderer")}
            >
                Canvas{rendererLabel ? `: ${rendererLabel}` : ""}
            </button>
        </div>
    );
}

function PresentationSubviewPanel({
    title,
    backLabel,
    onBack,
    children,
}: {
    title: string;
    backLabel: string;
    onBack: () => void;
    children: ReactNode;
}) {
    return (
        <div className="animate-in slide-in-from-right-4 flex min-h-0 flex-1 flex-col gap-3 duration-200 border p-1.5 rounded-md">
            <div className="flex items-center gap-2">
                <button
                    type="button"
                    className="inline-flex items-center gap-1 text-xs text-foreground/80"
                    onClick={onBack}
                >
                    <ChevronLeft className="size-4" aria-hidden />
                    {backLabel}
                </button>
                <span className="text-xs font-medium">{title}</span>
            </div>
            <div className="flex min-h-0 flex-1 flex-col gap-3">{children}</div>
        </div>
    );
}
