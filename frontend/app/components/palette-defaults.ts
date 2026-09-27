import type { NodeKind } from "./types";

export const SidebarGroups: Record<string, NodeKind[]> = {
    Tools: ["STT", "Chunker", "EnrichText", "EnrichAudio", "Validator"],
    LLM: ["StructuredOutput", "FunctionCall", "Summarize"],
    Logic: ["If", "Filter", "Throttle", "Retry", "Fork", "Join"],
    Data: ["Transform", "State", "ContextComposer", "RAGFetch"],
    Integrations: [
        "HTTP",
        "N8N",
        "Zapier",
        "DBWrite",
        "UIStream",
        "Webhook",
        "Queue",
    ],
    Special: ["OnEnd"], // Ingest is always on canvas, not in palette
};
