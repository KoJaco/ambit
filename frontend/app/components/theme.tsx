import {
    createContext,
    useContext,
    useEffect,
    useState,
    type ReactNode,
} from "react";

export type Theme = "light" | "dark";

const STORAGE_KEY = "theme";

function isTheme(value: string | null): value is Theme {
    return value === "light" || value === "dark";
}

export function getStoredTheme(): Theme | null {
    if (typeof window === "undefined") return null;
    try {
        const value = localStorage.getItem(STORAGE_KEY);
        return isTheme(value) ? value : null;
    } catch {
        return null;
    }
}

function getPreferredTheme(): Theme {
    if (typeof window === "undefined") return "light";
    return window.matchMedia("(prefers-color-scheme: light)").matches
        ? "light"
        : "dark";
}

function resolveTheme(): Theme | null {
    if (typeof window === "undefined") return null;
    return getStoredTheme() ?? getPreferredTheme();
}

const ThemeContext = createContext<
    [Theme | null, (theme: Theme) => void] | undefined
>(undefined);

export function ThemeProvider({ children }: { children: ReactNode }) {
    const [theme, setThemeState] = useState<Theme | null>(resolveTheme);

    useEffect(() => {
        const media = window.matchMedia("(prefers-color-scheme: light)");
        const applySystemTheme = () => {
            if (getStoredTheme()) return;
            setThemeState(media.matches ? "light" : "dark");
        };
        media.addEventListener("change", applySystemTheme);
        return () => media.removeEventListener("change", applySystemTheme);
    }, []);

    useEffect(() => {
        const onStorage = (event: StorageEvent) => {
            if (event.key !== STORAGE_KEY || !isTheme(event.newValue)) return;
            setThemeState(event.newValue);
        };
        window.addEventListener("storage", onStorage);
        return () => window.removeEventListener("storage", onStorage);
    }, []);

    const setTheme = (next: Theme) => {
        try {
            localStorage.setItem(STORAGE_KEY, next);
        } catch {
            // Private browsing can reject storage; the in-memory theme still updates.
        }
        setThemeState(next);
    };

    return (
        <ThemeContext.Provider value={[theme, setTheme]}>
            {children}
        </ThemeContext.Provider>
    );
}

export function useTheme() {
    const context = useContext(ThemeContext);
    if (!context) {
        throw new Error("useTheme must be used within a ThemeProvider");
    }
    return context;
}

// Keep this in sync with resolveTheme. It runs before hydration so the
// pre-rendered shell does not flash the wrong palette.
export const themeScript = `(function(){try{var stored=localStorage.getItem(${JSON.stringify(STORAGE_KEY)});var theme=stored==="light"||stored==="dark"?stored:window.matchMedia("(prefers-color-scheme: light)").matches?"light":"dark";var el=document.documentElement;el.dataset.theme=theme;el.classList.remove("light","dark");el.classList.add(theme);}catch(e){}})();`;
