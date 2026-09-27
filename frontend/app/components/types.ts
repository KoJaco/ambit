export type ControlBarTool =
    | "grab"
    | "pointer"
    | "undo"
    | "redo"
    | "zoom-in"
    | "zoom-out"
    | "recenter";

export type SidebarSection = {
    title: string;
    items: SidebarItem[];
};

export type SidebarItem = {
    label: string;
    displayTitle?: string;
    icon?: React.ReactNode;
};

// export type NodeKind =
//     | "agent"
//     | "note"
//     | "audio-in"
//     | "if-else"
//     | "loop"
//     | "wait"
//     | "transform"
//     | "state"
//     | "context"
//     | "browser"
//     | "code"
//     | "email"
//     | "lane";

export type CanvasNode<T = any> = {
    id: string;
    in: PortType[];
    out: PortType[];
    kind: NodeKind | SpecialKind;
    x: number; // left position
    y: number; // top position
    width: number;
    height: number;
    config?: T; // node-specific config
    special?: SpecialKind; // marks special nodes
    displayData: {
        kind: string;
        title: string;
        bgColor?: string;
        textColor?: string;
        icon?: React.ReactNode;
    };
};

export type CanvasConnection = {
    id: string;
    from: string; // node id
    to: string; // node id
};

// Core port types your canvas connects
export type PortType =
    | "any"
    | "audio"
    | "transcript_chunk"
    | "text"
    | "text_chunk"
    | "json"
    | "entities"
    | "events"
    | "context"
    | "labels"
    | "features"
    | "summary"
    | "ack"
    | "meta";

// Node kinds shown in the sidebar (Ingest is special; not in palette)
export type NodeKind =
    | "STT"
    | "Chunker"
    | "EnrichText"
    | "EnrichAudio"
    | "Validator"
    | "StructuredOutput"
    | "FunctionCall"
    | "Summarize"
    | "If"
    | "Filter"
    | "Throttle"
    | "Retry"
    | "Fork"
    | "Join"
    | "Transform"
    | "State"
    | "ContextComposer"
    | "RAGFetch"
    | "HTTP"
    | "N8N"
    | "Zapier"
    | "DBWrite"
    | "UIStream"
    | "Webhook"
    | "Queue"
    | "OnEnd";

export type SpecialKind = "Ingest" | "OnEnd";

export interface Edge {
    id: string;
    from: { nodeId: string; port?: PortType };
    to: { nodeId: string; port?: PortType };
}

export interface Flow {
    id: string;
    name: string;
    nodes: CanvasNode[];
    edges: Edge[];
    // global defaults that downstream nodes can "inherit"
    defaults?: {
        chunk_strategy?:
            | "inherit"
            | "on_final"
            | "every_N_chars"
            | "max_latency_ms"
            | "sentence_boundary"
            | "manual_signal";
        language?: string;
        redaction_policy?: "off" | "mask" | "hash";
    };
}

export type ChunkStrategy = NonNullable<Flow["defaults"]>["chunk_strategy"];
