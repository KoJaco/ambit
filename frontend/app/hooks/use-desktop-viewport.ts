import { useEffect, useState } from "react";
import { desktopViewportQuery, isDesktopViewport } from "../viewport";

function readSupported(): boolean {
    if (typeof window === "undefined") {
        return true;
    }
    if (typeof window.matchMedia === "function") {
        return window.matchMedia(desktopViewportQuery).matches;
    }
    return isDesktopViewport();
}

/** True when the viewport is large enough for the canvas UI (desktop / landscape tablet). */
export function useDesktopViewport(): boolean {
    const [supported, setSupported] = useState(readSupported);

    useEffect(() => {
        const media = window.matchMedia(desktopViewportQuery);
        const sync = () => setSupported(media.matches);
        sync();
        media.addEventListener("change", sync);
        return () => media.removeEventListener("change", sync);
    }, []);

    return supported;
}
