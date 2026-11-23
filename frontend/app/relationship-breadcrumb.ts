import { getNode, type Relationship } from "./api";

export type BreadcrumbCrumb = { id: string; name: string; href: string };

/** Type given to containers created by grouping (core DefaultContainerType). */
export const groupingContainerType = "container";

/** A container that only groups its members on the canvas; it adds no level worth a crumb. */
export function isGroupingContainer(node: { type: string }): boolean {
    return node.type.trim().toLowerCase() === groupingContainerType;
}

/**
 * Drops grouping-only containers from a root-to-leaf chain and keeps summary-node ancestors.
 * The last entry (the node being viewed, or the drill source) is always kept. The crumb before
 * the current one is then the level that actually contains it.
 */
export function visibleCrumbs(chain: { id: string; name: string; type: string }[]): BreadcrumbCrumb[] {
    return chain
        .filter((node, index) => index === chain.length - 1 || !isGroupingContainer(node))
        .map((node) => ({ id: node.id, name: node.name, href: `/node/${node.id}` }));
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
    if (!includeStart) {
        const first = await getNode(startId, signal);
        current = first.parent_id || undefined;
    }
    while (current && !seen.has(current)) {
        seen.add(current);
        const node = await getNode(current, signal);
        chain.push({ id: node.id, name: node.name, type: node.type });
        if (node.relationship_id && !node.parent_id) break;
        current = node.parent_id || undefined;
    }
    chain.reverse();
    return visibleCrumbs(chain);
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
