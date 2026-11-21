import { useId, useRef, type RefObject } from "react";
import { XIcon } from "lucide-react";
import { Overlay } from "./overlay";
import { Tooltip } from "./tooltip";
import { formatChord, shortcutsByGroup, type Platform } from "../shortcuts";

export function Kbd({ children }: { children: string }) {
    return (
        <kbd className="inline-flex min-w-6 items-center justify-center rounded-md border border-border bg-background px-1.5 py-0.5 font-mono text-[11px] text-foreground">
            {children}
        </kbd>
    );
}

export function CloseButton({ label, onClick, buttonRef }: { label: string; onClick: () => void; buttonRef?: RefObject<HTMLButtonElement | null> }) {
    return (
        <Tooltip label={`${label} (Esc)`}>
            <button
                ref={buttonRef}
                type="button"
                aria-label={label}
                onClick={onClick}
                className="inline-flex size-8 items-center justify-center rounded-full text-foreground hover:bg-accent hover:ring-1 hover:ring-inset hover:ring-border focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
            >
                <XIcon className="size-4" aria-hidden />
            </button>
        </Tooltip>
    );
}

export function ShortcutsDialog({
    platform,
    onClose,
    returnFocusRef,
}: {
    platform: Platform;
    onClose: () => void;
    returnFocusRef?: RefObject<HTMLElement | null>;
}) {
    const titleId = useId();
    const closeRef = useRef<HTMLButtonElement>(null);
    return (
        <Overlay variant="dialog" labelledBy={titleId} onClose={onClose} returnFocusRef={returnFocusRef} initialFocusRef={closeRef}>
            <div className="flex items-center justify-between gap-2 border-b border-border px-5 py-3">
                <h2 id={titleId} className="text-base font-semibold">
                    Keyboard shortcuts
                </h2>
                <CloseButton label="Close shortcuts" onClick={onClose} buttonRef={closeRef} />
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
                {shortcutsByGroup().map(({ group, items }) => (
                    <section key={group} aria-label={group} className="mb-4 last:mb-0">
                        <h3 className="mb-1.5 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">{group}</h3>
                        <ul className="divide-y divide-border">
                            {items.map((item) => (
                                <li key={item.id} className="flex items-center justify-between gap-3 py-1.5 text-sm">
                                    <span>{item.label}</span>
                                    <span className="flex items-center gap-1.5">
                                        {item.keys.map((chord, index) => (
                                            <span key={chord} className="flex items-center gap-1.5">
                                                {index > 0 ? <span className="text-[11px] text-muted-foreground">or</span> : null}
                                                <Kbd>{formatChord(chord, platform)}</Kbd>
                                            </span>
                                        ))}
                                    </span>
                                </li>
                            ))}
                        </ul>
                    </section>
                ))}
                <section aria-label="Mouse and trackpad" className="mt-4">
                    <h3 className="mb-1.5 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">Mouse and trackpad</h3>
                    <ul className="divide-y divide-border text-sm">
                        <li className="flex items-center justify-between gap-3 py-1.5">
                            <span>Pan temporarily</span>
                            <span>
                                Hold <Kbd>Space</Kbd> and drag
                            </span>
                        </li>
                        <li className="flex items-center justify-between gap-3 py-1.5">
                            <span>Select several nodes</span>
                            <span>
                                <Kbd>Shift</Kbd> or <Kbd>{formatChord("Mod", platform)}</Kbd> and click
                            </span>
                        </li>
                        <li className="flex items-center justify-between gap-3 py-1.5">
                            <span>Box select</span>
                            <span>
                                Hold <Kbd>Shift</Kbd> and drag
                            </span>
                        </li>
                        <li className="flex items-center justify-between gap-3 py-1.5">
                            <span>Zoom</span>
                            <span>Mouse wheel or pinch</span>
                        </li>
                    </ul>
                </section>
            </div>
        </Overlay>
    );
}
