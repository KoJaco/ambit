import type { ReactNode } from "react";

export type ControlBarTool =
    | "grab"
    | "pointer"
    | "undo"
    | "redo"
    | "zoom-in"
    | "zoom-out"
    | "recenter";

export type SidebarSection = {
    title: string;
    items: SidebarItem[];
};

export type SidebarItem = {
    label: string;
    displayTitle?: string;
    icon?: ReactNode;
};
