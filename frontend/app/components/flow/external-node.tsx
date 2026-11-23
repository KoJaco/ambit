import { useEffect, useState } from "react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import clsx from "clsx";
import { getNode } from "../../api";
import { stubTitle, type StubSide } from "../../external-stubs";
import type { ArchNode } from "../NodeCard";

export type ExternalNodeData = {
    otherId: string;
    label: string;
    direction: string;
    straightSide: StubSide;
    relationshipId?: string;
    drillable: boolean;
};


// Names are looked up once per id: a level can hold several stubs for the same outside node.
const names = new Map<string, Promise<string | undefined>>();
function nodeName(id: string): Promise<string | undefined> {
    let cached = names.get(id);
    if (!cached) {
        cached = getNode(id)
            .then((node) => node.name)
            .catch(() => {
                names.delete(id);
                return undefined;
            });
        names.set(id, cached);
    }
    return cached;
}

function useOtherName(id: string): string | undefined {
    const [name, setName] = useState<string | undefined>();
    useEffect(() => {
        let live = true;
        void nodeName(id).then((value) => {
            if (live) setName(value);
        });
        return () => {
            live = false;
        };
    }, [id]);
    return name;
}

// A half-width card. The side facing away from the relationship is straight, as if the card were
// cut off, while the side the edge attaches to keeps the card's rounding.
export function ExternalStub({ data, selected }: NodeProps<ArchNode>) {
    const stub = data.external;
    const name = useOtherName(stub?.otherId ?? "");
    if (!stub) return null;
    const title = stubTitle(name, stub.label, stub.otherId);
    const straightLeft = stub.straightSide === "left";
    return (
        <div
            className={clsx(
                "flex h-16 w-[110px] items-center border bg-card px-2.5 select-none",
                straightLeft ? "rounded-l-none rounded-r-[14px] border-l-2" : "rounded-l-[14px] rounded-r-none border-r-2",
                stub.drillable ? "cursor-pointer" : "cursor-default",
                selected ? "border-primary shadow-[0_0_0_4px_var(--selection-ring)]" : "border-border",
            )}
            title={stub.drillable ? `${title} (double-click to open the relationship)` : title}
        >
            {straightLeft ? <Handle type="source" position={Position.Right} /> : <Handle type="target" position={Position.Left} />}
            <span className="line-clamp-2 break-words text-xs font-medium text-muted-foreground">{title}</span>
        </div>
    );
}
