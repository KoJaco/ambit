import clsx from "clsx";
import { useEffect, useRef, type ReactNode } from "react";
import { usePanelPresence } from "../hooks/use-panel-presence";
import { railStackLayout, useRailState, type RightRailFront } from "../right-rail-state";
import { panelExitClass } from "./right-rail-panel";
// Below 1280px the centred control bar would sit under a full-height rail, so the rail stops above it.
const railClass = "pointer-events-none absolute top-0 right-0 z-50 h-full w-[min(440px,92vw)] max-[1279px]:h-[calc(100%-5rem)]";

export type { RightRailFront };

export function RightRailStack({
    front,
    onFocusInspector,
    onFocusReview,
    inspector,
    review,
}: {
    front: RightRailFront;
    onFocusInspector: () => void;
    onFocusReview: () => void;
    inspector: ReactNode | null;
    review: ReactNode | null;
}) {
    // Keep the last inspector around while it slides out after being closed.
    const lastInspector = useRef<ReactNode>(null);
    if (inspector != null) lastInspector.current = inspector;
    const presence = usePanelPresence(inspector != null);
    const inspectorContent = inspector ?? (presence.mounted ? lastInspector.current : null);
    useEffect(() => {
        if (!presence.mounted) lastInspector.current = null;
    }, [presence.mounted]);

    // Review is always mounted (it keeps proposals loading) but is only a sheet while it renders one.
    const reviewMounted = useRailState().review > 0;
    const { stacked, inspectorFront, reviewFront } = railStackLayout({
        hasInspector: inspectorContent != null,
        reviewMounted,
        front,
    });

    // Sheets are flush to the right edge. When two are open the back one peeks out on the left,
    // inset top and bottom, and a press on it brings it forward. Both wrappers stay mounted in a
    // fixed order, so promoting a sheet or a sheet appearing never remounts the other (and its state).
    function cardClass(isFront: boolean) {
        return clsx(
            "absolute right-0 w-full transition-transform empty:hidden motion-reduce:transition-none",
            isFront ? "top-0 z-20 h-full" : "inset-y-4 z-10 h-auto -translate-x-3",
        );
    }

    return (
        <div className={railClass}>
            <div className="relative h-full w-full">
                <div
                    className={clsx(
                        cardClass(inspectorFront),
                        stacked && "pointer-events-auto",
                        stacked && !inspectorFront && "cursor-pointer",
                        presence.exiting && clsx(panelExitClass, "pointer-events-none"),
                    )}
                    aria-hidden={presence.exiting || undefined}
                    inert={presence.exiting || undefined}
                    onPointerDown={stacked && !inspectorFront ? onFocusInspector : undefined}
                    role={stacked && !inspectorFront ? "presentation" : undefined}
                >
                    {inspectorContent}
                </div>
                <div
                    className={clsx(cardClass(reviewFront), stacked && "pointer-events-auto", stacked && !reviewFront && "cursor-pointer")}
                    onPointerDown={stacked && !reviewFront ? onFocusReview : undefined}
                    role={stacked && !reviewFront ? "presentation" : undefined}
                >
                    {review}
                </div>
            </div>
        </div>
    );
}
