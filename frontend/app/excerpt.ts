// Short plain-text preview of a markdown spec: whitespace collapsed, cut at a word boundary.
export function specExcerpt(markdown: string | undefined | null, max = 280): string {
    const text = (markdown ?? "").replace(/\s+/g, " ").trim();
    if (text.length <= max) return text;
    const cut = text.slice(0, max);
    const space = cut.lastIndexOf(" ");
    return `${(space > max * 0.6 ? cut.slice(0, space) : cut).trimEnd()}…`;
}
