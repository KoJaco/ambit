import { useEffect, useState } from "react";
import { Link } from "react-router";
import { getNode } from "../api";

export function Breadcrumbs({ nodeId, refreshKey }: { nodeId?: string; refreshKey: number }) {
    const [crumbs, setCrumbs] = useState<{ id: string; name: string }[]>([]);

    useEffect(() => {
        if (!nodeId) {
            setCrumbs([]);
            return;
        }
        const ac = new AbortController();
        let cancelled = false;
        setCrumbs([]);
        (async () => {
            const chain: { id: string; name: string }[] = [];
            const seen = new Set<string>();
            let current: string | undefined = nodeId;
            while (current && !seen.has(current)) {
                seen.add(current);
                const node = await getNode(current, ac.signal);
                chain.push({ id: node.id, name: node.name });
                current = node.parent_id || undefined;
            }
            chain.reverse();
            if (!cancelled) setCrumbs(chain);
        })().catch((err: unknown) => {
            if (cancelled || (err instanceof DOMException && err.name === "AbortError")) return;
            setCrumbs([]);
        });
        return () => {
            cancelled = true;
            ac.abort();
        };
    }, [nodeId, refreshKey]);

    return (
        <nav aria-label="Model" className="flex items-center gap-1 text-sm leading-none text-foreground">
            {nodeId ? (
                <Link to="/" className="rounded px-1 hover:bg-foreground/10">
                    Model
                </Link>
            ) : (
                <span className="px-1">Model</span>
            )}
            {crumbs.map((crumb, index) => {
                const current = index === crumbs.length - 1;
                return (
                    <span key={crumb.id} className="flex items-center gap-1">
                        <span className="text-foreground/40">/</span>
                        {current ? (
                            <span className="px-1">{crumb.name}</span>
                        ) : (
                            <Link to={`/node/${crumb.id}`} className="rounded px-1 hover:bg-foreground/10">
                                {crumb.name}
                            </Link>
                        )}
                    </span>
                );
            })}
        </nav>
    );
}
