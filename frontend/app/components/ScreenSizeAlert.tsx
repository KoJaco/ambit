// Trigger via provider context if the canvas is too small to display the nodes
// TODO: finish this
import {
    ScreenShareIcon,
    MoveRightIcon,
    TabletSmartphoneIcon,
} from "lucide-react";

const ScreenSizeAlert = () => {
    return (
        <div className="relative h-screen w-full flex flex-col items-center justify-center">
            <div className="flex flex-col gap-y-4 text-center">
                <div>
                    <TabletSmartphoneIcon className="w-10 h-10 text-foreground" />
                    <MoveRightIcon className="w-10 h-10 text-foreground" />
                    <ScreenShareIcon className="w-10 h-10 text-foreground" />
                </div>
                <h1 className="text-2xl font-bold">Screen too small</h1>
                <p className="text-sm text-foreground/80">
                    Please resize your screen or access this on a larger screen
                    to view the canvas.
                </p>
            </div>
        </div>
    );
};

export default ScreenSizeAlert;
