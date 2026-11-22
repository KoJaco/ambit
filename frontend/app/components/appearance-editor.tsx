import clsx from "clsx";
import { Check } from "lucide-react";
import type { SchemaColumn } from "../api";
import { RENDERER_IDS } from "../canvas-presentation";
import {
    COLOR_TOKENS,
    ICON_GROUPS,
    defaultColorForType,
    nodeDotClass,
    nodeGlyphClass,
    nodeSwatchClass,
    resolveIconComponent,
    resolveNodeColor,
} from "../node-presentation";
import { Tooltip } from "./tooltip";

export type Appearance = { icon?: string; color?: string; renderer?: string };

export const RENDERER_LABELS: Record<(typeof RENDERER_IDS)[number], string> = {
    card: "Card",
    schema: "Database schema",
    placeholder: "Placeholder",
    group: "Labeled group",
};

const focusRing = "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring";
const sectionLabel = "text-[11px] font-medium uppercase tracking-wide text-muted-foreground";

// Collapsed-state summary for the Appearance section: current icon tile (or colour dot) and renderer.
export function AppearanceSummary({ value, type }: { value: Appearance; type: string }) {
    const token = resolveNodeColor(type, value.color);
    const Icon = resolveIconComponent(value.icon);
    const renderer =
        !value.renderer || value.renderer === "card"
            ? "Card"
            : (RENDERER_LABELS[value.renderer as keyof typeof RENDERER_LABELS] ?? value.renderer);
    return (
        <span className="flex items-center gap-2 text-xs text-muted-foreground">
            {value.icon ? (
                <span className={nodeGlyphClass(token)} style={{ width: "1.25rem", height: "1.25rem", borderRadius: "0.375rem" }} aria-hidden>
                    <Icon className="size-3" strokeWidth={2} />
                </span>
            ) : (
                <span className={nodeDotClass(token)} aria-hidden />
            )}
            <span>{value.icon ?? "Default icon"}</span>
            <span aria-hidden>·</span>
            <span>{renderer}</span>
        </span>
    );
}

// Icon, colour and canvas renderer pickers. Empty string means "default" for each field.
// Lives inside a CollapsibleSection, which owns the heading and scrolling.
export function AppearanceFields({
    value,
    onChange,
    type = "",
    schema,
    onChangeSchema,
}: {
    value: Appearance;
    onChange: (next: Appearance) => void;
    type?: string;
    schema?: SchemaColumn[];
    onChangeSchema?: (rows: SchemaColumn[]) => void;
}) {
    const selectedRenderer = value.renderer || "card";
    const rows = schema ?? [];
    const set = (patch: Appearance) => onChange({ ...value, ...patch });
    const optionClass = (on: boolean) =>
        clsx(
            "rounded-[10px] border px-2 py-1.5 text-left text-xs",
            on ? "border-primary bg-accent" : "border-border hover:bg-accent",
            focusRing,
        );

    return (
        <div className="flex flex-col gap-4">
            <section className="flex flex-col gap-2" aria-label="Icon">
                <span className={sectionLabel}>Icon</span>
                <button type="button" aria-pressed={!value.icon} className={optionClass(!value.icon)} onClick={() => set({ icon: "" })}>
                    Default (colour dot)
                </button>
                {ICON_GROUPS.map((group) => (
                    <div key={group.label} className="flex flex-col gap-1">
                        <span className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">{group.label}</span>
                        <div className="grid grid-cols-6 gap-1">
                            {group.ids.map((id) => {
                                const Icon = resolveIconComponent(id);
                                return (
                                    <Tooltip key={id} label={id}>
                                        <button
                                            type="button"
                                            aria-label={id}
                                            aria-pressed={value.icon === id}
                                            className={clsx(
                                                "flex min-h-8 items-center justify-center rounded-[8px] border p-1.5",
                                                value.icon === id ? "border-primary bg-accent" : "border-border hover:bg-accent",
                                                focusRing,
                                            )}
                                            onClick={() => set({ icon: id })}
                                        >
                                            <Icon className="size-4" aria-hidden />
                                        </button>
                                    </Tooltip>
                                );
                            })}
                        </div>
                    </div>
                ))}
            </section>

            <section className="flex flex-col gap-2" aria-label="Colour">
                <span className={sectionLabel}>Colour</span>
                <button type="button" aria-pressed={!value.color} className={optionClass(!value.color)} onClick={() => set({ color: "" })}>
                    Default (from type: {defaultColorForType(type)})
                </button>
                <div className="grid grid-cols-3 gap-1.5">
                    {COLOR_TOKENS.map((token) => (
                        <button
                            key={token}
                            type="button"
                            aria-pressed={value.color === token}
                            className={clsx(
                                "flex min-h-8 items-center gap-1.5 rounded-[10px] border px-1.5 py-1 text-[11px] capitalize",
                                value.color === token ? "border-primary bg-accent" : "border-border hover:bg-accent",
                                focusRing,
                            )}
                            onClick={() => set({ color: token })}
                        >
                            <span className={nodeSwatchClass(token)} aria-hidden />
                            <span className="truncate">{token}</span>
                            {value.color === token ? <Check className="ml-auto size-3 shrink-0" aria-hidden /> : null}
                        </button>
                    ))}
                </div>
            </section>

            <section className="flex flex-col gap-2" aria-label="Canvas">
                <span className={sectionLabel}>Canvas</span>
                <div className="grid grid-cols-2 gap-1.5">
                    {RENDERER_IDS.map((id) => (
                        <button
                            key={id}
                            type="button"
                            aria-pressed={selectedRenderer === id}
                            className={optionClass(selectedRenderer === id)}
                            onClick={() => set({ renderer: id === "card" ? "" : id })}
                        >
                            {RENDERER_LABELS[id]}
                        </button>
                    ))}
                </div>
                {selectedRenderer === "schema" && onChangeSchema ? (
                    <div className="flex flex-col gap-1.5">
                        <span className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">Columns</span>
                        {rows.map((row, index) => (
                            <div key={index} className="flex gap-1">
                                <input
                                    aria-label={`Column ${index + 1} title`}
                                    className="min-w-0 flex-1 rounded-md border border-border bg-background px-1.5 py-1 text-xs"
                                    placeholder="title"
                                    value={row.title}
                                    onChange={(event) => {
                                        const next = rows.slice();
                                        next[index] = { ...row, title: event.target.value };
                                        onChangeSchema(next);
                                    }}
                                />
                                <input
                                    aria-label={`Column ${index + 1} type`}
                                    className="min-w-0 flex-1 rounded-md border border-border bg-background px-1.5 py-1 text-xs"
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
                                    aria-label={`Remove column ${index + 1}`}
                                    className={clsx("min-w-6 rounded-md border border-border px-1.5 text-xs", focusRing)}
                                    onClick={() => onChangeSchema(rows.filter((_, rowIndex) => rowIndex !== index))}
                                >
                                    ×
                                </button>
                            </div>
                        ))}
                        <button type="button" className={optionClass(false)} onClick={() => onChangeSchema([...rows, { title: "", type: "" }])}>
                            Add column
                        </button>
                    </div>
                ) : null}
            </section>
        </div>
    );
}
