import { describe, expect, it } from "vitest";
import {
    canExpandChild,
    expandedGroups,
    expandedKey,
    layoutSignature,
    layoutUsable,
    overBudgetHint,
    resolveExpanded,
    sanitizeViewModes,
    setMode,
    summaryText,
    toggleMode,
    type LevelChild,
} from "./level-view";

const defaults = { group_max: 8, level_budget: 24 };

function node(id: string, childCount = 0, extra: Partial<LevelChild> = {}): LevelChild {
    return { id, name: id, child_count: childCount, ...extra };
}

describe("resolveExpanded defaults", () => {
    it("opens small groups and leaves leaves alone", () => {
        const children = [node("a", 3), node("b"), node("c", 2)];
        expect(resolveExpanded(children, {}, defaults)).toEqual(["a", "c"]);
    });

    it("keeps big groups as summaries", () => {
        expect(resolveExpanded([node("a", 9), node("b", 8)], {}, defaults)).toEqual(["b"]);
    });

    it("stops opening once the projected count would pass the budget", () => {
        const children = [node("a", 8), node("b", 8), node("c", 8), node("d", 8)];
        // 4 visible + 8 = 12, + 8 = 20, next would be 28 > 24
        expect(resolveExpanded(children, {}, defaults)).toEqual(["a", "b"]);
    });

    it("is first come first served in response order", () => {
        const children = [node("z", 8), node("y", 8), node("x", 8)];
        expect(resolveExpanded(children, {}, { group_max: 8, level_budget: 12 })).toEqual(["z"]);
    });
});

describe("resolveExpanded architect modes", () => {
    it("group mode wins over group_max and budget", () => {
        const children = [node("a", 30), node("b", 2)];
        // forced a pushes the level to 32, so b no longer fits the budget
        expect(resolveExpanded(children, { a: "group" }, defaults)).toEqual(["a"]);
        expect(resolveExpanded(children, { a: "group", b: "group" }, defaults)).toEqual(["a", "b"]);
    });

    it("summary mode wins over defaults", () => {
        expect(resolveExpanded([node("a", 2), node("b", 2)], { a: "summary" }, defaults)).toEqual(["b"]);
    });

    it("a forced group counts against the budget for the others", () => {
        const children = [node("a", 20), node("b", 8)];
        // 2 + 20 = 22, b would make 30
        expect(resolveExpanded(children, { a: "group" }, defaults)).toEqual(["a"]);
    });

    it("drops a currently open group the architect collapsed, with its members", () => {
        const children = [
            node("a", 2, { expanded: true }),
            node("m1", 1, { group_id: "a" }),
            node("m2", 0, { group_id: "a" }),
            node("b", 2),
        ];
        expect(resolveExpanded(children, { a: "summary" }, defaults)).toEqual(["b"]);
    });
});

describe("resolveExpanded depth", () => {
    it("opens depth-2 candidates inside an open group", () => {
        const children = [node("a", 2, { expanded: true }), node("m1", 2, { group_id: "a" }), node("m2", 0, { group_id: "a" })];
        expect(resolveExpanded(children, {}, defaults)).toEqual(["a", "m1"]);
    });

    it("never requests depth 3", () => {
        const children = [
            node("a", 1, { expanded: true }),
            node("m", 1, { group_id: "a", expanded: true }),
            node("deep", 4, { group_id: "m" }),
        ];
        expect(resolveExpanded(children, { deep: "group" }, defaults)).toEqual(["a", "m"]);
    });

    it("ignores members of a group that is not open", () => {
        const children = [node("a", 20), node("orphan", 2, { group_id: "a" })];
        expect(resolveExpanded(children, {}, defaults)).toEqual([]);
    });

    it("is order independent in its output", () => {
        const one = resolveExpanded([node("b", 1), node("a", 1)], {}, defaults);
        const two = resolveExpanded([node("a", 1), node("b", 1)], {}, defaults);
        expect(one).toEqual(two);
        expect(one).toEqual(["a", "b"]);
    });
});

describe("overBudgetHint", () => {
    const groups = [
        { id: "a", name: "Alpha", members: 3 },
        { id: "b", name: "Beta", members: 9 },
        { id: "c", name: "Gamma", members: 6 },
        { id: "d", name: "Delta", members: 1 },
    ];

    it("is null within budget or with nothing to collapse", () => {
        expect(overBudgetHint(24, 24, groups)).toBeNull();
        expect(overBudgetHint(30, 24, [])).toBeNull();
    });

    it("names the largest groups first, at most three", () => {
        const hint = overBudgetHint(30, 24, groups);
        expect(hint?.largest.map((g) => g.id)).toEqual(["b", "c", "a"]);
        expect(hint?.message).toContain("Beta, Gamma, Alpha");
        expect(hint?.message).toContain("30");
    });

    it("breaks ties by name", () => {
        const hint = overBudgetHint(30, 24, [
            { id: "x", name: "Zed", members: 4 },
            { id: "y", name: "Abe", members: 4 },
        ]);
        expect(hint?.largest.map((g) => g.name)).toEqual(["Abe", "Zed"]);
    });
});

describe("expandedKey", () => {
    it("is empty for the empty set", () => {
        expect(expandedKey([])).toBe("");
    });

    it("is deterministic, order independent and short", () => {
        const key = expandedKey(["b", "a", "c"]);
        expect(key).toBe(expandedKey(["c", "b", "a"]));
        expect(key).toBe(expandedKey(["a", "b", "c", "a"]));
        expect(key).toMatch(/^[0-9a-f]{8}$/);
    });

    it("differs for different sets", () => {
        expect(expandedKey(["a"])).not.toBe(expandedKey(["b"]));
        expect(expandedKey(["a", "b"])).not.toBe(expandedKey(["ab"]));
    });

    it("matches a pinned FNV-1a value", () => {
        // fnv1a32("a") = 0xe40c292c
        expect(expandedKey(["a"])).toBe("e40c292c");
    });
});

describe("modes", () => {
    it("sanitizes stored junk", () => {
        expect(sanitizeViewModes({ a: "group", b: "nope", c: 1, d: "summary" })).toEqual({
            a: "group",
            d: "summary",
        });
        expect(sanitizeViewModes(null)).toEqual({});
        expect(sanitizeViewModes("x")).toEqual({});
    });

    it("setMode does not mutate", () => {
        const base = { a: "group" as const };
        expect(setMode(base, "b", "summary")).toEqual({ a: "group", b: "summary" });
        expect(base).toEqual({ a: "group" });
    });

    it("toggleMode flips the target and freezes the rest as shown", () => {
        const children = [node("a", 2, { expanded: true }), node("m", 0, { group_id: "a" }), node("b", 30), node("leaf")];
        const collapsed = toggleMode({}, children, "a");
        expect(collapsed).toEqual({ a: "summary", b: "summary" });
        const opened = toggleMode({}, children, "b");
        expect(opened).toEqual({ a: "group", b: "group" });
    });

    it("frozen modes keep other groups stable after a toggle", () => {
        const children = [node("a", 8, { expanded: true }), node("b", 8, { expanded: true })];
        const modes = toggleMode({}, children, "a");
        expect(resolveExpanded(children, modes, { group_max: 8, level_budget: 4 })).toEqual(["b"]);
    });
});

describe("expandedGroups", () => {
    it("counts members per open group", () => {
        const children = [node("a", 2, { expanded: true }), node("m1", 0, { group_id: "a" }), node("m2", 0, { group_id: "a" }), node("b", 2)];
        expect(expandedGroups(children)).toEqual([{ id: "a", name: "a", members: 2 }]);
    });
});

describe("summaryText", () => {
    it("lists the type mix", () => {
        expect(summaryText(5, [{ type: "service", count: 3 }, { type: "store", count: 2 }])).toBe("5 · 3 service, 2 store");
    });

    it("caps at three entries and counts the rest", () => {
        const mix = ["a", "b", "c", "d", "e"].map((type, i) => ({ type, count: 5 - i }));
        expect(summaryText(15, mix)).toBe("15 · 5 a, 4 b, 3 c +2");
    });

    it("falls back to the count", () => {
        expect(summaryText(4)).toBe("4");
        expect(summaryText(4, [])).toBe("4");
    });
});

describe("layout signature", () => {
    const before = [{ id: "a", group_id: "g1" }, { id: "b", group_id: "g2" }, { id: "g1" }, { id: "g2" }];
    const after = [{ id: "a", group_id: "g2" }, { id: "b", group_id: "g1" }, { id: "g1" }, { id: "g2" }];
    it("differs when the same ids move between groups and ignores order", () => {
        expect(layoutSignature(before)).not.toBe(layoutSignature(after));
        expect(layoutSignature(before)).toBe(layoutSignature([...before].reverse()));
    });
    it("only trusts a cache with a matching signature and id set", () => {
        const ids = ["a", "b"];
        const sig = layoutSignature(before);
        expect(layoutUsable(ids, ids, sig, sig)).toBe(true);
        expect(layoutUsable(ids, ids, null, sig)).toBe(false);
        expect(layoutUsable(ids, ids, sig, layoutSignature(after))).toBe(false);
        expect(layoutUsable(ids, ["a"], sig, sig)).toBe(false);
        expect(layoutUsable([], [], sig, sig)).toBe(false);
    });
});

describe("canExpandChild", () => {
    const kids: LevelChild[] = [
        { id: "top", child_count: 3, expanded: true },
        { id: "mid", child_count: 2, group_id: "top", expanded: true },
        { id: "deep", child_count: 2, group_id: "mid" },
        { id: "leaf" },
        { id: "closed", child_count: 4 },
        { id: "inner", child_count: 1, group_id: "top" },
    ];
    const find = (id: string) => kids.find((k) => k.id === id)!;
    it("offers expand for collapsed nodes within depth", () => {
        expect(canExpandChild(find("closed"), kids, false)).toBe(true);
        expect(canExpandChild(find("inner"), kids, false)).toBe(true);
    });
    it("hides expand when too deep, childless or on a relationship level", () => {
        expect(canExpandChild(find("deep"), kids, false)).toBe(false);
        expect(canExpandChild(find("leaf"), kids, false)).toBe(false);
        expect(canExpandChild(find("closed"), kids, true)).toBe(false);
    });
    it("keeps collapse for open groups", () => {
        expect(canExpandChild(find("mid"), kids, false)).toBe(true);
    });
});
