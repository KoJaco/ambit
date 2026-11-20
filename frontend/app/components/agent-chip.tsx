import { Bot } from "lucide-react";
import { Tooltip } from "./tooltip";

// Marks the node that scopes the active coding agent. Read-only; changed in the inspector.
export function AgentChip() {
    return (
        <Tooltip label="Active agent assignment: agents are limited to this node's scope">
            <span
                tabIndex={0}
                aria-label="Assigned"
                className="inline-flex shrink-0 items-center gap-1 rounded-full bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium text-primary focus-visible:outline-2 focus-visible:outline-ring"
            >
                <Bot className="size-3" aria-hidden />
                Agent
            </span>
        </Tooltip>
    );
}
