import { useEffect, useMemo, useState } from "react";
import {
    Background,
    MarkerType,
    ReactFlow,
    useReactFlow,
    type Edge,
    type NodeMouseHandler,
} from "@xyflow/react";
import { useNavigate } from "react-router";
import "@xyflow/react/dist/style.css";
import { NodeCard, type ArchNode, type CrossingMark } from "./NodeCard";

type Summary = {
    id: string;
    name: string;
    type: string;
    status: string;
    protected: boolean;
};

type LevelResponse = {
    node: Summary | null;
    children: Summary[];
    relationships: { from: string; to: string; label: string; kind: string }[];
    crossings: {
        node_id: string;
        direction: string;
        label: string;
        kind: string;
        other_id: string;
    }[];
    warnings: { severity: string; path: string; message: string }[];
};

const nodeTypes = { arch: NodeCard };

export default function FlowCanvas({
    nodeId,
    tool,
    onRequestRecenterRef,
    onRequestZoomRef,
}: {
    nodeId?: string;
    tool: "grab" | "pointer";
    onRequestRecenterRef?: (fn: () => void) => void;
    onRequestZoomRef?: (api: { zoomIn: () => void; zoomOut: () => void }) => void;
}) {
    const navigate = useNavigate();
    const { zoomIn, zoomOut, fitView } = useReactFlow();
    const [level, setLevel] = useState<LevelResponse | null>(null);
    const [error, setError] = useState<string | null>(null);

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
        const path = nodeId ? `/levels/${encodeURIComponent(nodeId)}` : "/levels";
        const ac = new AbortController();
        setError(null);
        fetch(path, { signal: ac.signal })
            .then(async (res) => {
                if (!res.ok) {
                    const body = (await res.json().catch(() => null)) as { error?: string } | null;
                    throw new Error(body?.error ?? `level request failed (${res.status})`);
                }
                return res.json() as Promise<LevelResponse>;
            })
            .then((body) => {
                setLevel(body);
            })
            .catch((err: unknown) => {
                if (err instanceof DOMException && err.name === "AbortError") return;
                setLevel(null);
                setError(err instanceof Error ? err.message : "level request failed");
            });
        return () => ac.abort();
    }, [nodeId]);

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
            return {
                id: child.id,
                type: "arch",
                position: { x: (index % 4) * 280, y: Math.floor(index / 4) * 160 },
                data: {
                    name: child.name,
                    type: child.type,
                    status: child.status,
                    protected: child.protected,
                    crossings: marks,
                },
            };
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

    return (
        <div className="h-screen w-full">
            <div className="pointer-events-none absolute left-4 top-4 z-40 text-sm text-foreground">
                {level?.node ? level.node.name : "Model"}
            </div>
            {error ? (
                <p className="absolute left-4 top-12 z-40 text-sm text-red-600">{error}</p>
            ) : null}
            <ReactFlow
                nodes={nodes}
                edges={edges}
                nodeTypes={nodeTypes}
                onNodeDoubleClick={onNodeDoubleClick}
                fitView
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
