import { useEffect, useRef, useState } from "react";
import { panelExitMs } from "../right-rail-state";

// Keeps a sheet mounted for the length of its exit animation after `show` turns false.
// With prefers-reduced-motion the sheet unmounts at once. `exiting` is true while it lingers.
export function usePanelPresence(show: boolean): { mounted: boolean; exiting: boolean } {
    const [lingering, setLingering] = useState(false);
    const wasShown = useRef(false);
    useEffect(() => {
        if (show) {
            wasShown.current = true;
            setLingering(false);
            return;
        }
        if (!wasShown.current) return;
        wasShown.current = false;
        const reduced = typeof window !== "undefined" && window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
        const ms = panelExitMs(!!reduced);
        if (ms === 0) return;
        setLingering(true);
        const timer = window.setTimeout(() => setLingering(false), ms);
        return () => window.clearTimeout(timer);
    }, [show]);
    return { mounted: show || lingering, exiting: !show && lingering };
}
