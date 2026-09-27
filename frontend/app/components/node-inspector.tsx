import type { Node, Edge } from "reactflow";
import { useReactFlow } from "reactflow";
import { NodeRegistry } from "./node-registry";
import type { NodeKind, PortType } from "./types";

const NodeInspector = ({
    selectedNodeId,
    selectedEdgeId,
    nodes,
    edges,
    onClose,
    resolvePortType,
}: {
    selectedNodeId: string | null;
    selectedEdgeId: string | null;
    nodes: Node[];
    edges: Edge[];
    onClose: () => void;
    resolvePortType: (
        nodeId?: string,
        handleId?: string,
        role?: "source" | "target"
    ) => PortType | undefined;
}) => {
    const rf = useReactFlow();
    return (
        <>
            {/* Right-side sliding inspector */}
            <div
                className="absolute top-1/2 bg-card -translate-y-1/2 right-4 max-h-[80vh] max-w-[240px] min-w-[200px] min-h-[50vh] border border-foreground/25 rounded-xl bg-background/95 backdrop-blur transition-transform duration-300 z-50"
                style={{
                    transform:
                        selectedNodeId || selectedEdgeId
                            ? "translateX(0)"
                            : "translateX(150%)",
                    pointerEvents:
                        selectedNodeId || selectedEdgeId ? "auto" : "none",
                }}
            >
                {(selectedNodeId || selectedEdgeId) && (
                    <div className="h-full overflow-y-auto">
                        <div className="flex items-center justify-between p-4 border-b border-foreground/20">
                            <div className="text-sm font-semibold">
                                {selectedNodeId ? "Node" : "Connection"}
                            </div>

                            <button
                                className="text-xs px-2 py-1 rounded-md border border-foreground/30 hover:bg-foreground/5"
                                onClick={onClose}
                            >
                                Close
                            </button>
                        </div>
                        <div className="p-4 text-sm">
                            {selectedNodeId &&
                                (() => {
                                    const n = nodes.find(
                                        (x) => x.id === selectedNodeId
                                    ) as any;
                                    if (!n) return null;
                                    return (
                                        <>
                                            {(() => {
                                                const def =
                                                    NodeRegistry[
                                                        n.data?.kind as NodeKind
                                                    ];
                                                if (!def?.description)
                                                    return null;
                                                return (
                                                    <div className="text-xs text-foreground/70">
                                                        {def.description}
                                                    </div>
                                                );
                                            })()}
                                            {(() => {
                                                if (n.data?.kind === "If") {
                                                    const branches: number[] =
                                                        Array.isArray(
                                                            n.data?.branches
                                                        )
                                                            ? (n.data
                                                                  .branches as number[])
                                                            : [0];
                                                    const titles: Record<
                                                        number,
                                                        string
                                                    > = (n.data?.branchTitles ||
                                                        {}) as Record<
                                                        number,
                                                        string
                                                    >;
                                                    const updateNodeData = (
                                                        partial: any
                                                    ) => {
                                                        rf.setNodes((nds) =>
                                                            nds.map((node) =>
                                                                node.id === n.id
                                                                    ? {
                                                                          ...node,
                                                                          data: {
                                                                              ...node.data,
                                                                              ...partial,
                                                                          },
                                                                      }
                                                                    : node
                                                            )
                                                        );
                                                    };
                                                    const addBranch = () => {
                                                        const nextIndex =
                                                            branches.length ===
                                                            0
                                                                ? 0
                                                                : Math.max(
                                                                      ...branches
                                                                  ) + 1;
                                                        updateNodeData({
                                                            branches: [
                                                                ...branches,
                                                                nextIndex,
                                                            ],
                                                        });
                                                    };
                                                    const removeBranch = (
                                                        index: number
                                                    ) => {
                                                        const remaining =
                                                            branches.filter(
                                                                (b) =>
                                                                    b !== index
                                                            );
                                                        const nextTitles = {
                                                            ...titles,
                                                        } as any;
                                                        delete nextTitles[
                                                            index
                                                        ];
                                                        updateNodeData({
                                                            branches: remaining,
                                                            branchTitles:
                                                                nextTitles,
                                                        });
                                                    };
                                                    return (
                                                        <div className="space-y-2 mt-3">
                                                            <div className="text-xs uppercase text-foreground/60">
                                                                If / Else
                                                            </div>
                                                            <div className="space-y-1">
                                                                {branches.map(
                                                                    (
                                                                        b,
                                                                        idx
                                                                    ) => (
                                                                        <div
                                                                            key={
                                                                                b
                                                                            }
                                                                            className="flex items-center gap-2"
                                                                        >
                                                                            <input
                                                                                id={`clause-${b}`}
                                                                                className="w-full text-xs px-2 py-1 rounded-md border border-foreground/25 bg-background focus-within:outline-none"
                                                                                placeholder={`${idx > 0 && idx === branches.length - 1 ? "Else" : "Condition"}`}
                                                                                value={
                                                                                    titles[
                                                                                        b
                                                                                    ] ??
                                                                                    ""
                                                                                }
                                                                                onChange={(
                                                                                    e
                                                                                ) =>
                                                                                    updateNodeData(
                                                                                        {
                                                                                            branchTitles:
                                                                                                {
                                                                                                    ...titles,
                                                                                                    [b]: e
                                                                                                        .target
                                                                                                        .value,
                                                                                                },
                                                                                        }
                                                                                    )
                                                                                }
                                                                                disabled={
                                                                                    idx >
                                                                                        0 &&
                                                                                    idx ===
                                                                                        branches.length -
                                                                                            1
                                                                                }
                                                                            />
                                                                            {idx >
                                                                                0 && (
                                                                                <button
                                                                                    className="text-xs hover:text-foreground text-foreground/50"
                                                                                    onClick={() =>
                                                                                        removeBranch(
                                                                                            b
                                                                                        )
                                                                                    }
                                                                                    title="Remove branch"
                                                                                >
                                                                                    ×
                                                                                </button>
                                                                            )}
                                                                        </div>
                                                                    )
                                                                )}
                                                                <div className="flex justify-end pt-1">
                                                                    <button
                                                                        className="text-xs hover:text-foreground text-foreground/50 px-2 py-1"
                                                                        onClick={
                                                                            addBranch
                                                                        }
                                                                    >
                                                                        Add else
                                                                        +
                                                                    </button>
                                                                </div>
                                                            </div>
                                                        </div>
                                                    );
                                                }
                                                return null;
                                            })()}
                                            {(() => {
                                                if (n.type === "lane")
                                                    return null;

                                                if (
                                                    n.data?.kind === "audio-in"
                                                ) {
                                                    return (
                                                        <div className="space-y-1 mt-2">
                                                            <div className="text-xs uppercase text-foreground/60">
                                                                Ports
                                                            </div>
                                                            <div className="grid grid-cols-2 gap-2 text-xs">
                                                                <div className="text-foreground/60">
                                                                    Inputs
                                                                </div>
                                                                <div>None</div>
                                                                <div className="text-foreground/60">
                                                                    Outputs
                                                                </div>
                                                                <div>audio</div>
                                                            </div>
                                                            {(() => {
                                                                const outbound =
                                                                    edges
                                                                        .filter(
                                                                            (
                                                                                e
                                                                            ) =>
                                                                                e.source ===
                                                                                n.id
                                                                        )
                                                                        .map(
                                                                            (
                                                                                e
                                                                            ) =>
                                                                                resolvePortType(
                                                                                    e.target,
                                                                                    (e.targetHandle ??
                                                                                        undefined) as
                                                                                        | string
                                                                                        | undefined,
                                                                                    "target"
                                                                                )
                                                                        )
                                                                        .filter(
                                                                            Boolean
                                                                        ) as PortType[];
                                                                if (
                                                                    !outbound.length
                                                                )
                                                                    return null;
                                                                return (
                                                                    <div className="grid grid-cols-2 gap-2 text-xs">
                                                                        <div className="text-foreground/60">
                                                                            Outgoing
                                                                        </div>
                                                                        <div>
                                                                            {outbound.join(
                                                                                ", "
                                                                            )}
                                                                        </div>
                                                                    </div>
                                                                );
                                                            })()}
                                                        </div>
                                                    );
                                                }
                                                const def =
                                                    NodeRegistry[
                                                        n.data?.kind as NodeKind
                                                    ];
                                                const ins = def?.in ?? [];
                                                const outs = def?.out ?? [];
                                                if (!ins.length && !outs.length)
                                                    return null;
                                                return (
                                                    <div className="space-y-1 mt-2">
                                                        <div className="text-xs uppercase text-foreground/60">
                                                            Ports
                                                        </div>
                                                        <div className="grid grid-cols-2 gap-2 text-xs">
                                                            <div className="text-foreground/60">
                                                                Inputs
                                                            </div>
                                                            <div>
                                                                {ins.length
                                                                    ? ins.join(
                                                                          ", "
                                                                      )
                                                                    : "None"}
                                                            </div>
                                                            <div className="text-foreground/60">
                                                                Outputs
                                                            </div>
                                                            <div>
                                                                {outs.length
                                                                    ? outs.join(
                                                                          ", "
                                                                      )
                                                                    : "None"}
                                                            </div>
                                                        </div>
                                                        {(() => {
                                                            const inbound =
                                                                edges
                                                                    .filter(
                                                                        (e) =>
                                                                            e.target ===
                                                                            n.id
                                                                    )
                                                                    .map((e) =>
                                                                        resolvePortType(
                                                                            e.source,
                                                                            (e.sourceHandle ??
                                                                                undefined) as
                                                                                | string
                                                                                | undefined,
                                                                            "source"
                                                                        )
                                                                    )
                                                                    .filter(
                                                                        Boolean
                                                                    ) as PortType[];
                                                            const outbound =
                                                                edges
                                                                    .filter(
                                                                        (e) =>
                                                                            e.source ===
                                                                            n.id
                                                                    )
                                                                    .map((e) =>
                                                                        resolvePortType(
                                                                            e.target,
                                                                            (e.targetHandle ??
                                                                                undefined) as
                                                                                | string
                                                                                | undefined,
                                                                            "target"
                                                                        )
                                                                    )
                                                                    .filter(
                                                                        Boolean
                                                                    ) as PortType[];
                                                            if (
                                                                !inbound.length &&
                                                                !outbound.length
                                                            )
                                                                return null;
                                                            return (
                                                                <div className="grid grid-cols-2 gap-2 text-xs mt-1">
                                                                    <div className="text-foreground/60">
                                                                        Incoming
                                                                    </div>
                                                                    <div>
                                                                        {inbound.join(
                                                                            ", "
                                                                        )}
                                                                    </div>
                                                                    <div className="text-foreground/60">
                                                                        Outgoing
                                                                    </div>
                                                                    <div>
                                                                        {outbound.join(
                                                                            ", "
                                                                        )}
                                                                    </div>
                                                                </div>
                                                            );
                                                        })()}
                                                    </div>
                                                );
                                            })()}
                                        </>
                                    );
                                })()}
                            {selectedEdgeId &&
                                (() => {
                                    const e = edges.find(
                                        (x) => x.id === selectedEdgeId
                                    ) as any;
                                    if (!e) return null;
                                    return (
                                        <div className="space-y-3">
                                            <div className="text-xs uppercase text-foreground/60">
                                                Connection
                                            </div>
                                            <div className="grid grid-cols-2 gap-2">
                                                <div className="text-foreground/60">
                                                    Id
                                                </div>
                                                <div>{e.id}</div>
                                                <div className="text-foreground/60">
                                                    From
                                                </div>
                                                <div>{e.source}</div>
                                                <div className="text-foreground/60">
                                                    To
                                                </div>
                                                <div>{e.target}</div>
                                                <div className="text-foreground/60">
                                                    Type
                                                </div>
                                                <div>{e.type}</div>
                                            </div>
                                        </div>
                                    );
                                })()}
                        </div>
                    </div>
                )}
            </div>
        </>
    );
};

export default NodeInspector;
