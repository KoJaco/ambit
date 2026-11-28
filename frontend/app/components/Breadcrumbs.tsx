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

    const linkClass =
        "rounded px-1 hover:bg-foreground/10 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring";
    // The last crumb is the current page and is not a link to itself. On a relationship page the
    // relationship label is last; otherwise the final node crumb is.
    const currentIsRelationship = Boolean(relationshipId && relLabel);
    const lastIndex = crumbs.length - 1;
    const modelIsCurrent = crumbs.length === 0 && !relationshipId;
    return (
        <nav aria-label="Breadcrumb" className="text-sm leading-snug text-foreground">
            <ol className="flex flex-wrap items-center gap-1">
                <li className="flex items-center gap-1">
                    {modelIsCurrent ? (
                        <span aria-current="page" className="px-1">
                            Model
                        </span>
                    ) : (
                        <Link to="/" className={linkClass}>
                            Model
                        </Link>
                    )}
                </li>
                {crumbs.map((crumb, index) => {
                    const current = !relationshipId && index === lastIndex;
                    return (
                        <li key={crumb.id} className="flex items-center gap-1">
                            <span aria-hidden className="text-foreground/40">
                                /
                            </span>
                            {current ? (
                                <span aria-current="page" className="px-1">
                                    {crumb.name}
                                </span>
                            ) : (
                                <Link to={crumb.href} className={linkClass}>
                                    {crumb.name}
                                </Link>
                            )}
                        </li>
                    );
                })}
                {currentIsRelationship ? (
                    <li className="flex items-center gap-1">
                        <span aria-hidden className="text-foreground/40">
                            /
                        </span>
                        <span aria-current="page" className="max-w-[14rem] px-1">
                            {relLabel}
                        </span>
                    </li>
                ) : null}
            </ol>
        </nav>
    );
}
