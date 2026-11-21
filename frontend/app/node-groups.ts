export type TypedItem = { name: string; type: string };
export type TypeGroup<T extends TypedItem> = { type: string; label: string; items: T[] };

export const UNTYPED_LABEL = "untyped";

function byText(a: string, b: string): number {
    return a.localeCompare(b, undefined, { sensitivity: "base", numeric: true });
}

// Groups nodes by type: groups alphabetical with "untyped" last, nodes sorted by name inside.
export function groupNodesByType<T extends TypedItem>(items: readonly T[]): TypeGroup<T>[] {
    const groups = new Map<string, T[]>();
    for (const item of items) {
        const key = item.type.trim();
        const list = groups.get(key);
        if (list) list.push(item);
        else groups.set(key, [item]);
    }
    return [...groups.entries()]
        .sort(([a], [b]) => {
            if (a === "" || b === "") return a === "" ? (b === "" ? 0 : 1) : -1;
            return byText(a, b);
        })
        .map(([type, list]) => ({
            type,
            label: type === "" ? UNTYPED_LABEL : type,
            items: list.slice().sort((a, b) => byText(a.name, b.name)),
        }));
}
