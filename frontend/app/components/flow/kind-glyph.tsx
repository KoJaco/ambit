import type { KindGlyphId } from "../../kind-style";

// Small decorative glyph for a relationship kind. Always aria-hidden: the kind is also named in text.
export function KindGlyphIcon({ glyph, className = "size-4" }: { glyph: KindGlyphId; className?: string }) {
    return (
        <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.5} strokeLinecap="round" className={className} aria-hidden>
            {glyph === "data" ? (
                <>
                    <path d="M3 4h10M3 8h10M3 12h6" />
                </>
            ) : glyph === "dots" ? (
                <>
                    <path d="M2 8h.01M6 8h.01M10 8h.01" strokeWidth={2.5} />
                    <path d="M13 5l3 3-3 3" transform="translate(-2 0)" />
                </>
            ) : (
                <>
                    <path d="M2 8h11" />
                    <path d="M9 4l4 4-4 4" />
                </>
            )}
        </svg>
    );
}
