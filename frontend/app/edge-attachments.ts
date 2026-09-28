import type { EdgeAttachment, Layout, NodePorts } from "./api";

export function defaultAttachment(): EdgeAttachment {
    return { source: "right:0", target: "left:0" };
}

export function flipAttachment(att: EdgeAttachment): EdgeAttachment {
    const flipSide = (side: string) => {
        const [s, i] = side.split(":");
        if (s === "left") return `right:${i ?? "0"}`;
        return `left:${i ?? "0"}`;
    };
    return { source: flipSide(att.source), target: flipSide(att.target) };
}

export function ensurePorts(layout: Layout, nodeId: string, side: "left" | "right"): Layout {
    const ports = { ...(layout.ports ?? {}) };
    const cur = ports[nodeId] ?? { left: 1, right: 1 };
    ports[nodeId] = { ...cur, [side]: cur[side] + 1 };
    return { ...layout, ports };
}

export function bumpPort(layout: Layout, nodeId: string, side: "left" | "right"): Layout {
    return ensurePorts(layout, nodeId, side);
}
