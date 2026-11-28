import { useEffect, useState } from "react";
import { nextPanelPresence, panelExitMs } from "../right-rail-state";

function prefersReducedMotion(): boolean {
    return typeof window !== "undefined" && !!window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
}

// Keeps a sheet mounted for the length of its exit animation after `show` turns false.
// With prefers-reduced-motion the sheet unmounts at once. `exiting` is true while it lingers.
// `lingering` is derived during render (previous value held in state) so the closing panel is
// never unmounted and remounted between the prop change and an effect.
export function usePanelPresence(show: boolean): { mounted: boolean; exiting: boolean } {
    const [state, setState] = useState({ show, lingering: false });
    let current = state;
    if (state.show !== show) {
        current = nextPanelPresence(state, show, prefersReducedMotion());
        setState(current);
    }
    const lingering = current.lingering;
    useEffect(() => {
        if (show || !lingering) return;
        const timer = window.setTimeout(() => setState((s) => (s.show ? s : { ...s, lingering: false })), panelExitMs(prefersReducedMotion()));
        return () => window.clearTimeout(timer);
    }, [show, lingering]);
    return { mounted: show || lingering, exiting: !show && lingering };
}
