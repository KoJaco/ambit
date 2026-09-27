import type { NodeKind, PortType } from "./types";

import type {
    STTConfig,
    ChunkerConfig,
    EnrichTextConfig,
    EnrichAudioConfig,
    ValidatorConfig,
    DBWriteConfig,
} from "./node-config";

// Reusable “factory” defaults for each node kind
export type RegistryNode = {
    kind: NodeKind;
    in: PortType[];
    out: PortType[];
    label: string;
    category: "Tools" | "LLM" | "Logic" | "Data" | "Integrations" | "Special";
    description?: string;
    config?: any;
    // Optional default data seeded when creating this node on the canvas
    initialData?: any;
    // Optional dynamic output port count computed from node data; if provided,
    // the node renderer should create this many source handles. Types are
    // resolved using def.out[idx] with fallback to def.out[0].
    dynamicOutCount?: (data: any) => number;
};

export const NodeRegistry: Record<NodeKind, RegistryNode> = {
    STT: {
        kind: "STT",
        in: ["audio"],
        out: ["transcript_chunk"],
        label: "STT",
        category: "Tools",
        description:
            "Converts incoming audio into text transcripts using a pluggable speech-to-text provider.",

        config: <STTConfig>{
            provider: "livekit",
            model: "default",
            language: "en",
            punctuation: true,
            numerals: "asr",
            diarization: "inherit",
        },
    },
    Chunker: {
        kind: "Chunker",
        in: ["transcript_chunk", "text"],
        out: ["text_chunk"],
        label: "Chunker",
        category: "Tools",
        description:
            "Splits transcripts or text into smaller chunks based on timing or character limits.",

        config: <ChunkerConfig>{ trigger: "on_final" },
    },
    EnrichText: {
        kind: "EnrichText",
        in: ["text", "text_chunk"],
        out: ["text", "entities", "events", "json"],
        label: "Enrich (Text)",
        category: "Tools",
        description:
            "Processes text with enrichment ops such as normalization, redaction, or entity extraction.",

        config: <EnrichTextConfig>{ chunk_strategy: "inherit", ops: [] },
    },
    EnrichAudio: {
        kind: "EnrichAudio",
        in: ["audio"],
        out: ["labels", "events", "features"],
        label: "Enrich (Audio)",
        category: "Tools",
        description:
            "Analyzes raw audio for features like noise, speaker activity, or classifications.",

        config: <EnrichAudioConfig>{ emit_policy: "on_change", ops: [] },
    },
    Validator: {
        kind: "Validator",
        in: ["json"],
        out: ["json"],
        label: "Validator",
        category: "Tools",
        description:
            "Validates structured data against a JSON schema, coercing or rejecting invalid fields.",

        config: <ValidatorConfig>{
            schema_ref: "schemas.default",
            strict: true,
            coerce: true,
        },
    },
    StructuredOutput: {
        kind: "StructuredOutput",
        in: ["text", "context"],
        out: ["json"],
        label: "Structured Output",
        category: "LLM",
        description:
            "Uses an LLM to convert text into structured JSON that matches a defined schema.",

        config: {
            schema_ref: "schemas.default",
            retries: 2,
            guardrails: "coerce",
            budget_tokens: 800,
        },
    },
    FunctionCall: {
        kind: "FunctionCall",
        in: ["text", "context"],
        out: ["json", "events"],
        label: "Function Call",
        category: "LLM",
        description:
            "Invokes functions or tools via LLM function calling and emits their results.",

        config: { tools: [], timeout_ms: 6000, deterministic: false },
    },
    Summarize: {
        kind: "Summarize",
        in: ["text", "context"],
        out: ["summary"],
        label: "Summarize/Note",
        category: "LLM",
        description:
            "Generates short summaries or notes from transcripts or contextual text.",

        config: { length: "short", style: "plain" },
    },
    If: {
        kind: "If",
        in: ["any" as PortType],
        out: ["any" as PortType],
        label: "If / Switch",
        category: "Logic",
        description:
            "Routes data based on an expression evaluation, creating conditional branches.",

        config: { expr: "$.json.qty > 0" },
        initialData: { branches: [0], branchTitles: {} },
        dynamicOutCount: (data: any) =>
            Array.isArray(data?.branches) && data.branches.length > 0
                ? data.branches.length
                : 1,
    },
    Filter: {
        kind: "Filter",
        in: ["json", "text", "entities", "events"],
        out: ["json", "text", "entities", "events"],
        label: "Filter",
        category: "Logic",
        description: "Passes data only if it matches the specified condition.",

        config: { expr: "$exists($.json.job_id)" },
    },
    Throttle: {
        kind: "Throttle",
        in: ["any" as PortType],
        out: ["any" as PortType],
        label: "Throttle / Debounce",
        category: "Logic",
        description:
            "Limits how often downstream nodes receive events to reduce overload.",

        config: { window_ms: 200 },
    },
    Retry: {
        kind: "Retry",
        in: ["any" as PortType],
        out: ["any" as PortType],
        label: "Retry (wrapper)",
        category: "Logic",
        description:
            "Retries execution of its downstream node on failure with backoff timing.",

        config: { attempts: 3, backoff_ms: 400 },
    },
    Fork: {
        kind: "Fork",
        in: ["any" as PortType],
        out: ["any" as PortType],
        label: "Fork",
        category: "Logic",
        description:
            "Splits a single input into multiple parallel execution branches.",

        config: {},
    },
    Join: {
        kind: "Join",
        in: ["any" as PortType],
        out: ["any" as PortType],
        label: "Join",
        category: "Logic",
        description:
            "Waits for multiple branches and merges their outputs into one stream.",

        config: { policy: "all", timeout_ms: 5000 },
    },
    Transform: {
        kind: "Transform",
        in: ["json"],
        out: ["json"],
        label: "Transform (JQ/jsonata)",
        category: "Data",
        description:
            "Maps or reshapes JSON data using JQ or JSONata expressions.",

        config: { map: { job_id: "$.json.job.id" } },
    },
    State: {
        kind: "State",
        in: ["json"],
        out: ["json"],
        label: "State Read/Write",
        category: "Data",
        description:
            "Stores or retrieves intermediate state data within the current session.",

        config: { op: "merge", key: "session", ttl_ms: 3600_000 },
    },
    ContextComposer: {
        kind: "ContextComposer",
        in: ["entities", "json", "context"],
        out: ["context"],
        label: "Context Composer",
        category: "Data",
        description:
            "Combines entities, documents, and state into a prompt context for LLM nodes.",

        config: {
            template: "Given entities: {{entities}} and state: {{state}}",
        },
    },
    RAGFetch: {
        kind: "RAGFetch",
        in: ["text", "entities"],
        out: ["context"],
        label: "RAG Fetch",
        category: "Data",
        description:
            "Retrieves relevant context documents from a vector index for retrieval-augmented generation.",

        config: { index: "default", topK: 3, filters: {} },
    },
    HTTP: {
        kind: "HTTP",
        in: ["json"],
        out: ["ack", "json"],
        label: "HTTP Action",
        category: "Integrations",
        description:
            "Makes an HTTP request to an external API with retry and response handling.",

        config: {
            method: "POST",
            url: "https://api.example.com",
            retry: { attempts: 3 },
        },
    },
    N8N: {
        kind: "N8N",
        in: ["json"],
        out: ["ack"],
        label: "n8n Trigger",
        category: "Integrations",
        description:
            "Sends structured output to an n8n webhook to trigger downstream automations.",

        config: { url: "https://n8n.example/hook", secret: "SECRET:N8N" },
    },
    Zapier: {
        kind: "Zapier",
        in: ["json"],
        out: ["ack"],
        label: "Zapier Trigger",
        category: "Integrations",
        description:
            "Triggers a Zapier webhook to start a Zap from structured Schma data.",

        config: { url: "https://hooks.zapier.com/xyz" },
    },
    DBWrite: {
        kind: "DBWrite",
        in: ["json"],
        out: ["ack"],
        label: "DB Write (External)",
        category: "Integrations",
        description:
            "Writes structured JSON data into an external database such as Postgres or Supabase.",

        config: <DBWriteConfig>{
            provider: "postgres",
            connection: "SECRET:PG_URL",
            table: "job_cards",
            op: "upsert",
            keys: ["job_id"],
            mapping: { job_id: "$.json.job.id", client: "$.json.client.name" },
            batch: { size: 20, flush_ms: 250 },
            on_fail: { strategy: "retry", max_attempts: 5, backoff_ms: 500 },
        },
    },
    UIStream: {
        kind: "UIStream",
        in: ["json", "text", "entities", "summary"],
        out: ["ack"],
        label: "UI Stream Out",
        category: "Integrations",
        description:
            "Streams data back to connected clients for live updates or visualization.",

        config: { channel: "default" },
    },
    Webhook: {
        kind: "Webhook",
        in: ["json", "events", "summary"],
        out: ["ack"],
        label: "Webhook Out",
        category: "Integrations",
        description:
            "Sends a payload to an arbitrary webhook endpoint for external processing.",

        config: { url: "https://example.com/hook", hmac_secret: "SECRET:HMAC" },
    },
    Queue: {
        kind: "Queue",
        in: ["json"],
        out: ["ack"],
        label: "Queue Out",
        category: "Integrations",
        description:
            "Publishes messages to a message queue such as SQS or NATS for asynchronous processing.",

        config: { provider: "sqs", queue: "schma-events" },
    },

    OnEnd: {
        kind: "OnEnd",
        in: ["json", "entities", "events", "context"],
        out: [],
        label: "On End (Finalize)",
        category: "Special",
        description:
            "Executes cleanup or export actions once audio ingestion completes.",
        config: { actions: ["flush_accumulators", "write_report", "notify"] },
    },
};
