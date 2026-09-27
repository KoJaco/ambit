import React, {
    useCallback,
    useEffect,
    useMemo,
    useRef,
    useState,
} from "react";
import ReactFlow, {
    Background,
    useEdgesState,
    useNodesState,
    addEdge,
    useReactFlow,
    Handle,
    Position,
    type NodeProps,
    type Node,
    type Edge,
    type OnConnectStartParams,
    type NodeChange,
} from "reactflow";
import { NodeResizer } from "@reactflow/node-resizer";
import "@reactflow/node-resizer/dist/style.css";
import "reactflow/dist/style.css";
import { NodeCard } from "./NodeCard";
import type { NodeKind, PortType } from "./types";
import { NodeRegistry } from "./node-registry";
import { isPortCompatible, PortMatrix } from "./ports";
import {
    AudioLinesIcon,
    SparklesIcon,
    StickyNoteIcon,
    SquareIcon,
    LaptopMinimalIcon,
    CodeIcon,
    MailIcon,
    BracesIcon,
    RepeatIcon,
    ClockIcon,
    MergeIcon,
    ActivityIcon,
    CombineIcon,
    ExternalLinkIcon,
    DatabaseIcon,
    XIcon,
} from "lucide-react";
import NodeInspector from "./node-inspector";
import { useUIContext } from "./ui-context";

// Default style for all edges
const DEFAULT_EDGE_STYLE: React.CSSProperties = {
    stroke: "rgb(99 102 241)",
    strokeWidth: 2,
    zIndex: 100,
};

const START_NODE_ID = "audio-in";

// Map registry categories to cohesive brand styles used in the sidebar
const categoryStyles: Record<string, { bg: string; text: string }> = {
    Tools: { bg: "bg-yellow-400", text: "text-yellow-950" },
    LLM: { bg: "bg-indigo-400", text: "text-indigo-950" },
    Logic: { bg: "bg-orange-400", text: "text-orange-950" },
    Data: { bg: "bg-sky-400", text: "text-sky-950" },
    Integrations: { bg: "bg-emerald-400", text: "text-emerald-950" },
    Special: { bg: "bg-rose-400", text: "text-rose-950" },
};

// Reuse the same icon semantics as the sidebar for cohesion
function getIconFor(kind: string, category: string) {
    const { bg, text } = categoryStyles[category] || {
        bg: "bg-foreground/10",
        text: "text-foreground",
    };
    const wrap = (n: React.ReactNode) => (
        <span className={`${bg} p-1 rounded-md ${text}`}>{n}</span>
    );
    const small = "w-3 h-3";
    switch (kind) {
        // Tools
        case "STT":
            return wrap(<AudioLinesIcon className={small} />);
        case "Chunker":
            return wrap(<SquareIcon className={small} />);
        case "EnrichText":
            return wrap(<SparklesIcon className={small} />);
        case "EnrichAudio":
            return wrap(<ActivityIcon className={small} />);
        case "Validator":
            return wrap(<BracesIcon className={small} />);
        // LLM
        case "StructuredOutput":
            return wrap(<BracesIcon className={small} />);
        case "FunctionCall":
            return wrap(<CodeIcon className={small} />);
        case "Summarize":
            return wrap(<StickyNoteIcon className={small} />);
        // Logic
        case "If":
            return wrap(<BracesIcon className={small} />);
        case "Filter":
            return wrap(<BracesIcon className={small} />);
        case "Throttle":
            return wrap(<ClockIcon className={small} />);
        case "Retry":
            return wrap(<RepeatIcon className={small} />);
        case "Fork":
            return wrap(<MergeIcon className={small} />);
        case "Join":
            return wrap(<CombineIcon className={small} />);
        // Data
        case "Transform":
            return wrap(<MergeIcon className={small} />);
        case "State":
            return wrap(<ActivityIcon className={small} />);
        case "ContextComposer":
            return wrap(<CombineIcon className={small} />);
        case "RAGFetch":
            return wrap(<ExternalLinkIcon className={small} />);
        // Integrations
        case "HTTP":
            return wrap(<ExternalLinkIcon className={small} />);
        case "N8N":
            return wrap(<ExternalLinkIcon className={small} />);
        case "Zapier":
            return wrap(<ExternalLinkIcon className={small} />);
        case "DBWrite":
            return wrap(<DatabaseIcon className={small} />);
        case "UIStream":
            return wrap(<LaptopMinimalIcon className={small} />);
        case "Webhook":
            return wrap(<MailIcon className={small} />);
        case "Queue":
            return wrap(<SquareIcon className={small} />);
        // Special
        case "OnEnd":
            return wrap(<SquareIcon className={small} />);
        default:
            return wrap(<SquareIcon className={small} />);
    }
}

// Map sidebar icon keys to lucide icons
function iconFromKey(key?: string): React.ReactNode | undefined {
    switch (key) {
        case "agent":
            return (
                <div className="bg-emerald-400 p-1 rounded-md">
                    <SparklesIcon className="w-3 h-3 text-foreground" />
                </div>
            );
        case "note":
            return (
                <div className="bg-gray-400 p-1 rounded-md">
                    <StickyNoteIcon className="w-3 h-3 text-gray-950" />
                </div>
            );
        case "end":
            return (
                <div className="bg-red-400 p-1 rounded-md">
                    <SquareIcon className="w-3 h-3 text-red-950" />
                </div>
            );

        case "browser":
            return (
                <div className="bg-yellow-400 p-1 rounded-md">
                    <LaptopMinimalIcon className="w-3 h-3 text-yellow-950" />
                </div>
            );
        case "code runner":
            return (
                <div className="bg-yellow-400 p-1 rounded-md">
                    <CodeIcon className="w-3 h-3 text-yellow-950" />
                </div>
            );
        case "email":
            return (
                <div className="bg-yellow-400 p-1 rounded-md">
                    <MailIcon className="w-3 h-3 text-yellow-950" />
                </div>
            );
        case "if-else":
            return (
                <div className="bg-orange-400 p-1 rounded-md">
                    <BracesIcon className="w-3 h-3 text-orange-950" />
                </div>
            );
        case "loop":
            return (
                <div className="bg-orange-400 p-1 rounded-md">
                    <RepeatIcon className="w-3 h-3 text-orange-950" />
                </div>
            );
        case "wait":
            return (
                <div className="bg-orange-400 p-1 rounded-md">
                    <ClockIcon className="w-3 h-3 text-orange-950" />
                </div>
            );
        case "transform":
            return (
                <div className="bg-orange-400 p-1 rounded-md">
                    <MergeIcon className="w-3 h-3 text-orange-950" />
                </div>
            );
        case "state":
            return (
                <div className="bg-orange-400 p-1 rounded-md">
                    <ActivityIcon className="w-3 h-3 text-orange-950" />
                </div>
            );
        case "context":
            return (
                <div className="bg-orange-400 p-1 rounded-md">
                    <CombineIcon className="w-3 h-3 text-orange-950" />
                </div>
            );
        default:
            return undefined;
    }
}

type ControlTool = "grab" | "pointer";

function NodeRF({ id, data, selected }: NodeProps) {
    // Reuse NodeCard visuals; React Flow handles dragging/position
    return (
        <div
            style={{ width: 180, height: 90, position: "relative" }}
            data-node-id={id}
            className="group"
        >
            <NodeCard
                node={{
                    id,
                    displayData: {
                        kind: data.kind,
                        title: data.title,
                        bgColor: data.bgColor,
                        textColor: data.textColor,
                        icon: data.icon,
                    },
                    x: 0,
                    y: 0,
                    width: 180,
                    height: 90,
                }}
                isSelected={!!selected}
                onPointerDown={() => {}}
            />
            {(() => {
                const def = NodeRegistry[data.kind as NodeKind];
                const inPorts: PortType[] = def
                    ? (def.in as PortType[])
                    : data.kind === "audio-in"
                      ? []
                      : (["any"] as PortType[]);
                const outCount = def?.dynamicOutCount
                    ? def.dynamicOutCount(data)
                    : undefined;
                const outPorts: PortType[] = def
                    ? outCount && outCount > 0
                        ? Array.from(
                              { length: outCount },
                              () => def.out?.[0] as PortType
                          )
                        : (def.out as PortType[]) || []
                    : data.kind === "audio-in"
                      ? (["audio"] as PortType[])
                      : (["any"] as PortType[]);
                if (!inPorts.length && !outPorts.length) return null;
                return (
                    <>
                        {/* inputs on left */}
                        {inPorts.map((p: PortType, idx: number) => (
                            <Handle
                                key={`in-${idx}`}
                                type="target"
                                position={Position.Left}
                                id={`in-${idx}`}
                                style={{
                                    width: 8,
                                    height: 8,
                                    backgroundColor: "rgb(99 102 241)",
                                    borderColor: "rgb(99 102 241)",
                                    zIndex: 100,
                                }}
                                className="group-hover:opacity-100 opacity-0"
                                data-port-type={p}
                            />
                        ))}
                        {/* outputs on right */}
                        {outPorts.map((p: PortType, idx: number) => (
                            <Handle
                                key={`out-${idx}`}
                                type="source"
                                position={Position.Right}
                                id={`out-${idx}`}
                                style={{
                                    width: 8,
                                    height: 8,
                                    backgroundColor: "rgb(99 102 241)",
                                    borderColor: "rgb(99 102 241)",
                                    zIndex: 100,
                                }}
                                className="group-hover:opacity-100 opacity-0"
                                data-port-type={p}
                            />
                        ))}
                    </>
                );
            })()}
        </div>
    );
}

// Lane node: resizable background band with a label
function LaneRF({ id, data, selected }: NodeProps) {
    return (
        <div
            style={{
                width: "100%",
                height: "100%",
            }}
            className="flex relative -z-10 border-r border-l border-foreground items-center px-4 relative"
        >
            {/* drag handle at top-right of lane */}
            <div
                className="node-drag-handle absolute top-1 right-1 w-4 h-4 rounded bg-foreground/10 hover:bg-foreground/20 cursor-grab active:cursor-grabbing z-[101]"
                title="Drag"
            />
            <NodeResizer
                isVisible={!!selected}
                minWidth={40}
                minHeight={40}
                handleStyle={{
                    borderColor: "rgb(148 163 184)",
                    width: 8,
                    height: 8,
                }}
                lineStyle={{ borderColor: "rgba(148,163,184,0.6)" }}
            />
            <div className="text-xs absolute -rotate-90 top-2 -left-[30px] font-semibold uppercase tracking-wide text-foreground/80">
                {data?.title ?? "Lane"}
            </div>
        </div>
    );
}

// IfElseRF removed: nodes are display-only; functionality is handled in the inspector

const nodeTypes = { card: NodeRF, lane: LaneRF } as const;

export function FlowCanvas({
    tool = "pointer",
    onRequestRecenterRef,
    onRequestZoomRef,
}: {
    tool?: ControlTool;
    onRequestRecenterRef?: (fn: () => void) => void;
    onRequestZoomRef?: (api: {
        zoomIn: () => void;
        zoomOut: () => void;
    }) => void;
}) {
    const initialNodes = useMemo(
        () => [
            {
                id: START_NODE_ID,
                type: "card",
                data: {
                    title: "Audio In",
                    kind: "audio-in",
                    bgColor: "bg-primary",
                    textColor: "primary-foreground",
                    icon: (
                        <div className="bg-primary p-1 rounded-md">
                            <AudioLinesIcon className="w-3 h-3 text-primary-foreground" />
                        </div>
                    ),
                },
                position: { x: 280, y: 180 },
                width: 180,
                height: 90,
                deletable: false,
                // dragHandle: ".node-drag-handle",
            },
        ],
        []
    );

    const [nodes, setNodes, onNodesChange] = useNodesState(initialNodes as any);
    const [edges, setEdges, onEdgesChange] = useEdgesState([] as any);
    const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
    const [selectedEdgeId, setSelectedEdgeId] = useState<string | null>(null);
    const { pendingNodeId, setPendingNodeId, setConvertPending } =
        useUIContext();
    const newIds = useRef({ node: 3, edge: 2 });
    const connectFromRef = useRef<OnConnectStartParams | null>(null);
    const [toast, setToast] = useState<{ message: string; id: number } | null>(
        null
    );
    const toastIdRef = useRef(0);

    const rf = useReactFlow();

    useEffect(() => {
        onRequestRecenterRef?.(() =>
            rf.fitView({ duration: 300, padding: 0.2 })
        );
    }, [onRequestRecenterRef, rf]);

    // Register conversion handler for sidebar double-clicks
    useEffect(() => {
        setConvertPending(() => (payload: { kind: string; title?: string }) => {
            const { kind, title } = payload;
            if (!pendingNodeId || pendingNodeId === START_NODE_ID) return;
            const actualKind =
                kind === "if-else" ? ("If" as const) : (kind as any);
            const def = NodeRegistry[actualKind as NodeKind];
            if (!def) return;
            setNodes((nds) =>
                nds.map((n) =>
                    n.id === pendingNodeId
                        ? {
                              ...n,
                              data: {
                                  ...n.data,
                                  title: title || def.label,
                                  kind: actualKind,
                                  bgColor:
                                      categoryStyles[def.category]?.bg ||
                                      "bg-foreground/10",
                                  textColor:
                                      categoryStyles[def.category]?.text ||
                                      "text-foreground",
                                  icon: getIconFor(actualKind, def.category),
                                  ...(def.initialData || {}),
                              },
                          }
                        : n
                )
            );
            setPendingNodeId(null);
        });
    }, [pendingNodeId, setConvertPending, setNodes, setPendingNodeId]);

    useEffect(() => {
        onRequestZoomRef?.({
            zoomIn: () => rf.zoomIn({ duration: 150 }),
            zoomOut: () => rf.zoomOut({ duration: 150 }),
        });
    }, [onRequestZoomRef, rf]);

    // memoized type maps to avoid re-creation warnings
    const nodeTypesMemo = useMemo(() => nodeTypes, []);

    // Prevent deletion of the start node by filtering out remove changes
    const handleNodesChange = useCallback(
        (changes: NodeChange[]) => {
            const filtered = changes.filter(
                (c) => !(c.type === "remove" && (c as any).id === START_NODE_ID)
            );
            onNodesChange(filtered);
        },
        [onNodesChange]
    );

    // Track rewiring start
    const onConnectStart = useCallback(
        (_: any, params: OnConnectStartParams) => {
            connectFromRef.current = params;
        },
        []
    );

    const onConnectEnd = useCallback(
        (event: any) => {
            // If the user ended a drag from a handle but didn't connect to a target,
            // create a blank card at the released position and preselect it.
            const started = connectFromRef.current;
            if (started) {
                const isMouse = (ev: any): ev is MouseEvent =>
                    !!(ev && (ev as MouseEvent).clientX !== undefined);
                const isTouch = (ev: any): ev is TouchEvent =>
                    !!(ev && (ev as TouchEvent).changedTouches !== undefined);
                let clientX: number | undefined;
                let clientY: number | undefined;
                if (isMouse(event)) {
                    clientX = event.clientX;
                    clientY = event.clientY;
                } else if (isTouch(event)) {
                    clientX = event.changedTouches[0]?.clientX;
                    clientY = event.changedTouches[0]?.clientY;
                }
                const targetEl = (event?.target as HTMLElement) || null;
                const droppedOnPane =
                    !!targetEl &&
                    (targetEl.classList.contains("react-flow__pane") ||
                        targetEl.classList.contains("react-flow__viewport") ||
                        targetEl.closest(".react-flow__pane"));
                if (droppedOnPane && clientX != null && clientY != null) {
                    const pos = rf.screenToFlowPosition({
                        x: clientX,
                        y: clientY,
                    });
                    const id = `n${newIds.current.node++}`;
                    const width = 180;
                    const height = 90;
                    setNodes((nds) => [
                        ...nds,
                        {
                            id,
                            type: "card",
                            data: {
                                title: "Select a node",
                                // undefined/unknown kind marks this as an empty card
                                kind: undefined,
                                bgColor: "bg-card",
                                textColor: "text-foreground",
                                icon: undefined,
                            },
                            position: {
                                x: pos.x - width / 2,
                                y: pos.y - height / 2,
                            },
                            width,
                            height,
                        } as unknown as Node,
                    ]);
                    // Preselect and mark as pending selection so a palette drop converts it
                    setSelectedNodeId(id);
                    setPendingNodeId(id);
                    // Create edge from the start to this new node
                    setEdges((eds) => [
                        ...eds,
                        addEdge(
                            {
                                source: started.nodeId as string,
                                sourceHandle:
                                    (started.handleId as string) ?? undefined,
                                target: id,
                                targetHandle: "in-0",
                                style: DEFAULT_EDGE_STYLE,
                            } as any,
                            []
                        )[0],
                    ]);
                    // Optional: nudge the user
                    const toastId = ++toastIdRef.current;
                    setToast({
                        message: "Select a node from the sidebar",
                        id: toastId,
                    });
                    setTimeout(() => {
                        setToast((t) => (t && t.id === toastId ? null : t));
                    }, 2000);
                }
            }
            connectFromRef.current = null;
        },
        [rf, setNodes, setEdges]
    );

    const resolvePortType = useCallback(
        (
            nodeId?: string,
            handleId?: string,
            role?: "source" | "target"
        ): PortType | undefined => {
            if (!nodeId || !handleId) return undefined;
            const n = (nodes as any[]).find((x) => x.id === nodeId);
            if (!n) return undefined;
            if (n.type === "lane") return undefined;
            const kind = n.data?.kind as NodeKind | string | undefined;
            if (kind === "audio-in") {
                return role === "source" ? ("audio" as PortType) : undefined;
            }
            const def = NodeRegistry[kind as NodeKind];
            if (!def) return undefined;
            const match = /-(\d+)$/.exec(handleId);
            const idx = match ? parseInt(match[1], 10) : 0;
            if (role === "source") return def.out[idx] ?? def.out[0];
            return def.in[idx] ?? def.in[0];
        },
        [nodes]
    );

    const onConnect = useCallback(
        (params: any) => {
            const start = connectFromRef.current;
            if (start) {
                setEdges((eds) => {
                    const sourcePort = resolvePortType(
                        params.source,
                        params.sourceHandle,
                        "source"
                    );
                    const targetPort = resolvePortType(
                        params.target,
                        params.targetHandle,
                        "target"
                    );
                    if (
                        sourcePort &&
                        targetPort &&
                        !isPortCompatible(sourcePort, targetPort)
                    ) {
                        const allowed = (PortMatrix[sourcePort] || []).join(
                            ", "
                        );
                        const id = ++toastIdRef.current;
                        setToast({
                            message: `Cannot connect ${sourcePort} → ${targetPort}. Allowed: ${allowed || "(none)"}`,
                            id,
                        });
                        setTimeout(() => {
                            setToast((t) => (t && t.id === id ? null : t));
                        }, 2800);
                        return eds;
                    }
                    return addEdge(
                        { ...params, style: DEFAULT_EDGE_STYLE },
                        eds
                    );
                });
                connectFromRef.current = null;
                return;
            }
            // Default create with style
            setEdges((eds) => {
                const sourcePort = resolvePortType(
                    params.source,
                    params.sourceHandle,
                    "source"
                );
                const targetPort = resolvePortType(
                    params.target,
                    params.targetHandle,
                    "target"
                );
                if (
                    sourcePort &&
                    targetPort &&
                    !isPortCompatible(sourcePort, targetPort)
                ) {
                    const allowed = (PortMatrix[sourcePort] || []).join(", ");
                    const id = ++toastIdRef.current;
                    setToast({
                        message: `Cannot connect ${sourcePort} → ${targetPort}. Allowed: ${allowed || "(none)"}`,
                        id,
                    });
                    setTimeout(() => {
                        setToast((t) => (t && t.id === id ? null : t));
                    }, 2800);
                    return eds;
                }
                return addEdge({ ...params, style: DEFAULT_EDGE_STYLE }, eds);
            });
        },
        [setEdges, nodes]
    );

    return (
        <div className="relative h-[calc(100vh)] w-full">
            <ReactFlow
                nodeTypes={nodeTypesMemo as any}
                nodes={nodes as any}
                edges={edges as any}
                defaultEdgeOptions={{
                    type: "smoothstep",
                }}
                onNodesChange={handleNodesChange}
                onEdgesChange={onEdgesChange}
                onConnect={onConnect}
                onConnectStart={onConnectStart}
                onConnectEnd={onConnectEnd}
                onDragOver={(e: React.DragEvent) => {
                    // Always prevent default so drop fires reliably
                    e.preventDefault();
                    e.dataTransfer.dropEffect = "move";
                }}
                onDrop={(e: React.DragEvent) => {
                    e.preventDefault();
                    const raw = e.dataTransfer.getData("application/reactflow");
                    if (!raw) return;
                    try {
                        const payload = JSON.parse(raw) as {
                            title: string;
                            kind: string;
                        };
                        // If we're currently selecting a node type for a pending empty card,
                        // convert that card into the dropped kind instead of creating a new one.
                        if (pendingNodeId && pendingNodeId !== START_NODE_ID) {
                            const def =
                                payload.kind === "if-else"
                                    ? NodeRegistry["If"]
                                    : NodeRegistry[payload.kind as NodeKind];
                            if (def) {
                                setNodes((nds) =>
                                    nds.map((n) =>
                                        n.id === pendingNodeId
                                            ? {
                                                  ...n,
                                                  data: {
                                                      ...n.data,
                                                      title: def.label,
                                                      kind:
                                                          payload.kind ===
                                                          "if-else"
                                                              ? ("If" as const)
                                                              : (payload.kind as any),
                                                      bgColor:
                                                          categoryStyles[
                                                              def.category
                                                          ]?.bg ||
                                                          "bg-foreground/10",
                                                      textColor:
                                                          categoryStyles[
                                                              def.category
                                                          ]?.text ||
                                                          "text-foreground",
                                                      icon: getIconFor(
                                                          payload.kind ===
                                                              "if-else"
                                                              ? "If"
                                                              : payload.kind,
                                                          def.category
                                                      ),
                                                      ...(def.initialData ||
                                                          {}),
                                                  },
                                              }
                                            : n
                                    )
                                );
                                setPendingNodeId(null);
                                return;
                            }
                        }
                        const pos = rf.screenToFlowPosition({
                            x: e.clientX,
                            y: e.clientY,
                        });
                        const id = `n${newIds.current.node++}`;
                        // create from registry
                        const def = NodeRegistry[payload.kind as NodeKind];
                        if (def) {
                            setNodes((nds) => [
                                ...nds,
                                {
                                    id,
                                    type: "card",
                                    data: {
                                        title: def.label,
                                        kind: payload.kind,
                                        bgColor:
                                            categoryStyles[def.category]?.bg ||
                                            "bg-foreground/10",
                                        textColor:
                                            categoryStyles[def.category]
                                                ?.text || "text-foreground",
                                        icon: getIconFor(
                                            payload.kind,
                                            def.category
                                        ),
                                    },
                                    position: pos,
                                    width: 180,
                                    height: 90,
                                } as unknown as Node,
                            ]);
                            return;
                        }
                        if (payload.kind === "lane") {
                            setNodes((nds) => [
                                ...nds,
                                {
                                    id,
                                    type: "lane",
                                    data: {
                                        title: payload.title,
                                    },
                                    position: { x: pos.x - 450, y: pos.y - 60 },
                                    style: { width: 900, height: 120 },
                                    // dragHandle: ".node-drag-handle",
                                    draggable: true,
                                    selectable: true,
                                } as unknown as Node,
                            ]);
                            return;
                        }
                        if (payload.kind === "if-else") {
                            const def = NodeRegistry["If"];
                            setNodes((nds) => [
                                ...nds,
                                {
                                    id,
                                    type: "card",
                                    data: {
                                        title: payload.title || def.label,
                                        kind: "If",
                                        bgColor:
                                            categoryStyles[def.category]?.bg ||
                                            "bg-foreground/10",
                                        textColor:
                                            categoryStyles[def.category]
                                                ?.text || "text-foreground",
                                        icon: getIconFor("If", def.category),
                                        ...(def.initialData || {}),
                                    },
                                    position: { x: pos.x - 110, y: pos.y - 60 },
                                    width: 180,
                                    height: 90,
                                } as unknown as Node,
                            ]);
                            return;
                        }
                        // fallback generic card if not found (should not happen)
                        setNodes((nds) => [
                            ...nds,
                            {
                                id,
                                type: "card",
                                data: {
                                    title: payload.title,
                                    kind: payload.kind,
                                    bgColor: "bg-card",
                                    textColor: "text-foreground",
                                    icon: undefined,
                                },
                                position: pos,
                                width: 180,
                                height: 90,
                            } as unknown as Node,
                        ]);
                    } catch {}
                }}
                nodesDraggable={tool === "grab"}
                panOnDrag={tool === "grab"}
                elementsSelectable={true}
                onNodeClick={(_, n) => {
                    setSelectedNodeId(n.id);
                    const k = (n.data as any)?.kind as string | undefined;
                    const known = k && (NodeRegistry as any)[k as any];
                    if (!known && n.type === "card" && n.id !== START_NODE_ID) {
                        setPendingNodeId(n.id);
                        const id = ++toastIdRef.current;
                        setToast({
                            message: "Select a node from the sidebar",
                            id,
                        });
                        setTimeout(() => {
                            setToast((t) => (t && t.id === id ? null : t));
                        }, 1800);
                    }
                }}
                onEdgeClick={(_, e) => {
                    setSelectedEdgeId(e.id);
                    setSelectedNodeId(null);
                }}
                onPaneClick={() => {
                    setSelectedNodeId(null);
                    setSelectedEdgeId(null);
                }}
                fitView
            >
                <Background gap={12} />
            </ReactFlow>

            {toast && (
                <div className="absolute bottom-4 right-4 max-w-sm z-[200]">
                    <div className="px-3 py-2 text-xs rounded-md border border-foreground/20 bg-background/95 shadow-sm">
                        {toast.message}
                    </div>
                </div>
            )}

            <NodeInspector
                selectedNodeId={selectedNodeId}
                selectedEdgeId={selectedEdgeId}
                nodes={nodes as unknown as Node[]}
                edges={edges as unknown as Edge[]}
                onClose={() => {
                    setSelectedNodeId(null);
                    setSelectedEdgeId(null);
                }}
                resolvePortType={resolvePortType}
            />
        </div>
    );
}

export default FlowCanvas;
