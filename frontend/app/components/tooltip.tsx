import {
    cloneElement,
    useCallback,
    useEffect,
    useId,
    useLayoutEffect,
    useRef,
    useState,
    type FocusEvent,
    type ReactElement,
} from "react";
import { createPortal } from "react-dom";
import { placeTooltip, type Placement } from "../tooltip-position";

export const tooltipDelayMs = 300;

type Anchorable = { "aria-describedby"?: string };

// Wraps ONE icon-only control. Shows after a short hover delay and immediately on keyboard focus,
// hides on Esc, blur, press or pointer leave. Rendered in a portal on document.body and positioned
// from the anchor's bounding box, so it stays readable inside zoomed canvas nodes.
export function Tooltip({
    label,
    children,
    prefer = "top",
}: {
    label: string;
    children: ReactElement<Anchorable>;
    prefer?: Placement;
}) {
    const id = useId();
    const wrapperRef = useRef<HTMLSpanElement>(null);
    const tipRef = useRef<HTMLDivElement>(null);
    const timer = useRef<number | null>(null);
    const [open, setOpen] = useState(false);
    const [pos, setPos] = useState<{ left: number; top: number } | null>(null);

    const clearTimer = useCallback(() => {
        if (timer.current !== null) {
            window.clearTimeout(timer.current);
            timer.current = null;
        }
    }, []);
    const hide = useCallback(() => {
        clearTimer();
        setOpen(false);
        setPos(null);
    }, [clearTimer]);
    const showSoon = () => {
        clearTimer();
        timer.current = window.setTimeout(() => setOpen(true), tooltipDelayMs);
    };

    useEffect(() => clearTimer, [clearTimer]);

    useEffect(() => {
        if (!open) return;
        const onKey = (event: KeyboardEvent) => {
            if (event.key === "Escape") hide();
        };
        document.addEventListener("keydown", onKey);
        window.addEventListener("scroll", hide, true);
        return () => {
            document.removeEventListener("keydown", onKey);
            window.removeEventListener("scroll", hide, true);
        };
    }, [open, hide]);

    useLayoutEffect(() => {
        if (!open) return;
        const anchor = wrapperRef.current?.firstElementChild;
        const tip = tipRef.current;
        if (!anchor || !tip) return;
        const box = anchor.getBoundingClientRect();
        const size = tip.getBoundingClientRect();
        const placed = placeTooltip(
            { left: box.left, top: box.top, width: box.width, height: box.height },
            { width: size.width, height: size.height },
            { width: window.innerWidth, height: window.innerHeight },
            { prefer },
        );
        setPos({ left: placed.left, top: placed.top });
    }, [open, label, prefer]);

    const onFocus = (event: FocusEvent) => {
        if ((event.target as HTMLElement).matches?.(":focus-visible")) {
            clearTimer();
            setOpen(true);
        }
    };

    return (
        <span
            ref={wrapperRef}
            className="contents"
            onMouseEnter={showSoon}
            onMouseLeave={hide}
            onMouseDown={hide}
            onFocus={onFocus}
            onBlur={hide}
        >
            {cloneElement(children, open ? { "aria-describedby": id } : {})}
            {open && typeof document !== "undefined"
                ? createPortal(
                      <div
                          ref={tipRef}
                          id={id}
                          role="tooltip"
                          style={{
                              position: "fixed",
                              left: pos?.left ?? 0,
                              top: pos?.top ?? 0,
                              visibility: pos ? "visible" : "hidden",
                          }}
                          className="pointer-events-none z-[100] w-max max-w-[16rem] rounded-md bg-foreground px-2 py-1 text-[11px] leading-snug text-background"
                      >
                          {label}
                      </div>,
                      document.body,
                  )
                : null}
        </span>
    );
}
