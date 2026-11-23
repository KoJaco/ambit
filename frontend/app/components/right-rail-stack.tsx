import clsx from "clsx";
import type { ReactNode } from "react";
// Below 1280px the centred control bar would sit under a full-height rail, so the rail stops above it.
const railClass = "pointer-events-none absolute top-0 right-0 z-50 h-full w-[min(440px,92vw)] max-[1279px]:h-[calc(100%-5rem)]";

export type RightRailFront = "inspector" | "review";

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
    const hasInspector = inspector != null;
    const hasReview = review != null;
    if (!hasInspector && !hasReview) return null;

    const stacked = hasInspector && hasReview;
    const inspectorFront = !stacked || front === "inspector";
    const reviewFront = !stacked || front === "review";

    // Sheets are flush to the right edge. When two are open the back one peeks out on the left,
    // inset top and bottom, and a press on it brings it forward.
    function cardClass(isFront: boolean) {
        return clsx(
            "absolute right-0 w-full transition-transform motion-reduce:transition-none",
            isFront ? "top-0 z-20 h-full" : "inset-y-4 z-10 h-auto -translate-x-3",
        );
    }

    function wrapBack(content: ReactNode, onPromote: () => void) {
        return (
            <div
                className={clsx(cardClass(false), "pointer-events-auto cursor-pointer")}
                onPointerDown={() => onPromote()}
                role="presentation"
            >
                {content}
            </div>
        );
    }

    function wrapFront(content: ReactNode) {
        return <div className={clsx(cardClass(true), "pointer-events-auto")}>{content}</div>;
    }

    if (!stacked) {
        const single = hasInspector ? inspector : review;
        const onFocus = hasInspector ? onFocusInspector : onFocusReview;
        return (
            <div className={railClass}>
                <div className="relative h-full w-full">
                    <div className="absolute top-0 right-0 z-10 h-full w-full">{single}</div>
                </div>
            </div>
        );
    }

    const back = inspectorFront ? review : inspector;
    const frontPanel = inspectorFront ? inspector : review;
    const onPromoteBack = inspectorFront ? onFocusReview : onFocusInspector;
    return (
        <div className={railClass}>
            <div className="relative h-full w-full">
                {wrapBack(back, onPromoteBack)}
                {wrapFront(frontPanel)}
            </div>
        </div>
    );
}
