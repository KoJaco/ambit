// Edge stubs: relationships that leave the current level are drawn as small "external" cards at
// the edge of the canvas, joined to the on-screen node they attach to. Pure so it can be tested.

import type { Crossing, Relationship } from "./api";

export type StubSide = "left" | "right";

export type ExternalStub = {
    id: string;
    // The on-screen node the stub attaches to.
    nodeId: string;
    otherId: string;
    direction: string;
    // Side of the stub that gets the straight (unrounded) edge: the one facing away from the relationship.
    straightSide: StubSide;
    // Relationship label, used until the other node's name is known.
    label: string;
    relationshipId?: string;
    drillable: boolean;
};

export type StubEdge = {
    id: string;
    source: string;
    target: string;
    kind: string;
    rel: Relationship;
};

export const stubIdPrefix = "ext:";

export function isStubId(id: string): boolean {
    return id.startsWith(stubIdPrefix);
}

// Layout runs left to right. An incoming relationship leaves the stub on its right, so the stub
// sits left of the node and its left side is straight; outgoing mirrors that.
export function straightSideFor(direction: string): StubSide {
    return direction === "in" ? "left" : "right";
}

export function buildExternalStubs(crossings: Crossing[] | undefined): { stubs: ExternalStub[]; edges: StubEdge[] } {
    const stubs: ExternalStub[] = [];
    const edges: StubEdge[] = [];
    const seen = new Set<string>();
    for (const crossing of crossings ?? []) {
        const base = crossing.relationship_id || `${crossing.node_id}>${crossing.direction}>${crossing.other_id}`;
        const id = `${stubIdPrefix}${base}`;
        if (seen.has(id)) continue;
        seen.add(id);
        const incoming = crossing.direction === "in";
        stubs.push({
            id,
            nodeId: crossing.node_id,
            otherId: crossing.other_id,
            direction: crossing.direction,
            straightSide: straightSideFor(crossing.direction),
            label: crossing.label,
            relationshipId: crossing.relationship_id,
            drillable: Boolean(crossing.drillable && crossing.relationship_id),
        });
        const from = incoming ? crossing.other_id : crossing.node_id;
        const to = incoming ? crossing.node_id : crossing.other_id;
        edges.push({
            id: `${id}:edge`,
            source: incoming ? id : crossing.node_id,
            target: incoming ? crossing.node_id : id,
            kind: crossing.kind,
            rel: {
                id: crossing.relationship_id || id,
                from,
                to,
                label: crossing.label,
                kind: crossing.kind,
                drillable: crossing.drillable,
            },
        });
    }
    return { stubs, edges };
}

// Name shown on a stub: the other node's name once known, else the relationship label, else its id.
export function stubTitle(name: string | undefined, label: string, otherId: string): string {
    return name?.trim() || label.trim() || otherId;
}
