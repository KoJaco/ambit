import type { PortType } from "./types";

export const PortMatrix: Record<PortType, PortType[]> = {
    any: [
        "audio",
        "transcript_chunk",
        "text",
        "text_chunk",
        "json",
        "entities",
        "events",
        "context",
        "labels",
        "features",
        "summary",
        "ack",
        "meta",
        "any",
    ],
    audio: ["audio", "transcript_chunk", "labels", "events", "features"], // audio → STT | EnrichAudio
    transcript_chunk: [
        "text_chunk",
        "text",
        "json",
        "entities",
        "events",
        "summary",
        "context",
    ], // allow direct to LLM/Text if needed
    text: ["json", "entities", "events", "summary", "context", "ack"],
    text_chunk: ["text", "json", "entities", "events", "summary", "context"],
    json: ["json", "ack", "summary", "context"], // transform/validate → integrations
    entities: ["json", "context", "ack"],
    events: ["json", "ack"],
    context: ["json", "summary"],
    labels: ["events", "json", "ack"],
    features: ["events", "json", "ack"],
    summary: ["json", "ack"],
    ack: [], // sinks only
    meta: ["json", "context", "events"],
};

export function isPortCompatible(from: PortType, to: PortType): boolean {
    if (from === "any" || to === "any") return true;
    const allowed = PortMatrix[from] || [];
    return allowed.includes(to);
}
