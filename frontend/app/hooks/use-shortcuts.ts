import { useEffect, useRef } from "react";
import { resolveShortcut, type Platform, type ShortcutId } from "../shortcuts";

// One document-level keydown listener. Handlers are read through a ref, so the listener is
// registered once and removed on unmount. `dialogOpen` makes every shortcut inert (overlays
// own Esc themselves).
export function useShortcuts(
    handlers: Partial<Record<ShortcutId, () => void>>,
    options: { dialogOpen: boolean; platform: Platform },
) {
    const latest = useRef({ handlers, options });
    latest.current = { handlers, options };

    useEffect(() => {
        const onKeyDown = (event: KeyboardEvent) => {
            if (event.defaultPrevented) return;
            const { handlers: current, options: opts } = latest.current;
            const id = resolveShortcut(event, {
                target: event.target as HTMLElement | null,
                dialogOpen: opts.dialogOpen,
                platform: opts.platform,
            });
            if (!id) return;
            const handler = current[id];
            if (!handler) return;
            event.preventDefault();
            handler();
        };
        document.addEventListener("keydown", onKeyDown);
        return () => document.removeEventListener("keydown", onKeyDown);
    }, []);
}
