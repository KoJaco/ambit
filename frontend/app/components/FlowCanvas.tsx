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
    type OnNodeDrag,
} from "@xyflow/react";
import { useNavigate } from "react-router";
import "@xyflow/react/dist/style.css";
import {
    getLayout,
    getLevel,
    getRelationshipLevel,
    layoutCacheKey,
    loadGroupSizes,
    loadLayoutSig,
    loadViewModes,
    saveGroupSizes,
    saveLayoutSig,
    putLayout,
    relationshipDrillId,
    saveViewModes,
    stageGroup,
    stageReparent,
    stageUngroup,
    type Level,
    type Relationship,
    type RelationshipLevel,
    type SchemaColumn,
} from "../api";
import {
    containerAt,
    drillsOnDoubleClick,
    outsideReparentTarget,
    defaultGroupName,
    groupableSelection,
    groupButtonLabel,
    stageErrorMessage,
    stagedMessage,
    type Rect,
} from "../canvas-structure";
import { StructurePanel, type StructureRequest } from "./structure-panel";
import { canvasEdgeType, canvasNodeType, isStackedCard, nodeLayoutSize } from "../canvas-presentation";
import { computeGroupSizes, layoutCompound, layoutLevel, type NodeSize, type Point } from "../layout";
import {
    canExpandChild,
    expandedGroups,
    groupTints,
    layoutSignature,
    layoutUsable,
    fallbackViewDefaults,
    maxLevelFetches,
    overBudgetHint,
    resolveExpanded,
    toggleMode,
    type ViewModes,
} from "../level-view";
import { NodeCard, type ArchNode, type ArchNodeData, type CrossingMark } from "./NodeCard";
import { ArchEdge } from "./ArchEdge";
import { AnimatedEdge } from "./flow/animated-edge";
import { ButtonEdge } from "./flow/button-edge";
import { DataEdge } from "./flow/data-edge";
import { ContainerNode, GroupFrame } from "./flow/group-node";
import { PlaceholderFrame } from "./flow/placeholder-node";
import { SchemaNode } from "./flow/schema-node";

const nodeTypes = {
    card: NodeCard,
    schema: SchemaNode,
    placeholder: PlaceholderFrame,
    group: GroupFrame,
    container: ContainerNode,
};
const edgeTypes = {
    arch: ArchEdge,
    button: ButtonEdge,
    animated: AnimatedEdge,
    data: DataEdge,
};

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

function nodeData(
    child: SummaryLike,
    crossings: Level["crossings"],
    onToggleMode?: (id: string) => void,
    onUngroup?: (id: string) => void,
): ArchNodeData {
    return {
        name: child.name,
        type: child.type,
        status: child.status,
        protected: child.protected,
        crossings: crossingMarks(child.id, crossings),
        icon: child.icon,
        color: child.color,
        renderer: child.renderer,
        schema: child.schema,
        childCount: child.child_count,
        typeMix: child.type_mix,
        internalRelCount: child.internal_rel_count,
        expanded: child.expanded,
        onToggleMode,
        onUngroup: (child.child_count ?? 0) > 0 ? onUngroup : undefined,
    };
}

// The expand control is withheld where the server would ignore the request.
function toggleFor(
    child: SummaryLike,
    level: Level | RelationshipLevel,
    onToggleMode?: (id: string) => void,
): ((id: string) => void) | undefined {
    return canExpandChild(child, level.children ?? [], "relationship" in level) ? onToggleMode : undefined;
}

type SummaryLike = {
    id: string;
    name: string;
    type: string;
    status: string;
    protected: boolean;
    icon?: string;
    color?: string;
    renderer?: string;
    schema?: SchemaColumn[];
    child_count?: number;
    group_id?: string;
    expanded?: boolean;
    internal_rel_count?: number;
    type_mix?: { type: string; count: number }[];
};

// Footprint per child; collapsed groups are drawn as stacked cards and reserve the sheet offset.
function layoutSizes(kids: SummaryLike[]): Record<string, NodeSize> {
    const sizes: Record<string, NodeSize> = {};
    for (const child of kids) {
        sizes[child.id] = nodeLayoutSize(
            child.renderer,
            child.schema?.length ?? 0,
            isStackedCard(child.renderer, child.child_count, child.expanded),
        );
    }
    return sizes;
}

// Container sizes: `mins` wrap the members; `applied` also honours sizes the architect set.
function containerSizing(
    compound: { id: string; parentId?: string }[],
    positions: Record<string, Point>,
    sizes: Record<string, NodeSize>,
    cacheKey: string,
) {
    return {
        mins: computeGroupSizes(compound, positions, sizes),
        applied: computeGroupSizes(compound, positions, sizes, loadGroupSizes(cacheKey)),
    };
}

// Left padding for fit-view so the sidebar overlay does not cover the first column.
function fitPadding() {
    const left = Math.round(Math.min(290, window.innerWidth * 0.3));
    return { top: 0.2, right: 0.2, bottom: 0.2, left: `${left}px` as const };
}

function buildArchNodes(
    level: Level | RelationshipLevel,
    positions: Record<string, Point>,
    containers: { mins: Record<string, NodeSize>; applied: Record<string, NodeSize> },
    onToggleMode?: (id: string) => void,
    onUngroup?: (id: string) => void,
    onResizeGroup?: (id: string, size: NodeSize) => void,
): ArchNode[] {
    const groupSizes = containers.applied;
    const tints = groupTints(level.children ?? []);
    const children = level.children ?? [];
    const present = new Set(children.map((child) => child.id));
    // Members must follow their group in the array; the server already orders them that way.
    return children.map((child, index) => {
        const parentId = child.group_id && present.has(child.group_id) ? child.group_id : undefined;
        const type = canvasNodeType(child.renderer, child.child_count, child.expanded);
        const size = groupSizes[child.id];
        return {
            id: child.id,
            type,
            selectable: true,
            draggable: true,
            ...(parentId ? { parentId, extent: "parent" as const } : {}),
            ...(type === "container" && size ? { style: { width: size.width, height: size.height } } : {}),
            position: positions[child.id] ?? {
                x: (index % 4) * 280,
                y: Math.floor(index / 4) * 160,
            },
            data: {
                ...nodeData(child, level.crossings ?? [], toggleFor(child, level, onToggleMode), onUngroup),
                ...(type === "container"
                    ? {
                          depth: tints[child.id],
                          minWidth: containers.mins[child.id]?.width,
                          minHeight: containers.mins[child.id]?.height,
                          onResizeGroup,
                      }
                    : {}),
            },
        };
    });
}

function sameSet(left: string[], right: string[]): boolean {
    return left.length === right.length && left.every((id, index) => id === right[index]);
}

function pointerPoint(event: MouseEvent | TouchEvent): { x: number; y: number } | null {
    if ("changedTouches" in event) {
        const touch = event.changedTouches[0];
        return touch ? { x: touch.clientX, y: touch.clientY } : null;
    }
    return { x: event.clientX, y: event.clientY };
}

function sameData(left: ArchNodeData, right: ArchNodeData): boolean {
    if (
        left.name !== right.name ||
        left.type !== right.type ||
        left.status !== right.status ||
        left.protected !== right.protected ||
        left.icon !== right.icon ||
        left.color !== right.color ||
        left.renderer !== right.renderer ||
        left.childCount !== right.childCount ||
        left.expanded !== right.expanded ||
        left.depth !== right.depth ||
        left.internalRelCount !== right.internalRelCount ||
        (left.typeMix ?? []).map((entry) => `${entry.type}:${entry.count}`).join() !==
            (right.typeMix ?? []).map((entry) => `${entry.type}:${entry.count}`).join() ||
        left.crossings.length !== right.crossings.length ||
        (left.schema?.length ?? 0) !== (right.schema?.length ?? 0)
    ) {
        return false;
    }
    if (
        (left.schema ?? []).some(
            (row, index) =>
                row.title !== right.schema?.[index]?.title || row.type !== right.schema?.[index]?.type,
        )
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

// Fetch a level, then refine the expand set by the 0010 defaults. The client only learns about
// depth-2 candidates from a response, so this is bounded to maxLevelFetches requests.
async function loadLevel(nodeId: string | undefined, modes: ViewModes, signal: AbortSignal): Promise<Level> {
    let expand = Object.keys(modes)
        .filter((id) => modes[id] === "group")
        .sort();
    let body = await getLevel(nodeId, signal, expand);
    for (let fetches = 1; fetches < maxLevelFetches; fetches += 1) {
        const next = resolveExpanded(body.children ?? [], modes, body.view_defaults ?? fallbackViewDefaults);
        if (sameSet(next, expand)) break;
        expand = next;
        body = await getLevel(nodeId, signal, expand);
    }
    return body;
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
    onProposalStaged,
}: {
    nodeId?: string;
    relationshipId?: string;
    tool: "grab" | "pointer";
    refreshKey?: number;
    onSelectNode?: (id: string) => void;
    onSelectEdge?: (edge: Relationship | null) => void;
    onRequestRecenterRef?: (fn: () => void) => void;
    onRequestZoomRef?: (api: { zoomIn: () => void; zoomOut: () => void }) => void;
    // Called after a structure edit staged a proposal; the host brings the review panel forward.
    onProposalStaged?: () => void;
}) {
    const navigate = useNavigate();
    const { zoomIn, zoomOut, fitView, screenToFlowPosition, getInternalNode } = useReactFlow();
    const [level, setLevel] = useState<Level | null>(null);
    const [relLevel, setRelLevel] = useState<RelationshipLevel | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [notFound, setNotFound] = useState(false);
    const [nodes, setNodes] = useState<ArchNode[]>([]);
    const [fitToken, setFitToken] = useState(0);
    const [notice, setNotice] = useState<{ kind: "info" | "staged" | "error"; text: string } | null>(null);
    const [request, setRequest] = useState<StructureRequest | null>(null);
    const [staging, setStaging] = useState(false);
    const groupButtonRef = useRef<HTMLButtonElement | null>(null);
    const openerRef = useRef<HTMLElement | null>(null);
    const nodesRef = useRef(nodes);
    nodesRef.current = nodes;
    const dragStart = useRef<{ id: string; position: Point } | null>(null);

    // View modes are per level (base key), kept out of the model: see layout-cache notes in
    // docs/contracts/local-http-api.md. The layout key additionally carries the expanded-set hash.
    const baseKey = layoutCacheKey({ nodeId, relationshipId });
    const storedModes = useMemo(() => loadViewModes(baseKey), [baseKey]);
    const [modeOverride, setModeOverride] = useState<{ key: string; modes: ViewModes } | null>(null);
    const viewModes = modeOverride?.key === baseKey ? modeOverride.modes : storedModes;
    const viewModesRef = useRef(viewModes);
    viewModesRef.current = viewModes;

    useEffect(() => {
        onRequestZoomRef?.({
            zoomIn: () => void zoomIn(),
            zoomOut: () => void zoomOut(),
        });
        onRequestRecenterRef?.(() => void fitView({ padding: fitPadding() }));
    }, [fitView, onRequestRecenterRef, onRequestZoomRef, zoomIn, zoomOut]);

    useEffect(() => {
        if (fitToken === 0) return;
        void fitView({ padding: fitPadding() });
    }, [fitToken, fitView]);

    useEffect(() => {
        setNodes([]);
        setLevel(null);
        setRelLevel(null);
        setNotice(null);
    }, [nodeId, relationshipId]);

    useEffect(() => {
        if (!notice) return;
        const timer = window.setTimeout(() => setNotice(null), notice.kind === "staged" ? 12000 : 8000);
        return () => window.clearTimeout(timer);
    }, [notice]);

    const levelRef = useRef<Level | null>(null);
    levelRef.current = level;
    const onToggleMode = useCallback(
        (id: string) => {
            const next = toggleMode(viewModesRef.current, levelRef.current?.children ?? [], id);
            saveViewModes(baseKey, next);
            setModeOverride({ key: baseKey, modes: next });
        },
        [baseKey],
    );

    // Resizing a group only records its size here (localStorage, next to the view modes); the typed
    // server layout carries positions only, and the top-left corner never moves.
    const cacheKeyRef = useRef("");
    const onResizeGroup = useCallback((id: string, size: NodeSize) => {
        const key = cacheKeyRef.current;
        if (!key) return;
        saveGroupSizes(key, {
            ...loadGroupSizes(key),
            [id]: { width: Math.round(size.width), height: Math.round(size.height) },
        });
    }, []);

    // Structure edits only ever stage a proposal; the model changes when the architect accepts it,
    // and the model-changed event then reloads the level (refreshKey).
    const stage = useCallback(
        async (run: () => Promise<unknown>): Promise<boolean> => {
            setStaging(true);
            try {
                await run();
                setNotice({ kind: "staged", text: stagedMessage() });
                onProposalStaged?.();
                return true;
            } catch (err: unknown) {
                setNotice({ kind: "error", text: stageErrorMessage(err) });
                return false;
            } finally {
                setStaging(false);
            }
        },
        [onProposalStaged],
    );

    // newParentId is "" for the root.
    const requestReparent = useCallback(
        (movedId: string, newParentId: string) => {
            void stage(() => stageReparent(movedId, newParentId));
        },
        [stage],
    );

    const onUngroup = useCallback((id: string) => {
        openerRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
        const child = levelRef.current?.children.find((item) => item.id === id);
        setRequest({ kind: "ungroup", id, name: child?.name ?? id });
    }, []);

    const closeRequest = useCallback(() => {
        setRequest(null);
        const opener = openerRef.current;
        openerRef.current = null;
        window.setTimeout(() => {
            if (opener && opener.isConnected) opener.focus();
        }, 0);
    }, []);

    const confirmRequest = useCallback(
        async (value: { name: string; deleteContainer: boolean }) => {
            if (!request) return;
            const ok = await stage(() =>
                request.kind === "group"
                    ? stageGroup({ node_ids: request.ids, name: value.name })
                    : stageUngroup(request.id, value.deleteContainer),
            );
            if (ok) closeRequest();
        },
        [request, stage, closeRequest],
    );

    const activeLevel = relLevel ?? level;
    const parentId = level?.node?.id ?? "";
    const childIds = (activeLevel?.children ?? [])
        .map((child) => `${child.id}>${child.group_id ?? ""}`)
        .join("\0");
    const edgeKey = (activeLevel?.relationships ?? []).map((rel) => rel.id).join("\n");
    const expandedIds = useMemo(
        () => (activeLevel?.children ?? []).filter((child) => child.expanded).map((child) => child.id),
        [activeLevel],
    );
    const expandedSig = expandedIds.join("\0");
    const cacheKey = layoutCacheKey({ nodeId, relationshipId, expanded: expandedIds });
    cacheKeyRef.current = cacheKey;

    useEffect(() => {
        if (!activeLevel) return;
        if (relationshipId && !relLevel) return;
        if (!relationshipId && (nodeId ?? "") !== parentId && nodeId !== undefined) return;
        let cancelled = false;
        const kids = activeLevel.children ?? [];
        const ids = kids.map((c) => c.id);
        const compound = kids.map((c) => ({ id: c.id, parentId: c.group_id }));
        const sizes = layoutSizes(kids);
        const edges = (activeLevel.relationships ?? []).map((rel) => ({ from: rel.from, to: rel.to }));
        const nested = kids.some((c) => c.group_id);
        getLayout(cacheKey)
            .then(async (cached) => {
                if (cancelled) return;
                const stored: Record<string, Point> = {};
                for (const position of cached.positions) {
                    stored[position.id] = { x: position.x, y: position.y };
                }
                // Hashed variants are not invalidated by create/delete/reparent (only the base key is),
                // so a cache is only trusted when it covers exactly the nodes on screen.
                // The id set alone is not enough: a node moved between two open groups keeps it, so the
                // membership signature (id>group_id) must also match the one saved with the layout.
                const sig = layoutSignature(kids);
                const usable = layoutUsable(
                    ids,
                    cached.positions.map((position) => position.id),
                    loadLayoutSig(cacheKey),
                    sig,
                );
                if (usable) {
                    setNodes(buildArchNodes(activeLevel, stored, containerSizing(compound, stored, sizes, cacheKey), onToggleMode, onUngroup, onResizeGroup));
                    setFitToken((v) => v + 1);
                    return;
                }
                const laid = nested
                    ? (await layoutCompound(compound, edges, sizes)).positions
                    : await layoutLevel(ids, edges, sizes);
                if (cancelled) return;
                setNodes(buildArchNodes(activeLevel, laid, containerSizing(compound, laid, sizes, cacheKey), onToggleMode, onUngroup, onResizeGroup));
                setFitToken((v) => v + 1);
                await putLayout(cacheKey, {
                    positions: ids.map((id) => ({ id, x: laid[id]?.x ?? 0, y: laid[id]?.y ?? 0 })),
                    ports: cached.ports,
                    attachments: cached.attachments,
                });
                saveLayoutSig(cacheKey, sig);
            })
            .catch((err: unknown) => {
                if (cancelled) return;
                setError(err instanceof Error ? err.message : "layout failed");
            });
        return () => {
            cancelled = true;
        };
    }, [nodeId, relationshipId, parentId, childIds, edgeKey, cacheKey, expandedSig, activeLevel, relLevel, onToggleMode, onUngroup, onResizeGroup]);

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
            loadLevel(nodeId, viewModesRef.current, ac.signal)
                .then((body) => setLevel(body))
                .catch((err: unknown) => {
                    if (err instanceof DOMException && err.name === "AbortError") return;
                    setLevel(null);
                    setError(err instanceof Error ? err.message : "level request failed");
                });
        }
        return () => ac.abort();
    }, [nodeId, relationshipId, refreshKey, viewModes]);

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
                const data = nodeData(child, activeLevel.crossings ?? [], toggleFor(child, activeLevel, onToggleMode), onUngroup);
                const type = canvasNodeType(child.renderer, child.child_count, child.expanded);
                if (node.type === type && sameData(node.data, data)) return [node];
                if ((node.type === "container") !== (type === "container")) {
                    // Open/closed changed: the layout effect rebuilds the level with sizes and parents.
                    changed = true;
                    return [];
                }
                changed = true;
                return [{ ...node, type, data: { ...data, depth: node.data.depth, minWidth: node.data.minWidth, minHeight: node.data.minHeight, onResizeGroup: node.data.onResizeGroup } }];
            });
            return changed ? next : current;
        });
    }, [activeLevel, onToggleMode, onUngroup]);

    const edges = useMemo<Edge[]>(() => {
        const rels = activeLevel?.relationships ?? [];
        const pairCount = new Map<string, number>();
        for (const rel of rels) {
            const key = `${rel.from}\0${rel.to}`;
            pairCount.set(key, (pairCount.get(key) ?? 0) + 1);
        }
        const pairIndex = new Map<string, number>();
        const names = new Map((activeLevel?.children ?? []).map((child) => [child.id, child.name]));
        return rels.map((rel) => {
            const kindForType = rel.aggregated ? undefined : rel.kind;
            const key = `${rel.from}\0${rel.to}`;
            const labelIndex = pairIndex.get(key) ?? 0;
            pairIndex.set(key, labelIndex + 1);
            const labelCount = pairCount.get(key) ?? 1;
            return {
                id: rel.id,
                type: canvasEdgeType(kindForType),
                source: rel.from,
                target: rel.to,
                markerEnd: { type: MarkerType.ArrowClosed },
                style: { cursor: relationshipDrillId(rel) ? "pointer" : "default" },
                data: {
                    drillable: rel.drillable,
                    rel,
                    fromNodeId: nodeId,
                    fromName: names.get(rel.from),
                    toName: names.get(rel.to),
                    labelIndex,
                    labelCount,
                },
            };
        });
    }, [activeLevel, nodeId]);

    // Drilling is only for summary cards. An expanded group already shows its children
    // on this level, so opening it would repeat the same picture (decision 0010).
    const onNodeDoubleClick: NodeMouseHandler<ArchNode> = (_event, node) => {
        if (!drillsOnDoubleClick(node.type)) return;
        navigate(`/node/${node.id}`);
    };
    const onNodeClick: NodeMouseHandler<ArchNode> = (_event, node) => {
        onSelectEdge?.(null);
        onSelectNode?.(node.id);
    };

    const onEdgeClick = useCallback(
        (_event: unknown, edge: Edge) => {
            const rel = edge.data?.rel as Relationship | undefined;
            // Aggregated edges have no relationship of their own to inspect; their pill lists the members.
            if (rel && !rel.aggregated) onSelectEdge?.(rel);
        },
        [onSelectEdge],
    );

    const onEdgeDoubleClick = useCallback(
        (_event: unknown, edge: Edge) => {
            const rel = edge.data?.rel as Relationship | undefined;
            const drillId = rel ? relationshipDrillId(rel) : undefined;
            if (drillId) navigate(`/relationship/${drillId}`, { state: { fromNodeId: nodeId } });
        },
        [navigate, nodeId],
    );

    const onNodesChange = useCallback((changes: NodeChange<ArchNode>[]) => {
        setNodes((current) => applyNodeChanges(changes, current));
    }, []);

    const onNodeDragStart: OnNodeDrag<ArchNode> = (_event, node) => {
        dragStart.current = { id: node.id, position: { ...node.position } };
    };

    // Absolute rects of open containers, from React Flow's measured nodes.
    function containerRects(): Rect[] {
        const rects: Rect[] = [];
        for (const item of nodesRef.current) {
            if (item.type !== "container") continue;
            const internal = getInternalNode(item.id);
            if (!internal) continue;
            const origin = internal.internals.positionAbsolute;
            rects.push({
                id: item.id,
                x: origin.x,
                y: origin.y,
                width: internal.measured.width ?? 0,
                height: internal.measured.height ?? 0,
            });
        }
        return rects;
    }

    // The member is clamped to its group (extent: parent), so leaving the group is read from the
    // pointer. Returns the drop target id ("" is the level itself), or null when still inside.
    function dropOutsideTarget(event: MouseEvent | TouchEvent, node: ArchNode): string | null {
        if (!node.parentId) return null;
        const pointer = pointerPoint(event);
        if (!pointer) return null;
        const at = screenToFlowPosition(pointer);
        const rects = containerRects();
        if (containerAt(at, rects.filter((rect) => rect.id === node.parentId)) !== null) return null;
        return outsideReparentTarget(containerAt(at, rects, [node.parentId, node.id]), level?.node?.id);
    }

    // A top-level node dropped on an open container moves into it (via proposal). Null otherwise.
    function dropInTarget(event: MouseEvent | TouchEvent, node: ArchNode): string | null {
        if (node.parentId || node.type === "container") return null;
        const pointer = pointerPoint(event);
        if (!pointer) return null;
        return containerAt(screenToFlowPosition(pointer), containerRects(), [node.id]);
    }

    const onNodeDragStop: OnNodeDrag<ArchNode> = (event, node) => {
        const outside = dropOutsideTarget(event, node) ?? dropInTarget(event, node);
        if (outside !== null) {
            const start = dragStart.current?.id === node.id ? dragStart.current.position : null;
            if (start) {
                setNodes((current) =>
                    current.map((item) => (item.id === node.id ? { ...item, position: start } : item)),
                );
            }
            requestReparent(node.id, outside);
            return;
        }
        const ids = (activeLevel?.children ?? []).map((child) => child.id);
        // Members are stored relative to their group, as React Flow holds them.
        const placed = new Map(nodesRef.current.map((item) => [item.id, item.position]));
        placed.set(node.id, node.position);
        void putLayout(cacheKey, {
            positions: ids.map((id) => {
                const point = placed.get(id) ?? { x: 0, y: 0 };
                return { id, x: point.x, y: point.y };
            }),
        }).then(() => saveLayoutSig(cacheKey, layoutSignature(activeLevel?.children ?? []))).catch((err: unknown) => {
            setError(err instanceof Error ? err.message : "layout save failed");
        });
    };

    const groupIds = useMemo(() => (relationshipId ? [] : groupableSelection(nodes)), [nodes, relationshipId]);
    const openGroupForm = useCallback(() => {
        openerRef.current = groupButtonRef.current;
        const names = (levelRef.current?.children ?? []).map((child) => child.name);
        setRequest({ kind: "group", ids: groupIds, suggestion: defaultGroupName(names) });
    }, [groupIds]);

    const hint = useMemo(() => {
        if (relationshipId) return null;
        const kids = level?.children ?? [];
        return overBudgetHint(
            kids.length,
            (level?.view_defaults ?? fallbackViewDefaults).level_budget,
            expandedGroups(kids),
        );
    }, [level, relationshipId]);

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
                <p className="pointer-events-none absolute top-16 left-1/2 z-50 max-w-md -translate-x-1/2 rounded-full border border-destructive/30 bg-card px-3 py-1.5 text-sm text-destructive">
                    {error}
                </p>
            ) : null}
            {notice || hint ? (
                <div className="pointer-events-none absolute top-24 left-1/2 z-50 flex max-w-md -translate-x-1/2 flex-col items-center gap-2">
                    {notice ? (
                        <p
                            role={notice.kind === "error" ? "alert" : "status"}
                            className={`flex items-center gap-2 rounded-full border bg-card px-3 py-1.5 text-[11px] shadow-sm ${
                                notice.kind === "error" ? "border-destructive/30 text-destructive" : "border-border text-foreground"
                            }`}
                        >
                            <span>{notice.text}</span>
                            {notice.kind === "staged" && onProposalStaged ? (
                                <button
                                    type="button"
                                    onClick={onProposalStaged}
                                    className="pointer-events-auto inline-flex h-6 items-center rounded-full border border-border px-2.5 font-medium hover:bg-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
                                >
                                    Open Proposals
                                </button>
                            ) : null}
                        </p>
                    ) : null}
                    {hint ? (
                        <p role="status" className="rounded-full border border-border bg-card px-3 py-1.5 text-sm text-muted-foreground shadow-sm">
                            {hint.message}
                        </p>
                    ) : null}
                </div>
            ) : null}
            {request || groupIds.length >= 2 ? (
                <div className="pointer-events-none absolute bottom-20 left-1/2 z-50 -translate-x-1/2">
                    {request ? (
                        <StructurePanel
                            request={request}
                            busy={staging}
                            onConfirm={(value) => void confirmRequest(value)}
                            onCancel={closeRequest}
                        />
                    ) : (
                        <button
                            ref={groupButtonRef}
                            type="button"
                            aria-label={groupButtonLabel(groupIds.length)}
                            onClick={openGroupForm}
                            className="pointer-events-auto inline-flex h-7 items-center rounded-full border border-border bg-card px-3 text-[11px] font-medium text-foreground shadow-sm hover:bg-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
                        >
                            {groupButtonLabel(groupIds.length)}
                        </button>
                    )}
                </div>
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
                onNodeDragStart={onNodeDragStart}
                onNodeDragStop={onNodeDragStop}
                panOnDrag={tool === "grab"}
                selectionOnDrag={tool === "pointer"}
                nodesConnectable={false}
                proOptions={{ hideAttribution: true }}
            >
                <Background gap={24} size={1} color="var(--canvas-dot)" />
            </ReactFlow>
        </div>
    );
}
