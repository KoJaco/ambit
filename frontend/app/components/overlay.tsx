import { useEffect, useRef, type ReactNode, type RefObject } from "react";
import { createPortal } from "react-dom";
import clsx from "clsx";
import { focusableSelector, isBackdropClick, nextFocusIndex } from "../overlay-logic";
import styles from "./overlay.module.css";

// Modal primitive for the shortcuts dialog and docs sheet. Portalled to document.body, with a
// blurred backdrop, focus trap, body scroll lock, Esc and backdrop-click to close, and focus
// returned to the opener. The caller renders it only while open.
export function Overlay({
    variant,
    labelledBy,
    onClose,
    returnFocusRef,
    initialFocusRef,
    children,
}: {
    variant: "dialog" | "sheet";
    labelledBy: string;
    onClose: () => void;
    returnFocusRef?: RefObject<HTMLElement | null>;
    initialFocusRef?: RefObject<HTMLElement | null>;
    children: ReactNode;
}) {
    const panelRef = useRef<HTMLDivElement>(null);
    const backdropRef = useRef<HTMLDivElement>(null);
    const pressTarget = useRef<EventTarget | null>(null);
    const onCloseRef = useRef(onClose);
    onCloseRef.current = onClose;

    useEffect(() => {
        const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
        const previousOverflow = document.body.style.overflow;
        document.body.style.overflow = "hidden";
        (initialFocusRef?.current ?? panelRef.current)?.focus();

        const onKeyDown = (event: KeyboardEvent) => {
            if (event.key === "Escape") {
                event.preventDefault();
                event.stopPropagation();
                onCloseRef.current();
                return;
            }
            if (event.key !== "Tab") return;
            const panel = panelRef.current;
            if (!panel) return;
            const items = Array.from(panel.querySelectorAll<HTMLElement>(focusableSelector));
            if (items.length === 0) {
                event.preventDefault();
                panel.focus();
                return;
            }
            const current = items.indexOf(document.activeElement as HTMLElement);
            const inside = panel.contains(document.activeElement);
            // Only take over at the edges or when focus escaped; otherwise let the browser move.
            const atEdge = event.shiftKey ? current <= 0 : current === items.length - 1;
            if (inside && !atEdge) return;
            event.preventDefault();
            items[nextFocusIndex(items.length, inside ? current : -1, event.shiftKey)]?.focus();
        };
        document.addEventListener("keydown", onKeyDown, true);

        return () => {
            document.removeEventListener("keydown", onKeyDown, true);
            document.body.style.overflow = previousOverflow;
            const target = returnFocusRef?.current ?? opener;
            window.setTimeout(() => {
                // Skip when another overlay took over (docs sheet handing off to the shortcuts dialog).
                if (target && target.isConnected && !document.querySelector('[aria-modal="true"]')) target.focus();
            }, 0);
        };
        // Mount-only: focus handling must not re-run on re-render.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    if (typeof document === "undefined") return null;

    return createPortal(
        <div
            ref={backdropRef}
            className={clsx(
                "fixed inset-0 z-[90] flex bg-foreground/30 backdrop-blur-sm",
                variant === "dialog" ? "items-center justify-center p-4" : "justify-end",
                styles.backdrop,
            )}
            onPointerDown={(event) => {
                pressTarget.current = event.target;
            }}
            onPointerUp={(event) => {
                if (isBackdropClick(pressTarget.current, event.target, backdropRef.current)) onClose();
                pressTarget.current = null;
            }}
        >
            <div
                ref={panelRef}
                role="dialog"
                aria-modal="true"
                aria-labelledby={labelledBy}
                tabIndex={-1}
                className={clsx(
                    "flex max-h-full flex-col border border-border bg-card text-card-foreground shadow-xl outline-none",
                    variant === "dialog"
                        ? "w-full max-w-md rounded-[20px]"
                        : "h-full w-[min(520px,92vw)] rounded-l-[20px] border-r-0",
                    variant === "dialog" ? styles.dialog : styles.sheet,
                )}
            >
                {children}
            </div>
        </div>,
        document.body,
    );
}
