import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const css = readFileSync(new URL("./app.css", import.meta.url), "utf8");

function block(selector: string): string {
    const start = css.indexOf(`${selector} {`);
    return css.slice(start, css.indexOf("}", start));
}

function token(scope: string, name: string): string {
    const match = block(scope).match(new RegExp(`--${name}:\\s*(#[0-9a-f]{6})`, "i"));
    if (!match) throw new Error(`missing --${name} in ${scope}`);
    return match[1];
}

function luminance(hex: string): number {
    const [r, g, b] = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255).map((v) =>
        v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4,
    );
    return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function ratio(a: string, b: string): number {
    const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
    return (hi + 0.05) / (lo + 0.05);
}

describe("edge stroke contrast (non-text UI, WCAG 1.4.11 needs 3:1)", () => {
    for (const [scope, label] of [
        [":root", "light"],
        [".dark", "dark"],
    ] as const) {
        it(`${label}: --edge is visible on the canvas and on both group fills`, () => {
            const edge = token(scope, "edge");
            expect(ratio(edge, token(scope, "background"))).toBeGreaterThanOrEqual(3);
            expect(ratio(edge, token(scope, "group"))).toBeGreaterThanOrEqual(3);
            expect(ratio(edge, token(scope, "group-2"))).toBeGreaterThanOrEqual(3);
        });

        it(`${label}: header text stays AA on both group fills`, () => {
            const fg = token(scope, "foreground");
            expect(ratio(fg, token(scope, "group"))).toBeGreaterThanOrEqual(4.5);
            expect(ratio(fg, token(scope, "group-2"))).toBeGreaterThanOrEqual(4.5);
        });
    }
});
