import { describe, expect, it } from "vitest";
import { groupNodesByType } from "./node-groups";

describe("groupNodesByType", () => {
    it("groups alphabetically with untyped last and sorts names inside", () => {
        const groups = groupNodesByType([
            { name: "Zed", type: "service" },
            { name: "loose", type: "" },
            { name: "Api", type: "service" },
            { name: "Db", type: "data store" },
            { name: "blank", type: "  " },
        ]);
        expect(groups.map((g) => g.label)).toEqual(["data store", "service", "untyped"]);
        expect(groups[1].items.map((i) => i.name)).toEqual(["Api", "Zed"]);
        expect(groups[2].items.map((i) => i.name)).toEqual(["blank", "loose"]);
        expect(groups[2].type).toBe("");
    });

    it("returns no groups for an empty list", () => {
        expect(groupNodesByType([])).toEqual([]);
    });
});
