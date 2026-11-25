import type { LucideIcon } from "lucide-react";
import {
    Archive,
    Bell,
    BookOpen,
    Bot,
    Box,
    Cable,
    Circle,
    Clock,
    Cloud,
    Code,
    Component,
    Cpu,
    CreditCard,
    Database,
    FileText,
    Folder,
    GitBranch,
    Globe,
    HardDrive,
    Hexagon,
    KeyRound,
    Layers,
    Lock,
    Mail,
    MessageSquare,
    Monitor,
    Network,
    Package,
    ScrollText,
    Search,
    Server,
    Settings,
    Share2,
    Shield,
    Smartphone,
    StickyNote,
    Table,
    Terminal,
    Users,
    Workflow,
    Zap,
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
    { label: "Flow", ids: ["git-branch", "share-2", "network", "cable", "zap", "clock"] },
    { label: "Clients", ids: ["smartphone", "monitor", "terminal", "code", "bot"] },
    { label: "Services", ids: ["key-round", "mail", "bell", "message-square", "credit-card", "search", "settings", "folder"] },
];

// Human labels from GET /icons, keyed by icon id.
export function iconLabelMap(icons: { id: string; label: string }[] | undefined): Record<string, string> {
    const labels: Record<string, string> = {};
    for (const icon of icons ?? []) if (icon.label.trim() !== "") labels[icon.id] = icon.label;
    return labels;
}

// Label for the picker button: the registry label, or the id when the registry is unavailable.
export function iconLabel(id: string, labels: Record<string, string>): string {
    return labels[id] || id;
}

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
    bell: Bell,
    bot: Bot,
    cable: Cable,
    clock: Clock,
    "credit-card": CreditCard,
    code: Code,
    folder: Folder,
    "key-round": KeyRound,
    mail: Mail,
    "message-square": MessageSquare,
    monitor: Monitor,
    search: Search,
    settings: Settings,
    smartphone: Smartphone,
    terminal: Terminal,
    zap: Zap,
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

export function nodeGlyphClass(token: ColorToken, size: "md" | "lg" = "md"): string {
    return size === "lg" ? `node-glyph node-glyph-lg node-glyph-${token}` : `node-glyph node-glyph-${token}`;
}

export function nodeDotClass(token: ColorToken): string {
    return `node-dot node-dot-${token}`;
}

export function nodeSwatchClass(token: ColorToken): string {
    return `node-swatch node-swatch-${token}`;
}

const STATUS_LABELS: Record<string, string> = {
    draft: "Draft",
    specified: "Specified",
    assigned: "Assigned",
    in_progress: "In progress",
    needs_review: "Needs review",
    done: "Done",
    blocked: "Blocked",
};

export function statusLabel(status: string): string {
    return STATUS_LABELS[status] ?? status;
}

export function statusTone(status: string): ColorToken {
    switch (status) {
        case "in_progress":
        case "assigned":
            return "blue";
        case "needs_review":
            return "amber";
        case "done":
            return "teal";
        case "blocked":
            return "rose";
        case "specified":
            return "violet";
        default:
            return "neutral";
    }
}
