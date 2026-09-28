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
import {
    getLayout,
    getLevel,
    getRelationshipLevel,
    layoutCacheKey,
    putLayout,
    type Level,
    type Relationship,
    type RelationshipLevel,
} from "../api";
import { layoutLevel, type Point } from "../layout";
import { NodeCard, type ArchNode, type ArchNodeData, type CrossingMark } from "./NodeCard";
import { ArchEdge } from "./ArchEdge";

const nodeTypes = { arch: NodeCard };
const edgeTypes = { arch: ArchEdge };

function crossingMarks(childId: string, crossings: Level["crossings"]): CrossingMark[] {
    return (crossings ?? [])
        .filter((crossing) => crossing.node_id === childId)
        .map((crossing) => ({
            direction: crossing.direction,
            label: crossing.label,
            otherId: crossing.other_id,
            relationshipId: crossing.relationship_id,
            drillable: crossing.drillable,
        }));
}

function nodeData(child: SummaryLike, crossings: Level["crossings"]): ArchNodeData {
    return {
        name: child.name,
        type: child.type,
        status: child.status,
        protected: child.protected,
        crossings: crossingMarks(child.id, crossings),
    };
}

type SummaryLike = { id: string; name: string; type: string; status: string; protected: boolean };

function buildArchNodes(
    level: Level | RelationshipLevel,
    positions: Record<string, Point>,
): ArchNode[] {
    const children = level.children ?? [];
    return children.map((child, index) => ({
        id: child.id,
        type: "arch" as const,
        selectable: true,
        draggable: true,
        position: positions[child.id] ?? {
            x: (index % 4) * 280,
            y: Math.floor(index / 4) * 160,
        },
        data: nodeData(child, level.crossings ?? []),
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
    relationshipId,
    tool,
    refreshKey = 0,
    onSelectNode,
    onSelectEdge,
    onRequestRecenterRef,
    onRequestZoomRef,
}: {
    nodeId?: string;
    relationshipId?: string;
    tool: "grab" | "pointer";
    refreshKey?: number;
    onSelectNode?: (id: string) => void;
    onSelectEdge?: (edge: Relationship | null) => void;
    onRequestRecenterRef?: (fn: () => void) => void;
    onRequestZoomRef?: (api: { zoomIn: () => void; zoomOut: () => void }) => void;
}) {
    const navigate = useNavigate();
    const { zoomIn, zoomOut, fitView } = useReactFlow();
    const [level, setLevel] = useState<Level | null>(null);
    const [relLevel, setRelLevel] = useState<RelationshipLevel | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [notFound, setNotFound] = useState(false);
    const [nodes, setNodes] = useState<ArchNode[]>([]);
    const [fitToken, setFitToken] = useState(0);
    const nodesRef = useRef(nodes);
    nodesRef.current = nodes;

    useEffect(() => {
        onRequestZoomRef?.({
            zoomIn: () => void zoomIn(),
            zoomOut: () => void zoomOut(),
        });
        onRequestRecenterRef?.(() => void fitView());
    }, [fitView, onRequestRecenterRef, onRequestZoomRef, zoomIn, zoomOut]);

    useEffect(() => {
        if (fitToken === 0) return;
        void fitView({ padding: 0.2 });
    }, [fitToken, fitView]);

    useEffect(() => {
        setNodes([]);
        setLevel(null);
        setRelLevel(null);
    }, [nodeId, relationshipId]);

    const activeLevel = relLevel ?? level;
    const parentId = level?.node?.id ?? "";
    const childIds = (activeLevel?.children ?? []).map((child) => child.id).join("\0");
    const edgeKey = (activeLevel?.relationships ?? []).map((rel) => rel.id).join("\n");
    const cacheKey = layoutCacheKey({ nodeId, relationshipId });

    useEffect(() => {
        if (!activeLevel) return;
        if (relationshipId && !relLevel) return;
        if (!relationshipId && (nodeId ?? "") !== parentId && nodeId !== undefined) return;
        let cancelled = false;
        const ids = (activeLevel.children ?? []).map((c) => c.id);
        const edges = (activeLevel.relationships ?? []).map((rel) => ({ from: rel.from, to: rel.to }));
        getLayout(cacheKey)
            .then(async (cached) => {
                if (cancelled) return;
                const stored: Record<string, Point> = {};
                for (const position of cached.positions) {
                    stored[position.id] = { x: position.x, y: position.y };
                }
                if (cached.positions.length > 0) {
                    setNodes(buildArchNodes(activeLevel, stored));
                    setFitToken((v) => v + 1);
                    return;
                }
                const laid = await layoutLevel(ids, edges);
                if (cancelled) return;
                setNodes(buildArchNodes(activeLevel, laid));
                setFitToken((v) => v + 1);
                await putLayout(cacheKey, {
                    positions: ids.map((id) => ({ id, x: laid[id]?.x ?? 0, y: laid[id]?.y ?? 0 })),
                    ports: cached.ports,
                    attachments: cached.attachments,
                });
            })
            .catch((err: unknown) => {
                if (cancelled) return;
                setError(err instanceof Error ? err.message : "layout failed");
            });
        return () => {
            cancelled = true;
        };
    }, [nodeId, relationshipId, parentId, childIds, edgeKey, cacheKey, activeLevel, relLevel]);

    useEffect(() => {
        const ac = new AbortController();
        setError(null);
        setNotFound(false);
        if (relationshipId) {
            getRelationshipLevel(relationshipId, ac.signal)
                .then((body) => setRelLevel(body))
                .catch((err: unknown) => {
                    if (err instanceof DOMException && err.name === "AbortError") return;
                    setRelLevel(null);
                    setNotFound(true);
                    setError(err instanceof Error ? err.message : "level request failed");
                });
        } else {
            getLevel(nodeId, ac.signal)
                .then((body) => setLevel(body))
                .catch((err: unknown) => {
                    if (err instanceof DOMException && err.name === "AbortError") return;
                    setLevel(null);
                    setError(err instanceof Error ? err.message : "level request failed");
                });
        }
        return () => ac.abort();
    }, [nodeId, relationshipId, refreshKey]);

    useEffect(() => {
        if (!activeLevel) return;
        setNodes((current) => {
            if (current.length === 0) return current;
            const children = new Map((activeLevel.children ?? []).map((child) => [child.id, child]));
            let changed = false;
            const next = current.flatMap((node) => {
                const child = children.get(node.id);
                if (!child) {
                    changed = true;
                    return [];
                }
                const data = nodeData(child, activeLevel.crossings ?? []);
                if (sameData(node.data, data)) return [node];
                changed = true;
                return [{ ...node, data }];
            });
            return changed ? next : current;
        });
    }, [activeLevel]);

    const edges = useMemo<Edge[]>(() => {
        const rels = activeLevel?.relationships ?? [];
        const pairCount = new Map<string, number>();
        for (const rel of rels) {
            const key = `${rel.from}\0${rel.to}`;
            pairCount.set(key, (pairCount.get(key) ?? 0) + 1);
        }
        const pairIndex = new Map<string, number>();
        return rels.map((rel) => {
            const key = `${rel.from}\0${rel.to}`;
            const labelIndex = pairIndex.get(key) ?? 0;
            pairIndex.set(key, labelIndex + 1);
            const labelCount = pairCount.get(key) ?? 1;
            return {
                id: rel.id,
                type: "arch",
                source: rel.from,
                target: rel.to,
                markerEnd: { type: MarkerType.ArrowClosed },
                style: { cursor: rel.drillable ? "pointer" : "default" },
                data: { drillable: rel.drillable, rel, fromNodeId: nodeId, labelIndex, labelCount },
            };
        });
    }, [activeLevel, nodeId]);

    const onNodeDoubleClick: NodeMouseHandler<ArchNode> = (_event, node) => {
        navigate(`/node/${node.id}`);
    };
    const onNodeClick: NodeMouseHandler<ArchNode> = (_event, node) => {
        onSelectEdge?.(null);
        onSelectNode?.(node.id);
    };

    const onEdgeClick = useCallback(
        (_event: unknown, edge: Edge) => {
            const rel = edge.data?.rel as Relationship | undefined;
            if (rel) onSelectEdge?.(rel);
        },
        [onSelectEdge],
    );

    const onEdgeDoubleClick = useCallback(
        (_event: unknown, edge: Edge) => {
            const rel = edge.data?.rel as Relationship | undefined;
            if (rel?.drillable) navigate(`/relationship/${rel.id}`, { state: { fromNodeId: nodeId } });
        },
        [navigate, nodeId],
    );

    const onNodesChange = useCallback((changes: NodeChange<ArchNode>[]) => {
        setNodes((current) => applyNodeChanges(changes, current));
    }, []);

    const onNodeDragStop = (_event: unknown, node: ArchNode) => {
        const ids = (activeLevel?.children ?? []).map((child) => child.id);
        const placed = new Map(nodesRef.current.map((item) => [item.id, item.position]));
        placed.set(node.id, node.position);
        void putLayout(cacheKey, {
            positions: ids.map((id) => {
                const point = placed.get(id) ?? { x: 0, y: 0 };
                return { id, x: point.x, y: point.y };
            }),
        }).catch((err: unknown) => {
            setError(err instanceof Error ? err.message : "layout save failed");
        });
    };

    if (notFound && relationshipId) {
        return (
            <div className="flex h-full items-center justify-center text-sm text-foreground/70">
                This relationship has no interior to open.
            </div>
        );
    }

    return (
        <div className="h-full w-full">
            {error ? (
                <p className="pointer-events-none absolute top-16 left-1/2 z-50 max-w-md -translate-x-1/2 rounded-md border border-red-600/30 bg-background px-3 py-1.5 text-sm text-red-600">
                    {error}
                </p>
            ) : null}
            <ReactFlow
                nodes={nodes}
                edges={edges}
                nodeTypes={nodeTypes}
                edgeTypes={edgeTypes}
                onNodesChange={onNodesChange}
                onNodeClick={onNodeClick}
                onNodeDoubleClick={onNodeDoubleClick}
                onEdgeClick={onEdgeClick}
                onEdgeDoubleClick={onEdgeDoubleClick}
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
