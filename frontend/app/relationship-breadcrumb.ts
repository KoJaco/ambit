import { getNode, getRelationshipLevel, type Relationship } from "./api";

/** Every ancestor is navigable, including grouping containers. */
export type BreadcrumbCrumb = { id: string; name: string; href: string; link: boolean };

/** Type given to containers created by grouping (core DefaultContainerType). */
export const groupingContainerType = "container";

/** A container that only groups its members on the canvas; it adds no level worth a crumb. */
export function isGroupingContainer(node: { type: string }): boolean {
    return node.type.trim().toLowerCase() === groupingContainerType;
}

/** Maps a root-to-leaf chain to crumbs. Every entry is a link, grouping containers included. */
export function visibleCrumbs(chain: { id: string; name: string; type: string }[]): BreadcrumbCrumb[] {
    return chain.map((node) => ({
        id: node.id,
        name: node.name,
        href: `/node/${node.id}`,
        link: true,
    }));
}

/** Ancestors from root down to startId (inclusive when includeStart). */
export async function nodeAncestorChain(
    startId: string,
    signal?: AbortSignal,
    includeStart = false,
): Promise<BreadcrumbCrumb[]> {
    const chain: { id: string; name: string; type: string }[] = [];
    const seen = new Set<string>();
    let current: string | undefined = startId;
    let interiorOf: string | undefined;
    if (!includeStart) {
        const first = await getNode(startId, signal);
        current = first.parent_id || undefined;
    }
    while (current && !seen.has(current)) {
        seen.add(current);
        const node = await getNode(current, signal);
        chain.push({ id: node.id, name: node.name, type: node.type });
        if (node.relationship_id && !node.parent_id) interiorOf = node.relationship_id;
        current = node.parent_id || undefined;
    }
    chain.reverse();
    const crumbs = visibleCrumbs(chain);
    if (!interiorOf) return crumbs;
    // The chain starts inside a relationship: continue above it through the edge's endpoints.
    try {
        const level = await getRelationshipLevel(interiorOf, signal);
        const above = await relationshipPrefixCrumbs(level.relationship, undefined, signal);
        const known = new Set(crumbs.map((c) => c.id));
        return [...above.filter((c) => !known.has(c.id)), ...crumbs];
    } catch {
        return crumbs;
    }
}

/**
 * Prefix crumbs for a relationship level: the node drill-down where the edge lives.
 */
export async function relationshipPrefixCrumbs(
    rel: Relationship,
    fromNodeId: string | undefined,
    signal?: AbortSignal,
): Promise<BreadcrumbCrumb[]> {
    if (fromNodeId) {
        return nodeAncestorChain(fromNodeId, signal, true);
    }
    const [fromNode, toNode] = await Promise.all([
        getNode(rel.from, signal),
        getNode(rel.to, signal),
    ]);
    if (
        fromNode.parent_id &&
        fromNode.parent_id === toNode.parent_id
    ) {
        return nodeAncestorChain(fromNode.parent_id, signal, true);
    }
    if (fromNode.parent_id) {
        return nodeAncestorChain(fromNode.parent_id, signal, true);
    }
    if (toNode.parent_id) {
        return nodeAncestorChain(toNode.parent_id, signal, true);
    }
    return [];
}

/** Best node route to return after deleting a relationship on its interior page. */
export function returnNodeIdAfterDelete(
    rel: Relationship,
    fromNodeId: string | undefined,
): string | undefined {
    if (fromNodeId) return fromNodeId;
    return rel.from;
}
