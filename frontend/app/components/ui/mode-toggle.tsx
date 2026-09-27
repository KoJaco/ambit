import { MoonStarIcon, SunIcon } from "lucide-react";
import { useTheme, type Theme } from "../theme";

export const ModeToggle = () => {
    const [theme, setTheme] = useTheme();

    function toggleTheme() {
        const nextTheme =
            (theme ?? "light") === "dark"
                ? ("light" as Theme)
                : ("dark" as Theme);
        setTheme(nextTheme);
    }

    return (
        <button
            type="button"
            aria-label="Toggle theme"
            onClick={toggleTheme}
            className="inline-flex h-8 w-8 items-center rounded-full border border-foreground/25 bg-primary-foreground text-primary px-2 text-sm"
        >
            {theme === "dark" ? <MoonStarIcon /> : <SunIcon />}
        </button>
    );
};
