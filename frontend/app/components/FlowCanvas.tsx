import { useEffect, useMemo, useRef, useState } from "react";
import {
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
import { NodeCard, type ArchNode, type CrossingMark } from "./NodeCard";
import NodeInspector from "./node-inspector";

const nodeTypes = { arch: NodeCard };

export default function FlowCanvas({
    nodeId,
    tool,
    refreshKey = 0,
    onRequestRecenterRef,
    onRequestZoomRef,
}: {
    nodeId?: string;
    tool: "grab" | "pointer";
    refreshKey?: number;
    onRequestRecenterRef?: (fn: () => void) => void;
    onRequestZoomRef?: (api: { zoomIn: () => void; zoomOut: () => void }) => void;
}) {
    const navigate = useNavigate();
    const { zoomIn, zoomOut, fitView } = useReactFlow();
    const [level, setLevel] = useState<Level | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [selectedId, setSelectedId] = useState<string | null>(null);
    const [reloadKey, setReloadKey] = useState(0);
    const [positions, setPositions] = useState<Record<string, Point> | null>(null);
    const [fitToken, setFitToken] = useState(0);
    const positionsRef = useRef<Record<string, Point> | null>(null);
    positionsRef.current = positions;

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
        setPositions(null);
        getLayout(key)
            .then(async (cached) => {
                if (cancelled) return;
                if (cached.positions.length > 0) {
                    const stored: Record<string, Point> = {};
                    for (const position of cached.positions) {
                        stored[position.id] = { x: position.x, y: position.y };
                    }
                    setPositions(stored);
                    setFitToken((value) => value + 1);
                    return;
                }
                const laid = await layoutLevel(ids, edges);
                if (cancelled) return;
                setPositions(laid);
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
    }, [nodeId, reloadKey, refreshKey]);

    const nodes = useMemo<ArchNode[]>(() => {
        const children = level?.children ?? [];
        const crossings = level?.crossings ?? [];
        return children.map((child, index) => {
            const marks: CrossingMark[] = crossings
                .filter((crossing) => crossing.node_id === child.id)
                .map((crossing) => ({
                    direction: crossing.direction,
                    label: crossing.label,
                    otherId: crossing.other_id,
                }));
            const point = positions?.[child.id];
            return {
                id: child.id,
                type: "arch",
                position: point ?? { x: (index % 4) * 280, y: Math.floor(index / 4) * 160 },
                data: {
                    name: child.name,
                    type: child.type,
                    status: child.status,
                    protected: child.protected,
                    crossings: marks,
                },
            };
        });
    }, [level, positions]);

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
        setSelectedId(node.id);
    };

    const onNodesChange = (changes: NodeChange<ArchNode>[]) => {
        setPositions((prev) => {
            if (!prev) return prev;
            let next = prev;
            for (const change of changes) {
                if (change.type === "position" && change.position) {
                    if (next === prev) next = { ...prev };
                    next[change.id] = change.position;
                }
            }
            positionsRef.current = next;
            return next;
        });
    };

    const onNodeDragStop = (_event: unknown, node: ArchNode) => {
        const current = { ...(positionsRef.current ?? {}), [node.id]: node.position };
        positionsRef.current = current;
        setPositions(current);
        const ids = (level?.children ?? []).map((child) => child.id);
        void putLayout(layoutCacheKey(nodeId), {
            positions: ids.map((id) => {
                const point = id === node.id ? node.position : (current[id] ?? { x: 0, y: 0 });
                return { id, x: point.x, y: point.y };
            }),
        });
    };

    return (
        <div className="h-screen w-full">
            <div className="pointer-events-none absolute left-4 top-4 z-40 text-sm text-foreground">
                {level?.node ? level.node.name : "Model"}
            </div>
            {error ? (
                <p className="absolute left-4 top-12 z-40 text-sm text-red-600">{error}</p>
            ) : null}
            <ReactFlow
                nodes={positions ? nodes : []}
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
            {selectedId ? (
                <NodeInspector
                    nodeId={selectedId}
                    onClose={() => setSelectedId(null)}
                    onSaved={() => setReloadKey((value) => value + 1)}
                />
            ) : null}
        </div>
    );
}
