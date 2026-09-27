import clsx from "clsx";
import styles from "./Sidebar.module.css";
import React from "react";
import { useMemo, useState } from "react";
import { useUIContext } from "./ui-context";
import {
    ActivityIcon,
    AudioLinesIcon,
    BracesIcon,
    ChevronDownIcon,
    ChevronLeftIcon,
    ChevronRightIcon,
    ChevronUpIcon,
    ClockIcon,
    CodeIcon,
    CombineIcon,
    DatabaseIcon,
    ExternalLinkIcon,
    LaptopMinimalIcon,
    MailIcon,
    MergeIcon,
    RepeatIcon,
    SparklesIcon,
    SquareIcon,
    StickyNoteIcon,
} from "lucide-react";

type SidebarSection = {
    title: string;
    items: { kind: string; label: string }[];
};

export function Sidebar() {
    const [open, setOpen] = useState(false);
    const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
    const { pendingNodeId, convertPending } = useUIContext();

    const sections = useMemo<SidebarSection[]>(() => {
        const groups: Record<string, { kind: string; label: string }[]> = {};
        Object.entries(NodeRegistry).forEach(([kind, def]) => {
            const title = def.category;
            if (!groups[title]) groups[title] = [];
            groups[title].push({ kind, label: def.label });
        });
        const order = [
            "Tools",
            "LLM",
            "Logic",
            "Data",
            "Integrations",
            "Special",
        ];
        return order
            .filter((title) => groups[title]?.length)
            .map((title) => ({ title, items: groups[title] }));
    }, []);

    const toggleSidebar = () => {
        setOpen(!open);
    };

    const categoryStyles: Record<string, { bg: string; text: string }> = {
        Tools: { bg: "bg-yellow-400", text: "text-yellow-950" },
        LLM: { bg: "bg-indigo-400", text: "text-indigo-950" },
        Logic: { bg: "bg-orange-400", text: "text-orange-950" },
        Data: { bg: "bg-sky-400", text: "text-sky-950" },
        Integrations: { bg: "bg-emerald-400", text: "text-emerald-950" },
        Special: { bg: "bg-rose-400", text: "text-rose-950" },
    };

    function getIconFor(kind: string, category: string) {
        const { bg, text } = categoryStyles[category] || {
            bg: "bg-foreground/10",
            text: "text-foreground",
        };
        const wrap = (n: React.ReactNode) => (
            <span className={clsx(bg, "p-2 rounded-lg", text)}>{n}</span>
        );
        const small = "w-3 h-3";
        switch (kind) {
            // Tools
            case "STT":
                return wrap(<AudioLinesIcon className={small} />);
            case "Chunker":
                return wrap(<SquareIcon className={small} />);
            case "EnrichText":
                return wrap(<SparklesIcon className={small} />);
            case "EnrichAudio":
                return wrap(<ActivityIcon className={small} />);
            case "Validator":
                return wrap(<BracesIcon className={small} />);
            // LLM
            case "StructuredOutput":
                return wrap(<BracesIcon className={small} />);
            case "FunctionCall":
                return wrap(<CodeIcon className={small} />);
            case "Summarize":
                return wrap(<StickyNoteIcon className={small} />);
            // Logic
            case "If":
                return wrap(<BracesIcon className={small} />);
            case "Filter":
                return wrap(<BracesIcon className={small} />);
            case "Throttle":
                return wrap(<ClockIcon className={small} />);
            case "Retry":
                return wrap(<RepeatIcon className={small} />);
            case "Fork":
                return wrap(<MergeIcon className={small} />);
            case "Join":
                return wrap(<CombineIcon className={small} />);
            // Data
            case "Transform":
                return wrap(<MergeIcon className={small} />);
            case "State":
                return wrap(<ActivityIcon className={small} />);
            case "ContextComposer":
                return wrap(<CombineIcon className={small} />);
            case "RAGFetch":
                return wrap(<ExternalLinkIcon className={small} />);
            // Integrations
            case "HTTP":
                return wrap(<ExternalLinkIcon className={small} />);
            case "N8N":
                return wrap(<ExternalLinkIcon className={small} />);
            case "Zapier":
                return wrap(<ExternalLinkIcon className={small} />);
            case "DBWrite":
                return wrap(<DatabaseIcon className={small} />);
            case "UIStream":
                return wrap(<LaptopMinimalIcon className={small} />);
            case "Webhook":
                return wrap(<MailIcon className={small} />);
            case "Queue":
                return wrap(<SquareIcon className={small} />);
            // Special
            case "OnEnd":
                return wrap(<SquareIcon className={small} />);
            default:
                return wrap(<SquareIcon className={small} />);
        }
    }

    return (
        <>
            {open ? (
                <button
                    className="absolute rounded-r-lg bg-primary text-primary-foreground h-[80px] w-[14px] left-0 z-100 bottom-1/2 translate-y-1/2"
                    onClick={toggleSidebar}
                    title="Open sidebar"
                    aria-label="Open sidebar"
                >
                    {/* » */}
                    <span className="text-xs">»</span>
                </button>
            ) : (
                <aside
                    className={clsx(
                        "absolute w-[200px] overflow-y-auto z-100 left-4 translate-y-1/2 bottom-1/2 h-auto min-h-[75vh] max-h-[90vh] border rounded-lg bg-card border-foreground/25 border-r",
                        pendingNodeId && "ring-2 ring-offset-2 ring-primary",
                        styles.sidebarScroll,
                        styles.sidebarScrollFade
                    )}
                    data-pending-selection={!!pendingNodeId}
                >
                    <div className="h-full relative">
                        <div className="p-2 pb-0">
                            <button
                                onClick={toggleSidebar}
                                title="Close sidebar"
                                aria-label="Close sidebar"
                            >
                                <span className="text-lg">«</span>
                            </button>
                        </div>
                        <div className="p-2 space-y-4">
                            {/* Sections */}
                            <div className="space-y-4">
                                {sections.map((section) => (
                                    <div key={section.title}>
                                        <button
                                            className="w-full flex items-center justify-between px-1 py-1 text-[10px] font-semibold uppercase tracking-wider text-foreground hover:bg-foreground/5 rounded"
                                            onClick={() =>
                                                setCollapsed((prev) => ({
                                                    ...prev,
                                                    [section.title]:
                                                        !prev[section.title],
                                                }))
                                            }
                                        >
                                            <span>{section.title}</span>
                                            {collapsed[section.title] ? (
                                                <ChevronDownIcon className="w-3 h-3" />
                                            ) : (
                                                <ChevronUpIcon className="w-3 h-3" />
                                            )}
                                        </button>

                                        {!collapsed[section.title] && (
                                            <div
                                                className={clsx(
                                                    "flex flex-col gap-2 items-center w-full"
                                                )}
                                            >
                                                {section.items.map((item) => (
                                                    <button
                                                        key={item.label}
                                                        title={item.label}
                                                        className={clsx(
                                                            "h-auto px-1 w-full rounded-sm text-sm cursor-pointer hover:bg-foreground/5 text-left"
                                                        )}
                                                        draggable
                                                        onDragStart={(e) => {
                                                            const payload = {
                                                                title: item.label,
                                                                kind: item.kind,
                                                            } as const;
                                                            e.dataTransfer.setData(
                                                                "application/reactflow",
                                                                JSON.stringify(
                                                                    payload
                                                                )
                                                            );
                                                            e.dataTransfer.effectAllowed =
                                                                "move";
                                                        }}
                                                        onDoubleClick={() => {
                                                            if (pendingNodeId) {
                                                                convertPending({
                                                                    kind: item.kind,
                                                                    title: item.label,
                                                                });
                                                            }
                                                        }}
                                                    >
                                                        <div className="text-foreground flex items-center gap-2">
                                                            {getIconFor(
                                                                item.kind,
                                                                section.title
                                                            )}
                                                            {item.label}
                                                        </div>
                                                    </button>
                                                ))}
                                            </div>
                                        )}
                                    </div>
                                ))}
                            </div>
                        </div>
                    </div>
                </aside>
            )}
        </>
    );
}

export default Sidebar;
