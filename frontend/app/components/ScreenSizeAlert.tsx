import {
    MonitorIcon,
    MoveHorizontalIcon,
    SmartphoneIcon,
} from "lucide-react";
import { MIN_VIEWPORT_HEIGHT, MIN_VIEWPORT_WIDTH } from "../viewport";

export default function ScreenSizeAlert() {
    return (
        <div className="flex min-h-screen w-full flex-col items-center justify-center bg-background px-6 py-12">
            <div
                className="flex max-w-md flex-col items-center gap-6 rounded-xl border border-foreground/20 bg-card/80 p-8 text-center shadow-sm backdrop-blur"
                role="status"
                aria-live="polite"
            >
                <div className="flex items-center justify-center gap-3 text-foreground/70">
                    <SmartphoneIcon className="size-9 shrink-0" aria-hidden />
                    <MoveHorizontalIcon
                        className="size-7 shrink-0 text-foreground/40"
                        aria-hidden
                    />
                    <MonitorIcon className="size-10 shrink-0" aria-hidden />
                </div>
                <div className="flex flex-col gap-2">
                    <h1 className="text-xl font-semibold tracking-tight text-foreground">
                        Desktop canvas only
                    </h1>
                    <p className="text-sm leading-relaxed text-foreground/75">
                        Ambit&apos;s architect view needs a wide screen so the graph,
                        sidebar, and inspector can sit on the canvas at once. Open this
                        page on a computer or rotate a tablet to landscape.
                    </p>
                    <p className="text-xs text-foreground/55">
                        Minimum viewport: {MIN_VIEWPORT_WIDTH}&times;
                        {MIN_VIEWPORT_HEIGHT}px
                    </p>
                </div>
            </div>
        </div>
    );
}
