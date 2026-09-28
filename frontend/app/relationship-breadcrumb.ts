import { getNode, type Relationship } from "./api";

export type BreadcrumbCrumb = { id: string; name: string; href: string };

/** Ancestors from root down to startId (inclusive when includeStart). */
export async function nodeAncestorChain(
    startId: string,
    signal?: AbortSignal,
    includeStart = false,
): Promise<BreadcrumbCrumb[]> {
    const chain: BreadcrumbCrumb[] = [];
    const seen = new Set<string>();
    let current: string | undefined = startId;
    if (!includeStart) {
        const first = await getNode(startId, signal);
        current = first.parent_id || undefined;
    }
    while (current && !seen.has(current)) {
        seen.add(current);
        const node = await getNode(current, signal);
        chain.push({ id: node.id, name: node.name, href: `/node/${node.id}` });
        if (node.relationship_id && !node.parent_id) break;
        current = node.parent_id || undefined;
    }
    chain.reverse();
    return chain;
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
