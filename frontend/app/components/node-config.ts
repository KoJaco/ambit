// Config type helpers for common nodes

export interface IngestConfig {
    source: "realtime" | "file";
    audio?: {
        sample_rate: number;
        channels: 1 | 2;
        vad: boolean;
        diarization: boolean;
    };
    file?: {
        accept: string[];
        max_mb: number;
        chunk_policy: "per_file" | "sentence_boundary";
    };
}

export interface STTConfig {
    provider: "livekit" | "openai" | "deepgram" | "whisper" | "custom";
    model: string;
    language?: string;
    punctuation?: boolean;
    numerals?: "asr" | "verbatim" | "normalize";
    diarization?: "inherit" | "model" | "none";
}

export interface ChunkerConfig {
    trigger:
        | "on_final"
        | "every_N_chars"
        | "max_latency_ms"
        | "sentence_boundary"
        | "manual_signal";
    every_N_chars?: number;
    max_latency_ms?: number;
}

export type EnrichTextOp =
    | {
          op: "normalize";
          case?: "as_is" | "lower" | "upper";
          numerals?: "none" | "to_int";
          punctuation?: "keep" | "strip";
      }
    | {
          op: "redact";
          types: ("PHONE" | "EMAIL" | "NAME" | "ADDRESS" | "IP")[];
          strategy: "mask" | "hash" | "drop";
      }
    | {
          op: "entity_extract";
          schema_ref: string;
          threshold?: number;
          accumulate?: boolean;
      }
    | {
          op: "datetime_resolve";
          reference: "session_start" | "now" | "custom";
          locale?: string;
      }
    | { op: "keyword_spot"; phrases: string[]; sensitivity?: number }
    | { op: "custom_regex"; rules: { find: string; replace: string }[] };

export interface EnrichTextConfig {
    chunk_strategy?: "inherit" | ChunkerConfig["trigger"];
    ops: EnrichTextOp[];
}

export type EnrichAudioOp =
    | { op: "spectrogram_classify"; model: string }
    | { op: "noise_profile" }
    | { op: "speaker_activity"; smoothing_ms?: number }
    | { op: "emotion" }
    | { op: "wakeword"; phrases: string[]; sensitivity?: number };

export interface EnrichAudioConfig {
    stride_ms?: number;
    aggregate_window_ms?: number;
    emit_policy?: "every_frame" | "every_N_frames" | "on_change";
    ops: EnrichAudioOp[];
}

export interface ValidatorConfig {
    schema_ref: string; // JSONSchema/TypeBox id
    strict?: boolean;
    coerce?: boolean;
}

export interface DBWriteConfig {
    provider: "postgres" | "supabase" | "mysql" | "firestore";
    connection: string; // SECRET: alias or DSN key
    table: string;
    op: "insert" | "upsert" | "update";
    keys?: string[];
    mapping: Record<string, string>; // "db_field": "$.json.path::int"
    batch?: { size: number; flush_ms: number };
    on_fail?: {
        strategy: "retry" | "dlq" | "skip";
        max_attempts?: number;
        backoff_ms?: number;
        dlq_webhook?: string;
    };
}
