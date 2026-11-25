import clsx from "clsx";
import { CheckIcon, ChevronDownIcon } from "lucide-react";
import { forwardRef, useCallback, useEffect, useId, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { firstEnabled, lastEnabled, listKeyAction, moveActive, typeaheadIndex, type SelectOption } from "../../select-nav";

export type { SelectOption };

type SelectProps = {
    id?: string;
    value: string;
    options: SelectOption[];
    onChange: (value: string) => void;
    disabled?: boolean;
    className?: string;
    "aria-label"?: string;
    "aria-describedby"?: string;
};

const focusableSelector =
    'a[href], button, input, select, textarea, [tabindex]:not([tabindex="-1"])';

const triggerClass =
    "flex w-full items-center justify-between gap-2 rounded-[10px] border border-transparent bg-accent px-2.5 py-1.5 text-left text-sm hover:border-border focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:border-primary disabled:cursor-not-allowed disabled:opacity-50";

// Themed single-select. Focus moves to the listbox while open (aria-activedescendant) and returns
// to the trigger on close. The popup is portaled so scroll containers and sheets never clip it.
export const Select = forwardRef<HTMLButtonElement, SelectProps>(function Select(
    { id, value, options, onChange, disabled, className, "aria-label": ariaLabel, "aria-describedby": describedBy },
    forwardedRef,
) {
    const autoId = useId();
    const triggerId = id ?? `${autoId}-trigger`;
    const listId = `${autoId}-list`;
    const triggerRef = useRef<HTMLButtonElement | null>(null);
    const listRef = useRef<HTMLUListElement>(null);
    const typed = useRef({ text: "", timer: 0 });
    const [open, setOpen] = useState(false);
    const [active, setActive] = useState(-1);
    const [pos, setPos] = useState<{ left: number; top: number; width: number; maxHeight: number } | null>(null);

    const setRefs = (el: HTMLButtonElement | null) => {
        triggerRef.current = el;
        if (typeof forwardedRef === "function") forwardedRef(el);
        else if (forwardedRef) forwardedRef.current = el;
    };

    const selectedIndex = options.findIndex((o) => o.value === value);
    const selected = options[selectedIndex];

    const close = useCallback((refocus: boolean) => {
        setOpen(false);
        setPos(null);
        if (refocus) triggerRef.current?.focus();
    }, []);

    const openList = () => {
        if (disabled) return;
        setActive(selectedIndex >= 0 && !options[selectedIndex].disabled ? selectedIndex : firstEnabled(options));
        setOpen(true);
    };

    const choose = (index: number) => {
        const option = options[index];
        if (!option || option.disabled) return;
        if (option.value !== value) onChange(option.value);
        close(true);
    };

    useLayoutEffect(() => {
        if (!open) return;
        const rect = triggerRef.current?.getBoundingClientRect();
        if (!rect) return;
        const below = window.innerHeight - rect.bottom - 8;
        const above = rect.top - 8;
        const flip = below < 160 && above > below;
        const maxHeight = Math.max(96, Math.min(280, flip ? above : below));
        setPos({
            left: rect.left,
            width: rect.width,
            maxHeight,
            top: flip ? rect.top - 4 - Math.min(maxHeight, options.length * 32 + 8) : rect.bottom + 4,
        });
    }, [open, options.length]);

    useEffect(() => {
        if (open && pos) listRef.current?.focus({ preventScroll: true });
    }, [open, pos]);

    useEffect(() => {
        if (!open) return;
        const onPointer = (event: PointerEvent) => {
            const t = event.target as Node;
            if (listRef.current?.contains(t) || triggerRef.current?.contains(t)) return;
            close(false);
        };
        const onScroll = (event: Event) => {
            if (listRef.current?.contains(event.target as Node)) return;
            close(false);
        };
        document.addEventListener("pointerdown", onPointer);
        window.addEventListener("scroll", onScroll, true);
        const onResize = () => close(false);
        window.addEventListener("resize", onResize);
        return () => {
            window.removeEventListener("resize", onResize);
            document.removeEventListener("pointerdown", onPointer);
            window.removeEventListener("scroll", onScroll, true);
        };
    }, [open, close]);

    useEffect(() => {
        if (!open || active < 0) return;
        document.getElementById(`${listId}-${active}`)?.scrollIntoView?.({ block: "nearest" });
    }, [open, active, listId]);

    useEffect(() => () => window.clearTimeout(typed.current.timer), []);

    const runTypeahead = (char: string) => {
        window.clearTimeout(typed.current.timer);
        typed.current.text += char;
        typed.current.timer = window.setTimeout(() => (typed.current.text = ""), 600);
        const hit = typeaheadIndex(options, typed.current.text, active >= 0 ? active : selectedIndex);
        return hit;
    };

    const onTriggerKeyDown = (event: React.KeyboardEvent) => {
        if (["ArrowDown", "ArrowUp", "Enter", " "].includes(event.key)) {
            event.preventDefault();
            openList();
        }
    };

    const focusAdjacent = (backwards: boolean) => {
        const trigger = triggerRef.current;
        if (!trigger) return;
        const root = trigger.closest<HTMLElement>('[role="dialog"]') ?? document.body;
        const items = Array.from(root.querySelectorAll<HTMLElement>(focusableSelector)).filter(
            (el) => !el.closest("[data-select-popup]") && !el.hasAttribute("disabled") && el.getClientRects().length > 0,
        );
        const at = items.indexOf(trigger);
        const next = at < 0 ? undefined : items[at + (backwards ? -1 : 1)];
        (next ?? trigger).focus();
    };

    const onListKeyDown = (event: React.KeyboardEvent) => {
        const action = listKeyAction(event.key, {
            ctrlKey: event.ctrlKey,
            metaKey: event.metaKey,
            altKey: event.altKey,
            shiftKey: event.shiftKey,
            searching: typed.current.text !== "",
        });
        if (action.preventDefault) event.preventDefault();
        if (action.stopPropagation) event.stopPropagation();
        switch (action.kind) {
            case "next":
                setActive((a) => moveActive(options, a, 1));
                break;
            case "prev":
                setActive((a) => moveActive(options, a, -1));
                break;
            case "first":
                setActive(firstEnabled(options));
                break;
            case "last":
                setActive(lastEnabled(options));
                break;
            case "choose":
                choose(active);
                break;
            case "close":
                close(true);
                break;
            case "tab":
                close(true);
                focusAdjacent(event.shiftKey);
                break;
            case "type": {
                const hit = runTypeahead(event.key);
                if (hit >= 0) setActive(hit);
                break;
            }
        }
    };

    const portalTarget =
        open && typeof document !== "undefined"
            ? (triggerRef.current?.closest<HTMLElement>('[role="dialog"]') ?? document.body)
            : null;

    return (
        <>
            <button
                ref={setRefs}
                id={triggerId}
                type="button"
                disabled={disabled}
                role="combobox"
                aria-haspopup="listbox"
                aria-expanded={open}
                aria-controls={open ? listId : undefined}
                aria-activedescendant={open && active >= 0 ? `${listId}-${active}` : undefined}
                aria-label={ariaLabel}
                aria-describedby={describedBy}
                className={clsx(triggerClass, className)}
                onClick={() => (open ? close(false) : openList())}
                onKeyDown={onTriggerKeyDown}
            >
                <span className="min-w-0 truncate">{selected?.label ?? ""}</span>
                <ChevronDownIcon className="size-4 shrink-0 text-muted-foreground" aria-hidden />
            </button>
            {portalTarget
                ? createPortal(
                      <ul
                          ref={listRef}
                          id={listId}
                          role="listbox"
                          data-select-popup=""
                          tabIndex={-1}
                          aria-labelledby={triggerId}
                          aria-activedescendant={active >= 0 ? `${listId}-${active}` : undefined}
                          onKeyDown={onListKeyDown}
                          style={{
                              position: "fixed",
                              left: pos?.left ?? 0,
                              top: pos?.top ?? 0,
                              minWidth: pos?.width,
                              width: "max-content",
                              maxWidth: "min(28rem, 90vw)",
                              maxHeight: pos?.maxHeight,
                              visibility: pos ? "visible" : "hidden",
                          }}
                          className="z-[100] overflow-y-auto rounded-[10px] border border-border bg-popover p-1 text-sm text-popover-foreground shadow-lg focus:outline-none"
                      >
                          {options.map((option, index) => (
                              <li
                                  key={option.value}
                                  id={`${listId}-${index}`}
                                  role="option"
                                  aria-selected={option.value === value}
                                  aria-disabled={option.disabled || undefined}
                                  className={clsx(
                                      "flex min-h-7 cursor-pointer items-center justify-between gap-2 rounded-md px-2 py-1",
                                      index === active && "bg-accent ring-2 ring-inset ring-ring/40",
                                      option.disabled && "cursor-not-allowed opacity-50",
                                  )}
                                  onPointerMove={() => !option.disabled && setActive(index)}
                                  onClick={() => choose(index)}
                              >
                                  <span className="min-w-0 break-words">{option.label}</span>
                                  {option.value === value ? <CheckIcon className="size-4 shrink-0 text-primary" aria-hidden /> : null}
                              </li>
                          ))}
                      </ul>,
                      portalTarget,
                  )
                : null}
        </>
    );
});
