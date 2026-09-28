import clsx from "clsx";
import type { ReactNode } from "react";
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

    function cardClass(isFront: boolean) {
        return clsx(
            "absolute top-0 right-0 h-full w-full transition-transform",
            isFront ? "z-20 -translate-x-3 translate-y-3" : "z-10 translate-x-0 translate-y-0",
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
            <div className="pointer-events-none absolute top-0 right-0 h-full w-80">
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
        <div className="pointer-events-none absolute top-0 right-0 h-full w-80">
            <div className="relative h-full w-full">
                {wrapBack(back, onPromoteBack)}
                {wrapFront(frontPanel)}
            </div>
        </div>
    );
}
