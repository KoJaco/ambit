import { useId, useRef, type RefObject } from "react";
import { Overlay } from "./overlay";
import { CloseButton } from "./shortcuts-dialog";
import { docsSections, inlineText, type DocBlock, type Inline } from "../docs-content";

const codeClass = "rounded bg-muted px-1 py-0.5 font-mono text-[12px]";
const linkClass =
    "rounded-sm font-medium text-primary underline underline-offset-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary";

function Inlines({ content, onOpenShortcuts }: { content: Inline[]; onOpenShortcuts: () => void }) {
    return (
        <>
            {content.map((part, index) => {
                if (typeof part === "string") return part;
                if ("code" in part) {
                    return (
                        <code key={index} className={codeClass}>
                            {part.code}
                        </code>
                    );
                }
                return (
                    <button key={index} type="button" onClick={onOpenShortcuts} className={linkClass}>
                        {part.text}
                    </button>
                );
            })}
        </>
    );
}

function Block({ block, onOpenShortcuts }: { block: DocBlock; onOpenShortcuts: () => void }) {
    switch (block.type) {
        case "h3":
            return <h4 className="mt-4 mb-1 text-sm font-semibold">{block.text}</h4>;
        case "p":
            return (
                <p className="my-2 text-sm leading-relaxed">
                    <Inlines content={block.content} onOpenShortcuts={onOpenShortcuts} />
                </p>
            );
        case "ul":
        case "ol": {
            const List = block.type === "ul" ? "ul" : "ol";
            return (
                <List className={`my-2 space-y-1.5 pl-5 text-sm leading-relaxed ${block.type === "ul" ? "list-disc" : "list-decimal"}`}>
                    {block.items.map((item, index) => (
                        <li key={index}>
                            <Inlines content={item} onOpenShortcuts={onOpenShortcuts} />
                        </li>
                    ))}
                </List>
            );
        }
        case "code":
            return (
                <pre
                    tabIndex={0}
                    aria-label={block.caption ? `${block.caption}: ${block.text.split("\n")[0]}` : undefined}
                    className="my-2 overflow-x-auto rounded-xl border border-border bg-muted px-3 py-2 font-mono text-[12px] leading-relaxed focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
                >
                    <code>{block.text}</code>
                </pre>
            );
    }
}

function scrollToSection(id: string) {
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    document.getElementById(`docs-${id}`)?.scrollIntoView({ behavior: reduce ? "auto" : "smooth", block: "start" });
}

export function DocsSheet({
    onClose,
    onOpenShortcuts,
    returnFocusRef,
}: {
    onClose: () => void;
    onOpenShortcuts: () => void;
    returnFocusRef?: RefObject<HTMLElement | null>;
}) {
    const titleId = useId();
    const closeRef = useRef<HTMLButtonElement>(null);
    return (
        <Overlay variant="sheet" labelledBy={titleId} onClose={onClose} returnFocusRef={returnFocusRef} initialFocusRef={closeRef}>
            <div className="flex items-center justify-between gap-2 border-b border-border px-5 py-3">
                <h2 id={titleId} className="text-base font-semibold">
                    Documentation
                </h2>
                <CloseButton label="Close documentation" onClick={onClose} buttonRef={closeRef} />
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
                <nav aria-label="Sections" className="mb-2">
                    <ul className="flex flex-wrap gap-x-4 gap-y-1 text-sm">
                        {docsSections.map((section) => (
                            <li key={section.id}>
                                <button type="button" onClick={() => scrollToSection(section.id)} className={linkClass}>
                                    {section.title}
                                </button>
                            </li>
                        ))}
                    </ul>
                </nav>
                {docsSections.map((section) => (
                    <section key={section.id} id={`docs-${section.id}`} aria-label={inlineText([section.title])} className="mt-6 scroll-mt-2">
                        <h3 className="text-lg font-semibold">{section.title}</h3>
                        {section.blocks.map((block, index) => (
                            <Block key={index} block={block} onOpenShortcuts={onOpenShortcuts} />
                        ))}
                    </section>
                ))}
            </div>
        </Overlay>
    );
}
