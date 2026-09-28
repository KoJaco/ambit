import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
    applyNodeChanges,
    Background,
    MarkerType,
    ReactFlow,
    useReactFlow,
    type Edge,
    type NodeChange,
    type NodeMouseHandler,
} from "@xyflow/react";
import { useNavigate } from "react-router";
import "@xyflow/react/dist/style.css";
import { getLayout, getLevel, layoutCacheKey, putLayout, type Level } from "../api";
import { layoutLevel, type Point } from "../layout";
import { NodeCard, type ArchNode, type ArchNodeData, type CrossingMark } from "./NodeCard";

const nodeTypes = { arch: NodeCard };

function crossingMarks(childId: string, crossings: Level["crossings"]): CrossingMark[] {
    return (crossings ?? [])
        .filter((crossing) => crossing.node_id === childId)
        .map((crossing) => ({
            direction: crossing.direction,
            label: crossing.label,
            otherId: crossing.other_id,
        }));
}

function nodeData(child: Level["children"][number], crossings: Level["crossings"]): ArchNodeData {
    return {
        name: child.name,
        type: child.type,
        status: child.status,
        protected: child.protected,
        crossings: crossingMarks(child.id, crossings),
    };
}

function buildArchNodes(level: Level, positions: Record<string, Point>): ArchNode[] {
    return (level.children ?? []).map((child, index) => ({
        id: child.id,
        type: "arch",
        position: positions[child.id] ?? {
            x: (index % 4) * 280,
            y: Math.floor(index / 4) * 160,
        },
        data: nodeData(child, level.crossings),
    }));
}

function sameData(left: ArchNodeData, right: ArchNodeData): boolean {
    if (
        left.name !== right.name ||
        left.type !== right.type ||
        left.status !== right.status ||
        left.protected !== right.protected ||
        left.crossings.length !== right.crossings.length
    ) {
        return false;
    }
    return left.crossings.every(
        (mark, index) =>
            mark.direction === right.crossings[index].direction &&
            mark.label === right.crossings[index].label &&
            mark.otherId === right.crossings[index].otherId,
    );
}

export default function FlowCanvas({
    nodeId,
    tool,
    refreshKey = 0,
    onSelectNode,
    onRequestRecenterRef,
    onRequestZoomRef,
}: {
    nodeId?: string;
    tool: "grab" | "pointer";
    refreshKey?: number;
    onSelectNode?: (id: string) => void;
    onRequestRecenterRef?: (fn: () => void) => void;
    onRequestZoomRef?: (api: { zoomIn: () => void; zoomOut: () => void }) => void;
}) {
    const navigate = useNavigate();
    const { zoomIn, zoomOut, fitView } = useReactFlow();
    const [level, setLevel] = useState<Level | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [nodes, setNodes] = useState<ArchNode[]>([]);
    const [fitToken, setFitToken] = useState(0);
    const nodesRef = useRef(nodes);
    nodesRef.current = nodes;

    useEffect(() => {
        onRequestZoomRef?.({
            zoomIn: () => {
                void zoomIn();
            },
            zoomOut: () => {
                void zoomOut();
            },
        });
        onRequestRecenterRef?.(() => {
            void fitView();
        });
    }, [fitView, onRequestRecenterRef, onRequestZoomRef, zoomIn, zoomOut]);

    useEffect(() => {
        if (fitToken === 0) return;
        void fitView({ padding: 0.2 });
    }, [fitToken, fitView]);

    useEffect(() => {
        setNodes([]);
    }, [nodeId]);

    const parentId = level?.node?.id ?? "";
    const childIds = (level?.children ?? []).map((child) => child.id).join("\0");
    const edgeKey = (level?.relationships ?? []).map((rel) => `${rel.from}\0${rel.to}`).join("\n");

    useEffect(() => {
        if (!level) return;
        if ((nodeId ?? "") !== parentId) return;
        let cancelled = false;
        const key = layoutCacheKey(nodeId);
        const ids = (level.children ?? []).map((child) => child.id);
        const edges = (level.relationships ?? []).map((rel) => ({ from: rel.from, to: rel.to }));
        getLayout(key)
            .then(async (cached) => {
                if (cancelled) return;
                if (cached.positions.length > 0) {
                    const stored: Record<string, Point> = {};
                    for (const position of cached.positions) {
                        stored[position.id] = { x: position.x, y: position.y };
                    }
                    setNodes(buildArchNodes(level, stored));
                    setFitToken((value) => value + 1);
                    return;
                }
                const laid = await layoutLevel(ids, edges);
                if (cancelled) return;
                setNodes(buildArchNodes(level, laid));
                setFitToken((value) => value + 1);
                await putLayout(key, {
                    positions: ids.map((id) => ({ id, x: laid[id].x, y: laid[id].y })),
                });
            })
            .catch((err: unknown) => {
                if (cancelled) return;
                setError(err instanceof Error ? err.message : "layout failed");
            });
        return () => {
            cancelled = true;
        };
        // parentId, childIds, and edgeKey are the structural signature. A field
        // edit changes none of them, so this effect does not rerun elk.
        // eslint-disable-next-line react-hooks/exhaustive-deps -- level is read through those keys
    }, [nodeId, parentId, childIds, edgeKey]);

    useEffect(() => {
        const ac = new AbortController();
        setError(null);
        getLevel(nodeId, ac.signal)
            .then((body) => {
                setLevel(body);
            })
            .catch((err: unknown) => {
                if (err instanceof DOMException && err.name === "AbortError") return;
                setLevel(null);
                setError(err instanceof Error ? err.message : "level request failed");
            });
        return () => ac.abort();
    }, [nodeId, refreshKey]);

    useEffect(() => {
        if (!level) return;
        setNodes((current) => {
            if (current.length === 0) return current;
            const children = new Map((level.children ?? []).map((child) => [child.id, child]));
            let changed = false;
            const next = current.flatMap((node) => {
                const child = children.get(node.id);
                if (!child) {
                    changed = true;
                    return [];
                }
                const data = nodeData(child, level.crossings);
                if (sameData(node.data, data)) return [node];
                changed = true;
                return [{ ...node, data }];
            });
            return changed ? next : current;
        });
    }, [level]);

    const edges = useMemo<Edge[]>(() => {
        return (level?.relationships ?? []).map((rel) => ({
            id: `${rel.from}->${rel.to}`,
            source: rel.from,
            target: rel.to,
            label: rel.label,
            markerEnd: { type: MarkerType.ArrowClosed },
        }));
    }, [level]);

    const onNodeDoubleClick: NodeMouseHandler<ArchNode> = (_event, node) => {
        navigate(`/node/${node.id}`);
    };
    const onNodeClick: NodeMouseHandler<ArchNode> = (_event, node) => {
        onSelectNode?.(node.id);
    };

    const onNodesChange = useCallback((changes: NodeChange<ArchNode>[]) => {
        setNodes((current) => applyNodeChanges(changes, current));
    }, []);

    const onNodeDragStop = (_event: unknown, node: ArchNode) => {
        const ids = (level?.children ?? []).map((child) => child.id);
        const placed = new Map(nodesRef.current.map((item) => [item.id, item.position]));
        placed.set(node.id, node.position);
        void putLayout(layoutCacheKey(nodeId), {
            positions: ids.map((id) => {
                const point = placed.get(id) ?? { x: 0, y: 0 };
                return { id, x: point.x, y: point.y };
            }),
        }).catch((err: unknown) => {
            setError(err instanceof Error ? err.message : "layout save failed");
        });
    };

    return (
        <div className="h-screen w-full">
            {error ? (
                <p className="pointer-events-none absolute top-16 left-1/2 z-50 max-w-md -translate-x-1/2 rounded-md border border-red-600/30 bg-background px-3 py-1.5 text-sm text-red-600">
                    {error}
                </p>
            ) : null}
            <ReactFlow
                nodes={nodes}
                edges={edges}
                nodeTypes={nodeTypes}
                onNodesChange={onNodesChange}
                onNodeClick={onNodeClick}
                onNodeDoubleClick={onNodeDoubleClick}
                onNodeDragStop={onNodeDragStop}
                panOnDrag={tool === "grab"}
                selectionOnDrag={tool === "pointer"}
                nodesConnectable={false}
                proOptions={{ hideAttribution: true }}
            >
                <Background />
            </ReactFlow>
        </div>
    );
}
