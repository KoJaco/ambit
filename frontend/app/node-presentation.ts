import type { LucideIcon } from "lucide-react";
import {
    Archive,
    BookOpen,
    Box,
    Circle,
    Cloud,
    Component,
    Cpu,
    Database,
    FileText,
    GitBranch,
    Globe,
    HardDrive,
    Hexagon,
    Layers,
    Lock,
    Network,
    Package,
    ScrollText,
    Server,
    Share2,
    Shield,
    StickyNote,
    Table,
    Users,
    Workflow,
} from "lucide-react";

export const COLOR_TOKENS = [
    "neutral",
    "slate",
    "blue",
    "teal",
    "green",
    "amber",
    "violet",
    "rose",
] as const;

export type ColorToken = (typeof COLOR_TOKENS)[number];

const TYPE_COLOR_MAP: Record<string, ColorToken> = {
    service: "blue",
    module: "teal",
    component: "violet",
    package: "slate",
    note: "amber",
    doc: "amber",
    adr: "amber",
    database: "green",
    cache: "green",
    queue: "green",
    saas: "rose",
    external: "rose",
};

export const ICON_GROUPS: { label: string; ids: string[] }[] = [
    { label: "Structure", ids: ["box", "hexagon", "layers", "component", "package"] },
    { label: "Compute", ids: ["server", "cpu", "cloud", "globe", "workflow"] },
    { label: "Data", ids: ["database", "hard-drive", "archive", "table"] },
    { label: "Documents", ids: ["file-text", "scroll-text", "book-open", "sticky-note"] },
    { label: "Access", ids: ["users", "shield", "lock"] },
    { label: "Flow", ids: ["git-branch", "share-2", "network"] },
];

const ICON_COMPONENTS: Record<string, LucideIcon> = {
    box: Box,
    hexagon: Hexagon,
    layers: Layers,
    component: Component,
    package: Package,
    server: Server,
    cpu: Cpu,
    cloud: Cloud,
    globe: Globe,
    workflow: Workflow,
    database: Database,
    "hard-drive": HardDrive,
    archive: Archive,
    table: Table,
    "file-text": FileText,
    "scroll-text": ScrollText,
    "book-open": BookOpen,
    "sticky-note": StickyNote,
    users: Users,
    shield: Shield,
    lock: Lock,
    "git-branch": GitBranch,
    "share-2": Share2,
    network: Network,
};

export function defaultColorForType(type: string): ColorToken {
    const key = type.trim().toLowerCase();
    return TYPE_COLOR_MAP[key] ?? "neutral";
}

export function resolveNodeColor(type: string, storedColor?: string): ColorToken {
    if (storedColor && (COLOR_TOKENS as readonly string[]).includes(storedColor)) {
        return storedColor as ColorToken;
    }
    return defaultColorForType(type);
}

export function resolveIconComponent(iconId?: string): LucideIcon {
    if (iconId && ICON_COMPONENTS[iconId]) {
        return ICON_COMPONENTS[iconId];
    }
    return Circle;
}

export function nodeGlyphClass(token: ColorToken): string {
    return `node-glyph node-glyph-${token}`;
}

export function nodeDotClass(token: ColorToken): string {
    return `node-dot node-dot-${token}`;
}

export function nodeSwatchClass(token: ColorToken): string {
    return `node-swatch node-swatch-${token}`;
}
