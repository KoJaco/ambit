// Pure helpers for partial-model parts: scoping the root level to a part and validating the part form.
import type { Level, Part, Relationship, Summary } from "./api";

/** Splits the textarea value into trimmed, non-empty, de-duplicated globs (one per line). */
export function parseGlobs(text: string): string[] {
    const seen = new Set<string>();
    const out: string[] = [];
    for (const line of text.split(/\r?\n/)) {
        const glob = line.trim();
        if (glob === "" || seen.has(glob)) continue;
        seen.add(glob);
        out.push(glob);
    }
    return out;
}

/** Keeps only the roots that belong to the part. A null part means "All": nothing is filtered. */
export function filterRoots<T extends { id: string }>(roots: T[], part: Pick<Part, "root_node_ids"> | null): T[] {
    if (!part) return roots;
    const ids = new Set(part.root_node_ids);
    return roots.filter((r) => ids.has(r.id));
}

/**
 * Scopes a root-level response to a part. A node is in the part when it is a part root or sits
 * inside one (any ancestor through `group_id` is a root), so an open root keeps its members.
 * Edges and crossings that would point at a removed node are dropped. Levels below the root
 * are never scoped.
 */
export function scopeRootLevel(level: Level, part: Pick<Part, "root_node_ids"> | null): Level {
    if (!part || level.node) return level;
    const all = level.children ?? [];
    const roots = new Set(part.root_node_ids);
    const byId = new Map(all.map((c) => [c.id, c]));
    const inPart = (child: Summary): boolean => {
        const seen = new Set<string>();
        for (let at: Summary | undefined = child; at && !seen.has(at.id); at = at.group_id ? byId.get(at.group_id) : undefined) {
            if (roots.has(at.id)) return true;
            seen.add(at.id);
        }
        return false;
    };
    const children = all.filter(inPart);
    const keep = new Set(children.map((c) => c.id));
    const inScope = (r: Relationship) => keep.has(r.from) && keep.has(r.to);
    return {
        ...level,
        children,
        relationships: (level.relationships ?? []).filter(inScope),
        crossings: (level.crossings ?? []).filter((c) => keep.has(c.node_id)),
    };
}

/** The stored selection if that part still exists, else null (All). */
export function resolveSelectedPart(storedId: string | null | undefined, parts: Part[]): Part | null {
    if (!storedId) return null;
    return parts.find((p) => p.id === storedId) ?? null;
}

export function slugifyPartId(name: string): string {
    return name
        .trim()
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, "-")
        .replace(/^-+|-+$/g, "");
}

export type PartForm = {
    // Set when editing an existing part; the id never changes.
    id?: string;
    name: string;
    includeText: string;
    rootIds: string[];
};

export type PartFormResult = { ok: true; part: Part } | { ok: false; errors: { name?: string; include?: string } };

export function validatePartForm(form: PartForm, existing: Part[]): PartFormResult {
    const errors: { name?: string; include?: string } = {};
    const name = form.name.trim();
    const include = parseGlobs(form.includeText);
    if (name === "") errors.name = "Enter a name.";
    const id = form.id ?? slugifyPartId(name);
    if (name !== "" && id === "") errors.name = "Use letters or numbers in the name.";
    else if (!form.id && id !== "" && existing.some((p) => p.id === id)) errors.name = "A part with this name already exists.";
    if (include.some((g) => g.startsWith("/") || g.split("/").includes(".."))) {
        errors.include = "Globs must be relative paths without \"..\".";
    }
    if (errors.name || errors.include) return { ok: false, errors };
    return { ok: true, part: { id, name, include, root_node_ids: Array.from(new Set(form.rootIds)) } };
}

export function partSummary(part: Part): string {
    const g = part.include.length;
    const r = part.root_node_ids.length;
    return `${g} ${g === 1 ? "glob" : "globs"}, ${r} ${r === 1 ? "root" : "roots"}`;
}
