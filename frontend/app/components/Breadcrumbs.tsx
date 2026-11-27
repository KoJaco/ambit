import { useEffect, useState } from "react";
import { Link, useLocation } from "react-router";
import { getRelationshipLevel } from "../api";
import { nodeAncestorChain, relationshipPrefixCrumbs, type BreadcrumbCrumb } from "../relationship-breadcrumb";

type DrillState = { fromNodeId?: string };

export function Breadcrumbs({
    nodeId,
    relationshipId,
    refreshKey,
}: {
    nodeId?: string;
    relationshipId?: string;
    refreshKey: number;
}) {
    const location = useLocation();
    const drillState = (location.state ?? {}) as DrillState;
    const [crumbs, setCrumbs] = useState<BreadcrumbCrumb[]>([]);
    const [relLabel, setRelLabel] = useState<string | null>(null);

    useEffect(() => {
        if (relationshipId) {
            const ac = new AbortController();
            setCrumbs([]);
            setRelLabel(null);
            getRelationshipLevel(relationshipId, ac.signal)
                .then(async (level) => {
                    setRelLabel(level.relationship.label || level.relationship.id);
                    const prefix = await relationshipPrefixCrumbs(
                        level.relationship,
                        drillState.fromNodeId,
                        ac.signal,
                    );
                    if (!ac.signal.aborted) setCrumbs(prefix);
                })
                .catch(() => {
                    if (!ac.signal.aborted) setRelLabel(relationshipId);
                });
            return () => ac.abort();
        }
        setRelLabel(null);
        if (!nodeId) {
            setCrumbs([]);
            return;
        }
        const ac = new AbortController();
        setCrumbs([]);
        nodeAncestorChain(nodeId, ac.signal, true)
            .then((chain) => {
                if (!ac.signal.aborted) setCrumbs(chain);
            })
            .catch(() => {
                if (!ac.signal.aborted) setCrumbs([]);
            });
        return () => ac.abort();
    }, [nodeId, relationshipId, refreshKey, drillState.fromNodeId]);

    return (
        <nav aria-label="Model" className="flex flex-wrap items-center gap-1 text-sm leading-snug text-foreground">
            <Link to="/" className="rounded px-1 hover:bg-foreground/10">
                Model
            </Link>
            {crumbs.map((crumb) => (
                <span key={crumb.id} className="flex items-center gap-1">
                    <span className="text-foreground/40">/</span>
                    <Link to={crumb.href} className="rounded px-1 hover:bg-foreground/10">
                        {crumb.name}
                    </Link>
                </span>
            ))}
            {relationshipId && relLabel ? (
                <span className="flex items-center gap-1">
                    <span className="text-foreground/40">/</span>
                    <span className="max-w-[14rem] px-1">{relLabel}</span>
                </span>
            ) : null}
        </nav>
    );
}
