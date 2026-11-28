import { describe, expect, it } from "vitest";
import type { Level, Part } from "./api";
import { filterRoots, parseGlobs, partSummary, resolveSelectedPart, scopeRootLevel, slugifyPartId, validatePartForm } from "./parts-scope";

const sum = (id: string) => ({ id, name: id, type: "", status: "", protected: false });
const part: Part = { id: "api", name: "API", include: ["api/**"], root_node_ids: ["a", "b"] };

describe("parseGlobs", () => {
    it("trims, drops blanks and duplicates", () => {
        expect(parseGlobs(" api/** \r\n\nweb/**\napi/**\n")).toEqual(["api/**", "web/**"]);
    });
});

describe("filterRoots", () => {
    const roots = [sum("a"), sum("b"), sum("c")];
    it("returns all for no part and the part's roots otherwise", () => {
        expect(filterRoots(roots, null)).toHaveLength(3);
        expect(filterRoots(roots, part).map((r) => r.id)).toEqual(["a", "b"]);
    });
});

describe("scopeRootLevel", () => {
    const level: Level = {
        node: null,
        children: [sum("a"), sum("b"), sum("c")],
        relationships: [
            { id: "1", from: "a", to: "b", label: "", kind: "" },
            { id: "2", from: "a", to: "c", label: "", kind: "" },
        ],
        crossings: [],
        warnings: [],
    };
    it("drops edges to removed nodes", () => {
        const out = scopeRootLevel(level, part);
        expect(out.children.map((c) => c.id)).toEqual(["a", "b"]);
        expect(out.relationships.map((r) => r.id)).toEqual(["1"]);
    });
    it("keeps the descendants of a part root that is open, with the edges among them", () => {
        const open: Level = {
            node: null,
            children: [
                { ...sum("demo-app"), expanded: true, child_count: 2 },
                { ...sum("web"), group_id: "demo-app" },
                { ...sum("db"), group_id: "demo-app" },
                { ...sum("other"), expanded: true, child_count: 1 },
                { ...sum("other-kid"), group_id: "other" },
            ],
            relationships: [
                { id: "1", from: "web", to: "db", label: "", kind: "" },
                { id: "2", from: "web", to: "other-kid", label: "", kind: "" },
            ],
            crossings: [{ node_id: "db" }, { node_id: "other-kid" }] as Level["crossings"],
            warnings: [],
        };
        const out = scopeRootLevel(open, { root_node_ids: ["demo-app"] });
        expect(out.children.map((c) => c.id)).toEqual(["demo-app", "web", "db"]);
        expect(out.relationships.map((r) => r.id)).toEqual(["1"]);
        expect(out.crossings.map((c) => c.node_id)).toEqual(["db"]);
    });

    it("keeps depth-2 members through the ancestor chain", () => {
        const deep: Level = {
            node: null,
            children: [{ ...sum("r"), expanded: true }, { ...sum("g"), group_id: "r", expanded: true }, { ...sum("leaf"), group_id: "g" }, sum("x")],
            relationships: [],
            crossings: [],
            warnings: [],
        };
        expect(scopeRootLevel(deep, { root_node_ids: ["r"] }).children.map((c) => c.id)).toEqual(["r", "g", "leaf"]);
    });

    it("leaves All and non-root levels alone", () => {
        expect(scopeRootLevel(level, null)).toBe(level);
        const inner = { ...level, node: sum("a") };
        expect(scopeRootLevel(inner, part)).toBe(inner);
    });
});

describe("resolveSelectedPart", () => {
    it("falls back to All for unknown ids", () => {
        expect(resolveSelectedPart("api", [part])).toBe(part);
        expect(resolveSelectedPart("gone", [part])).toBeNull();
        expect(resolveSelectedPart(null, [part])).toBeNull();
    });
});

describe("validatePartForm", () => {
    it("requires a name and derives the id", () => {
        expect(validatePartForm({ name: " ", includeText: "", rootIds: [] }, [])).toEqual({ ok: false, errors: { name: "Enter a name." } });
        const r = validatePartForm({ name: "Billing API", includeText: "billing/**\n", rootIds: ["x", "x"] }, []);
        expect(r).toEqual({ ok: true, part: { id: "billing-api", name: "Billing API", include: ["billing/**"], root_node_ids: ["x"] } });
        expect(slugifyPartId("  A/B  c ")).toBe("a-b-c");
    });
    it("rejects duplicates on create but not edit", () => {
        expect(validatePartForm({ name: "API", includeText: "", rootIds: [] }, [part]).ok).toBe(false);
        expect(validatePartForm({ id: "api", name: "API", includeText: "", rootIds: [] }, [part]).ok).toBe(true);
    });
    it("rejects absolute and parent globs", () => {
        expect(validatePartForm({ name: "x", includeText: "/etc/**", rootIds: [] }, []).ok).toBe(false);
        expect(validatePartForm({ name: "x", includeText: "../x/**", rootIds: [] }, []).ok).toBe(false);
    });
});

describe("partSummary", () => {
    it("pluralizes", () => {
        expect(partSummary({ ...part, include: ["a"], root_node_ids: ["a"] })).toBe("1 glob, 1 root");
        expect(partSummary(part)).toBe("1 glob, 2 roots");
    });
});
