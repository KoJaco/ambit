import { useState } from "react";
import clsx from "clsx";
import styles from "./Sidebar.module.css";

export function Sidebar() {
    const [collapsed, setCollapsed] = useState(false);
    return (
        <aside
            className={clsx(
                "absolute top-0 left-0 z-40 h-screen border-r border-foreground/20 bg-background",
                collapsed ? "w-12" : "w-64"
            )}
        >
            <button
                type="button"
                className="m-2 rounded-md border border-foreground/20 px-2 py-1 text-xs"
                onClick={() => setCollapsed((value) => !value)}
            >
                {collapsed ? ">" : "<"}
            </button>
            {collapsed ? null : (
                <div className={clsx("h-[calc(100%-3rem)] px-3", styles.sidebarScroll)} />
            )}
        </aside>
    );
}

export default Sidebar;
