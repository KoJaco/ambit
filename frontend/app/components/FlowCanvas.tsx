import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
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
import clsx from "clsx";
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
    pruneOrphanedGroupSizes,
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
    acceptsDrop,
    containerAt,
    drillsOnDoubleClick,
    resolveDropTarget,
    straddledGroups,
    defaultGroupName,
    groupableSelection,
    stageErrorMessage,
    stagedMessage,
    type Rect,
} from "../canvas-structure";
import type { StructureRequest } from "./structure-panel";
import { barStore } from "../bottom-bar";
import { history } from "../history";
import { INSPECTOR_CLOSE_REQUEST_EVENT, isInspectorDirty, paneClickCloses, useRailState } from "../right-rail-state";
import { canvasEdgeType, canvasNodeType, isStackedCard, nodeLayoutSize } from "../canvas-presentation";
import { buildExternalStubs, isStubActivationKey, stubAriaLabel } from "../external-stubs";
import { computeGroupSizes, containerSize, layoutCompound, layoutLevel, maxSizes, pushOverlaps, seedPositions, type NodeSize, type Point } from "../layout";
import {
    canExpandChild,
    expandedGroups,
    groupTints,
    sizeFloors,
    layoutSignature,
    layoutUsable,
    memberSignature,
    usableGroupSizes,
    fallbackViewDefaults,
    maxLevelFetches,
    overBudgetHint,
    resolveExpanded,
    toggleMode,
    type ViewModes,
} from "../level-view";
import { NodeCard, type ArchNode, type ArchNodeData } from "./NodeCard";
import { ArchEdge } from "./ArchEdge";
import { AnimatedEdge } from "./flow/animated-edge";
import { ButtonEdge } from "./flow/button-edge";
import { DataEdge } from "./flow/data-edge";
import { ContainerNode, GroupFrame } from "./flow/group-node";
import { ExternalStub } from "./flow/external-node";
import { GhostEdge, type GhostEdgeData } from "./flow/ghost-edge";
import { GhostNode, ghostNodeClass } from "./flow/ghost-node";
import { buildGhostPreview, emptyGhostPreview, ghostLeafSize, layoutCovers, placeGhosts, type CanvasPreview, type GhostLayout, type Rect as GhostRect } from "../ghost-preview";
import { partSelection } from "../part-selection";
import { scopeRootLevel } from "../parts-scope";
import { PlaceholderFrame } from "./flow/placeholder-node";
import { SchemaNode } from "./flow/schema-node";

// Shift joins the default Meta/Control; Shift + drag on the pane still box-selects (selectionKeyCode).
const multiSelectionKeys = ["Shift", "Meta", "Control"];
const nodeTypes = {
    card: NodeCard,
    schema: SchemaNode,
    placeholder: PlaceholderFrame,
    group: GroupFrame,
    container: ContainerNode,
    external: ExternalStub,
    ghost: GhostNode,
};
const edgeTypes = {
    arch: ArchEdge,
    button: ButtonEdge,
    animated: AnimatedEdge,
    data: DataEdge,
    ghost: GhostEdge,
};

function nodeData(
    child: SummaryLike,
    onToggleMode?: (id: string) => void,
    onUngroup?: (id: string) => void,
): ArchNodeData {
    return {
        name: child.name,
        type: child.type,
        status: child.status,
        protected: child.protected,
        assigned: child.assigned,
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
    assigned?: boolean;
};

// Footprint per child; collapsed groups are drawn as stacked cards and reserve the sheet offset.
function layoutSizes(kids: SummaryLike[], stubIds: string[] = []): Record<string, NodeSize> {
    const sizes: Record<string, NodeSize> = {};
    for (const id of stubIds) sizes[id] = nodeLayoutSize("external");
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
    baseKey: string,
    // Sizes groups already had on screen: a toggle may grow a group but never shrinks it. Empty when
    // membership changed, so a group that lost members shrinks back to fit.
    floor: Record<string, NodeSize> = {},
) {
    // Only the architect's own resizes count, and only while the group still has the members it had then.
    const resized = usableGroupSizes(loadGroupSizes(baseKey), compound);
    return {
        mins: computeGroupSizes(compound, positions, sizes),
        applied: computeGroupSizes(compound, positions, sizes, maxSizes(resized, floor)),
    };
}

// Left padding for fit-view so the sidebar overlay does not cover the first column.
// Room kept clear for chrome that floats over the canvas: the sidebar on the left and, when a
// right-rail sheet is open, the sheet (same width as the rail in right-rail-stack).
export function fitInsets(viewportWidth: number, railOpen: boolean) {
    const left = Math.round(Math.min(290, viewportWidth * 0.3));
    const right = railOpen ? Math.round(Math.min(440, viewportWidth * 0.92)) + 24 : 0;
    return { left, right };
}

function fitPadding(railOpen: boolean) {
    const { left, right } = fitInsets(window.innerWidth, railOpen);
    return {
        top: 0.2,
        bottom: 0.2,
        left: `${left}px` as const,
        right: right > 0 ? (`${right}px` as const) : 0.2,
    };
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
    const { stubs } = buildExternalStubs(level.crossings);
    const stubNodes: ArchNode[] = stubs.map((stub, index) => ({
        id: stub.id,
        type: "external" as const,
        selectable: false,
        // Only a stub that opens a relationship is a keyboard stop; it is then a button.
        focusable: stub.drillable,
        ariaRole: stub.drillable ? ("button" as const) : undefined,
        ariaLabel: stub.drillable ? stubAriaLabel(stub.label, stub.direction, "outside this level") : undefined,
        className: "group/stub",
        // Movable (layout only, persisted with the level's positions) but never selectable or deletable.
        draggable: true,
        deletable: false,
        position: positions[stub.id] ?? { x: -200, y: index * 80 },
        data: {
            name: stub.label || stub.otherId,
            type: "external",
            status: "",
            protected: false,
            external: {
                otherId: stub.otherId,
                label: stub.label,
                direction: stub.direction,
                straightSide: stub.straightSide,
                relationshipId: stub.relationshipId,
                drillable: stub.drillable,
            },
        },
    }));
    const archNodes = children.map((child, index) => {
        const parentId = child.group_id && present.has(child.group_id) ? child.group_id : undefined;
        const type = canvasNodeType(child.renderer, child.child_count, child.expanded);
        // An open group with no members left still needs a size, or it collapses to its header.
        const size = groupSizes[child.id] ?? (type === "container" ? containerSize([], {}, {}) : undefined);
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
                ...nodeData(child, toggleFor(child, level, onToggleMode), onUngroup),
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
    return [...archNodes, ...stubNodes];
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
        left.assigned !== right.assigned ||
        left.icon !== right.icon ||
        left.color !== right.color ||
        left.renderer !== right.renderer ||
        left.childCount !== right.childCount ||
        left.expanded !== right.expanded ||
        left.depth !== right.depth ||
        left.internalRelCount !== right.internalRelCount ||
        (left.typeMix ?? []).map((entry) => `${entry.type}:${entry.count}`).join() !==
            (right.typeMix ?? []).map((entry) => `${entry.type}:${entry.count}`).join() ||
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
    return true;
}

// Fetch a level, then refine the expand set by the 0010 defaults. The client only learns about
// depth-2 candidates from a response, so this is bounded to maxLevelFetches requests.
async function loadLevel(
    nodeId: string | undefined,
    modes: ViewModes,
    signal: AbortSignal,
): Promise<{ level: Level; partId: string }> {
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
    // The part is read once so the level and the part it was scoped with stay together.
    const part = partSelection.get().part;
    return { level: scopeRootLevel(body, part), partId: part && !nodeId ? part.id : "" };
}

export default function FlowCanvas({
    nodeId,
    relationshipId,
    tool,
    refreshKey = 0,
    onSelectNode,
    inspectedId,
    onSelectEdge,
    onRequestRecenterRef,
    onRequestZoomRef,
    onProposalStaged,
    onPaneClick: onPaneClickHost,
    preview,
}: {
    nodeId?: string;
    relationshipId?: string;
    tool: "grab" | "pointer";
    refreshKey?: number;
    onSelectNode?: (id: string | null) => void;
    // Node open in the inspector; keeps a persistent ring even after the canvas selection is cleared.
    inspectedId?: string | null;
    onSelectEdge?: (edge: Relationship | null) => void;
    onRequestRecenterRef?: (fn: () => void) => void;
    onRequestZoomRef?: (api: { zoomIn: () => void; zoomOut: () => void }) => void;
    // Called after a structure edit staged a proposal; the host brings the review panel forward.
    onProposalStaged?: () => void;
    // Called after a click on empty canvas (the canvas has already cleared selection and inspector).
    onPaneClick?: () => void;
    // Pending proposals drawn as ghosts over this level. Display only: never saved or laid out.
    preview?: CanvasPreview;
}) {
    const navigate = useNavigate();
    const { zoomIn, zoomOut, fitView, screenToFlowPosition, getInternalNode } = useReactFlow();
    const railOpenRef = useRef(false);
    railOpenRef.current = useRailState().panels > 0;
    const [level, setLevel] = useState<Level | null>(null);
    // Part the shown level was scoped to ("" for the whole model).
    const [levelPartId, setLevelPartId] = useState("");
    // Re-fetch (and re-scope the root level) when the selected part changes.
    const selectedPart = useSyncExternalStore(partSelection.subscribe, () => partSelection.get().part, () => null);
    const [relLevel, setRelLevel] = useState<RelationshipLevel | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [notFound, setNotFound] = useState(false);
    const [nodes, setNodes] = useState<ArchNode[]>([]);
    const [fitToken, setFitToken] = useState(0);
    const [notice, setNotice] = useState<{ kind: "info" | "staged" | "error"; text: string } | null>(null);
    const [request, setRequest] = useState<StructureRequest | null>(null);
    const [staging, setStaging] = useState(false);
    const openerRef = useRef<HTMLElement | null>(null);
    const nodesRef = useRef(nodes);
    nodesRef.current = nodes;
    const dragStart = useRef<{ id: string; position: Point } | null>(null);
    // Group or summary card the dragged node would land in, ringed while the drag is over it.
    const [dropTargetId, setDropTargetId] = useState<string | null>(null);

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
        onRequestRecenterRef?.(() => void fitView({ padding: fitPadding(railOpenRef.current) }));
    }, [fitView, onRequestRecenterRef, onRequestZoomRef, zoomIn, zoomOut]);

    useEffect(() => {
        if (fitToken === 0) return;
        void fitView({ padding: fitPadding(railOpenRef.current) });
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
    // The level the canvas shows: the relationship interior when open, else the node level.
    const activeLevelRef = useRef<Level | RelationshipLevel | null>(null);
    activeLevelRef.current = relLevel ?? level;
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
    // Group sizes are keyed by the level (base key), not the expanded set, so they survive toggles.
    const onResizeGroup = useCallback((id: string, size: NodeSize) => {
        const members = (activeLevelRef.current?.children ?? []).map((child) => ({ id: child.id, parentId: child.group_id }));
        saveGroupSizes(baseKey, {
            ...loadGroupSizes(baseKey),
            [id]: { width: Math.round(size.width), height: Math.round(size.height), sig: memberSignature(id, members) },
        });
    }, [baseKey]);

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
    const stubKey = buildExternalStubs(activeLevel?.crossings)
        .stubs.map((stub) => `${stub.id}@${stub.nodeId}`)
        .join("\n");
    const expandedIds = useMemo(
        () => (activeLevel?.children ?? []).filter((child) => child.expanded).map((child) => child.id),
        [activeLevel],
    );
    const expandedSig = expandedIds.join("\0");
    const cacheKey = layoutCacheKey({ nodeId, relationshipId, expanded: expandedIds, partId: relLevel ? undefined : levelPartId });
    cacheKeyRef.current = cacheKey;
    // Base and full key of the layout last applied; a change of only the full key is a group toggle.
    const appliedRef = useRef<{ base: string; key: string; sig?: string; part: string } | null>(null);
    // Set while a toggle's layout is still in flight, so toggling straight back (same key as the
    // one on screen) still takes the keep-positions path instead of refitting the viewport.
    const toggleIntentRef = useRef(false);

    // Older builds saved auto-grown sizes under `<base>~<hash>` keys that nothing reads any more.
    useEffect(() => {
        pruneOrphanedGroupSizes();
    }, []);

    useEffect(() => {
        if (!activeLevel) return;
        if (relationshipId && !relLevel) return;
        if (!relationshipId && (nodeId ?? "") !== parentId && nodeId !== undefined) return;
        let cancelled = false;
        const kids = activeLevel.children ?? [];
        // Edge stubs take part in layout so room is reserved for them next to the node they attach to.
        const ext = buildExternalStubs(activeLevel.crossings);
        const stubIds = ext.stubs.map((stub) => stub.id);
        const ids = [...kids.map((c) => c.id), ...stubIds];
        const compound = [...kids.map((c) => ({ id: c.id, parentId: c.group_id })), ...stubIds.map((id) => ({ id }))];
        const sizes = layoutSizes(kids, stubIds);
        const edges = [
            ...(activeLevel.relationships ?? []).map((rel) => ({ from: rel.from, to: rel.to })),
            ...ext.edges.map((edge) => ({ from: edge.source, to: edge.target })),
        ];
        const nested = kids.some((c) => c.group_id);
        const last = appliedRef.current;
        const toggled =
            !!last && last.base === baseKey && last.part === levelPartId && (last.key !== cacheKey || toggleIntentRef.current) && nodesRef.current.length > 0;
        if (toggled) toggleIntentRef.current = true;
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
                // Data-driven membership change (a proposal was accepted, nodes grouped or moved): the
                // server drops the saved layout on structure edits, so what is on screen is the only
                // record of the architect's arrangement. Keep it and place only what is new.
                const lastApplied = appliedRef.current;
                const membershipChanged =
                    !toggled && !usable && !!lastApplied && lastApplied.base === baseKey && lastApplied.part === levelPartId && nodesRef.current.length > 0;
                if (toggled || membershipChanged) {
                    // Keep every placed node where it is, place only what was revealed or changed,
                    // grow groups only when needed, and leave the viewport alone.
                    const prev = nodesRef.current;
                    const seeded = seedPositions(
                        compound,
                        prev.map((item) => ({ id: item.id, parentId: item.parentId, position: item.position })),
                        sizes,
                        usable ? stored : {},
                    );
                    // Sizes kept nodes had on screen: the floor for containers, and the baseline
                    // growth is measured against when pushing siblings clear.
                    const oldSizes: Record<string, NodeSize> = {};
                    const onScreen: Record<string, NodeSize> = {};
                    for (const item of prev) {
                        const width = item.style?.width;
                        const height = item.style?.height;
                        if (item.type === "container" && typeof width === "number" && typeof height === "number") {
                            onScreen[item.id] = { width, height };
                            oldSizes[item.id] = { width, height };
                        } else {
                            const w = item.measured?.width ?? item.width;
                            const h = item.measured?.height ?? item.height;
                            if (typeof w === "number" && typeof h === "number") oldSizes[item.id] = { width: w, height: h };
                        }
                    }
                    // Only a toggle keeps on-screen sizes as a floor, and only for containers whose own
                    // members are unchanged; a group that lost or gained a member shrinks to fit.
                    const floor = sizeFloors(
                        toggled,
                        onScreen,
                        prev.map((item) => ({ id: item.id, parentId: item.parentId })),
                        compound,
                    );
                    // Growth (card -> container, or a container gaining members) can run into later
                    // siblings; push them clear, then re-measure since moved members can grow a parent.
                    let placed = seeded;
                    let containers = containerSizing(compound, placed, sizes, baseKey, floor);
                    for (let round = 0; round < 4; round += 1) {
                        const nextSizes = { ...sizes, ...containers.applied };
                        const pushed = pushOverlaps(compound, placed, oldSizes, nextSizes);
                        if (compound.every((item) => pushed[item.id]?.x === placed[item.id]?.x && pushed[item.id]?.y === placed[item.id]?.y)) break;
                        placed = pushed;
                        containers = containerSizing(compound, placed, sizes, baseKey, floor);
                    }
                    // Auto-grown sizes are deliberately not persisted: only the architect's own
                    // resizes (onResizeGroup) are a permanent floor, so groups shrink on a relayout.
                    appliedRef.current = { base: baseKey, key: cacheKey, sig, part: levelPartId };
                    toggleIntentRef.current = false;
                    setNodes(buildArchNodes(activeLevel, placed, containers, onToggleMode, onUngroup, onResizeGroup));
                    await putLayout(cacheKey, {
                        positions: ids.map((id) => ({ id, x: placed[id]?.x ?? 0, y: placed[id]?.y ?? 0 })),
                        ports: cached.ports,
                        attachments: cached.attachments,
                    });
                    saveLayoutSig(cacheKey, sig);
                    return;
                }
                if (usable) {
                    appliedRef.current = { base: baseKey, key: cacheKey, sig, part: levelPartId };
                    toggleIntentRef.current = false;
                    setNodes(buildArchNodes(activeLevel, stored, containerSizing(compound, stored, sizes, baseKey), onToggleMode, onUngroup, onResizeGroup));
                    setFitToken((v) => v + 1);
                    return;
                }
                const laid = nested
                    ? (await layoutCompound(compound, edges, sizes)).positions
                    : await layoutLevel(ids, edges, sizes);
                if (cancelled) return;
                appliedRef.current = { base: baseKey, key: cacheKey, sig, part: levelPartId };
                toggleIntentRef.current = false;
                setNodes(buildArchNodes(activeLevel, laid, containerSizing(compound, laid, sizes, baseKey), onToggleMode, onUngroup, onResizeGroup));
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
    }, [nodeId, relationshipId, parentId, childIds, edgeKey, stubKey, cacheKey, baseKey, expandedSig, levelPartId, activeLevel, relLevel, onToggleMode, onUngroup, onResizeGroup]);

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
                .then(({ level: body, partId }) => {
                    setLevel(body);
                    setLevelPartId(partId);
                })
                .catch((err: unknown) => {
                    if (err instanceof DOMException && err.name === "AbortError") return;
                    setLevel(null);
                    setError(err instanceof Error ? err.message : "level request failed");
                });
        }
        return () => ac.abort();
    }, [nodeId, relationshipId, refreshKey, viewModes, selectedPart]);

    useEffect(() => {
        if (!activeLevel) return;
        setNodes((current) => {
            if (current.length === 0) return current;
            const children = new Map((activeLevel.children ?? []).map((child) => [child.id, child]));
            let changed = false;
            const next = current.flatMap((node) => {
                if (node.type === "external") return [node];
                const child = children.get(node.id);
                if (!child) {
                    changed = true;
                    return [];
                }
                const data = nodeData(child, toggleFor(child, activeLevel, onToggleMode), onUngroup);
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
        const stubEdges: Edge[] = buildExternalStubs(activeLevel?.crossings).edges.map((edge) => ({
            id: edge.id,
            type: canvasEdgeType(edge.kind),
            source: edge.source,
            target: edge.target,
            markerEnd: { type: MarkerType.ArrowClosed },
            style: { cursor: edge.rel.drillable ? "pointer" : "default" },
            data: {
                drillable: edge.rel.drillable,
                rel: edge.rel,
                fromNodeId: nodeId,
                labelIndex: 0,
                labelCount: 1,
            },
        }));
        const regular = rels.map((rel) => {
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
        return [...regular, ...stubEdges];
    }, [activeLevel, nodeId]);

    // Ghost preview: derived from the proposals and the level on screen, drawn only through
    // displayNodes/displayEdges. `nodes`, the layout cache and the group-size store never see it.
    const ghost = useMemo(() => {
        if (!preview || relationshipId || !level || (level.node?.id ?? "") !== (nodeId ?? "")) return emptyGhostPreview;
        return buildGhostPreview({
            diffs: preview.diffs,
            parentId: level.node?.id ?? null,
            nodeIds: (level.children ?? []).map((child) => child.id),
            edgeIds: (level.relationships ?? []).map((rel) => rel.id),
        });
    }, [preview?.diffs, level, relationshipId, nodeId]); // eslint-disable-line react-hooks/exhaustive-deps
    // Placement is held still while a node is being dragged so ghosts do not chase the pointer.
    const ghostLayoutRef = useRef<GhostLayout | null>(null);
    const ghostLayout = useMemo(() => {
        if (ghost.ghostNodes.length === 0) return null;
        if (nodes.some((item) => item.dragging) && layoutCovers(ghostLayoutRef.current, ghost.ghostNodes)) return ghostLayoutRef.current;
        const obstacles: Record<string, GhostRect> = {};
        for (const item of nodes) {
            if (item.parentId) continue;
            const styleW = typeof item.style?.width === "number" ? item.style.width : undefined;
            const styleH = typeof item.style?.height === "number" ? item.style.height : undefined;
            obstacles[item.id] = {
                x: item.position.x,
                y: item.position.y,
                width: item.measured?.width ?? styleW ?? ghostLeafSize.width,
                height: item.measured?.height ?? styleH ?? ghostLeafSize.height,
            };
        }
        ghostLayoutRef.current = placeGhosts(ghost.ghostNodes, ghost.ghostEdges, obstacles);
        return ghostLayoutRef.current;
    }, [ghost, nodes]);
    const highlightKey = preview?.highlightKey ?? null;
    const previewNodes = useMemo<ArchNode[]>(() => {
        if (!preview || (ghost.ghostNodes.length === 0 && Object.keys(ghost.nodeMarks).length === 0)) return nodes;
        const real = nodes.map((item) => {
            const mark = ghost.nodeMarks[item.id];
            if (!mark) return item;
            return {
                ...item,
                className: clsx(item.className, mark.kind === "removed" ? "ghost-removed" : "ghost-changed", highlightKey === mark.key && "ghost-highlight"),
                ariaLabel: `${item.data.name}, proposed ${mark.kind === "removed" ? "for removal" : "to change"}`,
            };
        });
        const placed = ghostLayout;
        const fresh: ArchNode[] = placed
            ? ghost.ghostNodes.map((item) => ({
                  id: item.id,
                  type: "ghost" as const,
                  className: ghostNodeClass(highlightKey === item.key),
                  selectable: false,
                  draggable: false,
                  deletable: false,
                  connectable: false,
                  focusable: true,
                  ariaRole: "button" as const,
                  ariaLabel: `Proposed new node ${item.name}. Open in Review`,
                  ...(item.parentGhostId ? { parentId: item.parentGhostId, extent: "parent" as const } : {}),
                  position: placed.positions[item.id] ?? { x: 0, y: 0 },
                  width: (placed.sizes[item.id] ?? ghostLeafSize).width,
                  height: (placed.sizes[item.id] ?? ghostLeafSize).height,
                  // Ghosts are not in `nodes`, so React Flow's measurements are never fed back to them.
                  measured: placed.sizes[item.id] ?? ghostLeafSize,
                  data: {
                      name: item.name,
                      type: item.type,
                      status: "",
                      protected: false,
                      icon: item.icon,
                      color: item.color,
                      ghost: { container: item.hasChildren },
                  },
              }))
            : [];
        return [...real, ...fresh];
    }, [nodes, ghost, ghostLayout, highlightKey, preview]);
    const displayNodes = useMemo<ArchNode[]>(() => {
        const marked = (id: string) => id === inspectedId || id === dropTargetId;
        if (!previewNodes.some((item) => item.type !== "ghost" && marked(item.id))) return previewNodes;
        return previewNodes.map((item) =>
            item.type !== "ghost" && marked(item.id)
                ? {
                      ...item,
                      data: {
                          ...item.data,
                          ...(item.id === inspectedId ? { inspected: true } : {}),
                          ...(item.id === dropTargetId ? { dropTarget: true } : {}),
                      },
                  }
                : item,
        );
    }, [previewNodes, inspectedId, dropTargetId]);
    const displayEdges = useMemo<Edge[]>(() => {
        if (!preview || (ghost.ghostEdges.length === 0 && Object.keys(ghost.edgeMarks).length === 0)) return edges;
        const variantProps = (variant: "new" | "changed" | "removed", label: string, key: string, proposalId: string, index: number, hasContract?: boolean): GhostEdgeData["ghost"] => ({
            variant,
            label,
            hasContract,
            highlighted: highlightKey === key,
            onActivate: () => preview.onActivate(proposalId, index),
            onHover: (on) => preview.onHover(on ? key : null),
        });
        const real = edges.map((edge) => {
            const mark = ghost.edgeMarks[edge.id];
            if (!mark) return edge;
            return { ...edge, type: "ghost", data: { ...edge.data, ghost: variantProps(mark.kind, mark.label, mark.key, mark.proposalId, mark.index, mark.hasContract) } };
        });
        const fresh: Edge[] = ghost.ghostEdges.map((edge) => ({
            id: edge.id,
            type: "ghost",
            source: edge.source,
            target: edge.target,
            selectable: false,
            focusable: false,
            markerEnd: { type: MarkerType.ArrowClosed },
            data: { ghost: variantProps("new", edge.label, edge.key, edge.proposalId, edge.index, edge.hasContract) },
        }));
        return [...real, ...fresh];
    }, [edges, ghost, highlightKey, preview]);
    const activateGhost = (id: string) => {
        const item = ghost.ghostNodes.find((candidate) => candidate.id === id);
        if (item) preview?.onActivate(item.proposalId, item.index);
    };
    const hoverNode: NodeMouseHandler<ArchNode> = (_event, node) => {
        const key = node.type === "ghost" ? ghost.ghostNodes.find((item) => item.id === node.id)?.key : ghost.nodeMarks[node.id]?.key;
        if (key) preview?.onHover(key);
    };
    const unhoverNode: NodeMouseHandler<ArchNode> = (_event, node) => {
        if (node.type === "ghost" || ghost.nodeMarks[node.id]) preview?.onHover(null);
    };

    // Drilling is only for summary cards. An expanded group already shows its children
    // on this level, so opening it would repeat the same picture (decision 0010).
    const openStub = (node: ArchNode) => {
        const stub = node.data.external;
        if (stub?.drillable && stub.relationshipId) {
            navigate(`/relationship/${stub.relationshipId}`, { state: { fromNodeId: nodeId } });
        }
    };
    // Enter/Space on a focused stub acts like a double-click.
    const onStubKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
        if (!isStubActivationKey(event.key)) return;
        const target = event.target as HTMLElement;
        if (!target.classList?.contains("react-flow__node")) return;
        const ghostId = target.getAttribute("data-id") ?? "";
        if (ghost.ghostNodes.some((item) => item.id === ghostId)) {
            event.preventDefault();
            activateGhost(ghostId);
            return;
        }
        const node = nodes.find((n) => n.id === target.getAttribute("data-id"));
        if (node?.type !== "external") return;
        event.preventDefault();
        openStub(node);
    };
    const onNodeDoubleClick: NodeMouseHandler<ArchNode> = (_event, node) => {
        if (node.type === "ghost") return;
        if (node.type === "external") {
            openStub(node);
            return;
        }
        if (!drillsOnDoubleClick(node.type)) return;
        navigate(`/node/${node.id}`);
    };
    const onNodeClick: NodeMouseHandler<ArchNode> = (_event, node) => {
        if (node.type === "ghost") {
            activateGhost(node.id);
            return;
        }
        if (node.type === "external") return;
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
        // Above every group while dragging, so it is never hidden behind one it passes over.
        setNodes((current) => current.map((item) => (item.id === node.id ? { ...item, zIndex: 1000 } : item)));
    };

    // Absolute rects of nodes that accept a drop (open groups and collapsed summary cards), from
    // React Flow's measured nodes.
    function containerRects(): Rect[] {
        const rects: Rect[] = [];
        for (const item of nodesRef.current) {
            if (!acceptsDrop(item)) continue;
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

    // Where the dragged node would be reparented, read from the pointer ("" is the level itself,
    // null is no change). Sibling groups and cards at the same level count as targets.
    function dropTargetAt(event: MouseEvent | TouchEvent, node: ArchNode): string | null {
        if (node.type === "external") return null;
        const pointer = pointerPoint(event);
        if (!pointer) return null;
        return resolveDropTarget({
            nodeId: node.id,
            nodes: nodesRef.current.map((item) => ({ id: item.id, parentId: item.parentId })),
            rects: containerRects(),
            point: screenToFlowPosition(pointer),
            levelNodeId: level?.node?.id,
        });
    }

    // Open groups the dragged node now half overlaps (absolute coordinates).
    function straddled(node: ArchNode): string[] {
        const internal = getInternalNode(node.id);
        if (!internal) return [];
        const origin = internal.internals.positionAbsolute;
        const groups: Rect[] = containerRects().filter((rect) => nodesRef.current.find((item) => item.id === rect.id)?.type === "container");
        return straddledGroups(
            { id: node.id, x: origin.x, y: origin.y, width: internal.measured.width ?? 0, height: internal.measured.height ?? 0 },
            groups,
            nodesRef.current.map((item) => ({ id: item.id, parentId: item.parentId })),
        );
    }

    const onNodeDrag: OnNodeDrag<ArchNode> = (event, node) => {
        const target = dropTargetAt(event, node);
        // "" is the level itself (no node to ring), and a node never targets its own parent group.
        const next = target && target !== node.parentId ? target : null;
        setDropTargetId((current) => (current === next ? current : next));
    };

    const onNodeDragStop: OnNodeDrag<ArchNode> = (event, node) => {
        setDropTargetId(null);
        const target = dropTargetAt(event, node);
        const outside = target !== null && target !== (node.parentId ?? null) ? target : null;
        const start = dragStart.current?.id === node.id ? dragStart.current.position : null;
        // The raised layer only lasts for the drag.
        const settle = (position?: Point) =>
            setNodes((current) =>
                current.map((item) =>
                    item.id === node.id ? { ...item, zIndex: undefined, ...(position ? { position } : {}) } : item,
                ),
            );
        if (outside !== null) {
            settle(start ?? undefined);
            requestReparent(node.id, outside);
            return;
        }
        // Dropped half behind a group it is not joining: snap back rather than leave it unreachable.
        if (start && straddled(node).length > 0) {
            settle(start);
            setNotice({ kind: "info", text: "Moved back. Drop a node onto a group to move it inside." });
            return;
        }
        settle();
        const ids = [
            ...(activeLevel?.children ?? []).map((child) => child.id),
            ...buildExternalStubs(activeLevel?.crossings).stubs.map((stub) => stub.id),
        ];
        // Members are stored relative to their group, as React Flow holds them.
        const placed = new Map(nodesRef.current.map((item) => [item.id, item.position]));
        placed.set(node.id, node.position);
        const toPositions = (map: Map<string, Point>) =>
            ids.map((id) => {
                const point = map.get(id) ?? { x: 0, y: 0 };
                return { id, x: point.x, y: point.y };
            });
        if (start && (start.x !== node.position.x || start.y !== node.position.y)) {
            const movedTo = node.position;
            const key = cacheKey;
            const sig = layoutSignature(activeLevel?.children ?? []);
            const before = new Map(placed).set(node.id, start);
            const move = async (to: Point) => {
                await putLayout(key, { positions: toPositions(new Map(before).set(node.id, to)) });
                saveLayoutSig(key, sig);
                // Only touch the canvas when the level the move happened on is still the one shown.
                if (cacheKeyRef.current === key) {
                    setNodes((current) => current.map((item) => (item.id === node.id ? { ...item, position: to } : item)));
                }
            };
            history.record({
                label: `move ${node.data.name}`,
                undo: () => move(start),
                redo: () => move(movedTo),
            });
        }
        void putLayout(cacheKey, {
            positions: toPositions(placed),
        }).then(() => saveLayoutSig(cacheKey, layoutSignature(activeLevel?.children ?? []))).catch((err: unknown) => {
            setError(err instanceof Error ? err.message : "layout save failed");
        });
    };

    const groupIds = useMemo(() => (relationshipId ? [] : groupableSelection(nodes)), [nodes, relationshipId]);
    const openGroupForm = useCallback((opener: HTMLElement | null) => {
        openerRef.current = opener;
        const names = (levelRef.current?.children ?? []).map((child) => child.name);
        setRequest({ kind: "group", ids: groupIds, suggestion: defaultGroupName(names) });
    }, [groupIds]);

    const clearSelection = useCallback(() => {
        setNodes((current) => current.map((node) => (node.selected ? { ...node, selected: false } : node)));
    }, []);

    // Cancel on the selection bar also closes an inspector left open on the last clicked node.
    const clearSelectionAndInspector = useCallback(() => {
        clearSelection();
        // Escape must not drop unsaved inspector edits: ask the inspector to confirm instead.
        if (isInspectorDirty()) {
            window.dispatchEvent(new Event(INSPECTOR_CLOSE_REQUEST_EVENT));
            return;
        }
        onSelectNode?.(null);
    }, [clearSelection, onSelectNode]);

    // A click on empty canvas closes whatever the right rail shows for this level: the inspector
    // (node or edge) and, through the host, an open Review panel. Escape still works as before.
    const onPaneClick = useCallback(() => {
        clearSelection();
        if (paneClickCloses(isInspectorDirty()).inspector) {
            onSelectEdge?.(null);
            onSelectNode?.(null);
        }
        onPaneClickHost?.();
    }, [clearSelection, onSelectEdge, onSelectNode, onPaneClickHost]);

    // The bottom bar (ControlBar) swaps the toolbar for the group/ungroup form or the selection
    // bar. This canvas owns the state and the staging; the bar only renders it.
    useEffect(() => {
        barStore.set({
            request,
            busy: staging,
            selectedCount: groupIds.length,
            onOpenGroup: openGroupForm,
            onConfirm: (value) => void confirmRequest(value),
            onCancel: closeRequest,
            onClearSelection: clearSelectionAndInspector,
        });
    }, [request, staging, groupIds.length, openGroupForm, confirmRequest, closeRequest, clearSelectionAndInspector]);
    useEffect(() => () => barStore.reset(), []);

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
            <ReactFlow
                nodes={displayNodes}
                edges={displayEdges}
                nodeTypes={nodeTypes}
                multiSelectionKeyCode={multiSelectionKeys}
                edgeTypes={edgeTypes}
                onNodesChange={onNodesChange}
                onKeyDown={onStubKeyDown}
                onNodeClick={onNodeClick}
                onNodeMouseEnter={hoverNode}
                onNodeMouseLeave={unhoverNode}
                onNodeDoubleClick={onNodeDoubleClick}
                onEdgeClick={onEdgeClick}
                onEdgeDoubleClick={onEdgeDoubleClick}
                onNodeDragStart={onNodeDragStart}
                onNodeDrag={onNodeDrag}
                onPaneClick={onPaneClick}
                onNodeDragStop={onNodeDragStop}
                panOnDrag={tool === "grab"}
                selectionOnDrag={tool === "pointer"}
                nodesConnectable={false}
                deleteKeyCode={null}
                proOptions={{ hideAttribution: true }}
            >
                <Background gap={24} size={1} color="var(--canvas-dot)" />
            </ReactFlow>
        </div>
    );
}
