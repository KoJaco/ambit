import { describe, expect, it } from "vitest";
import { firstEnabled, listKeyAction, lastEnabled, moveActive, typeaheadIndex, type SelectOption } from "./select-nav";

const opts: SelectOption[] = [
    { value: "a", label: "Alpha", disabled: true },
    { value: "b", label: "Beta" },
    { value: "c", label: "Bravo" },
    { value: "d", label: "Delta" },
];

describe("select navigation", () => {
    it("finds first and last enabled", () => {
        expect(firstEnabled(opts)).toBe(1);
        expect(lastEnabled(opts)).toBe(3);
        expect(firstEnabled([])).toBe(-1);
    });
    it("moves skipping disabled and clamps at the ends", () => {
        expect(moveActive(opts, 1, 1)).toBe(2);
        expect(moveActive(opts, 1, -1)).toBe(1);
        expect(moveActive(opts, 3, 1)).toBe(3);
    });
    it("typeahead matches prefixes and cycles on repeated letters", () => {
        expect(typeaheadIndex(opts, "d", 1)).toBe(3);
        expect(typeaheadIndex(opts, "b", 1)).toBe(2);
        expect(typeaheadIndex(opts, "bb", 1)).toBe(2);
        expect(typeaheadIndex(opts, "br", 1)).toBe(2);
        expect(typeaheadIndex(opts, "al", 1)).toBe(-1);
        expect(typeaheadIndex(opts, "", 1)).toBe(-1);
    });
    it("list keys all stop propagation; handled keys and typeahead preventDefault", () => {
        for (const k of ["ArrowDown", "ArrowUp", "Home", "End", "Enter", " ", "Escape", "Tab", "a", "Delete", "z"]) {
            expect(listKeyAction(k).stopPropagation).toBe(true);
        }
        expect(listKeyAction("z", { ctrlKey: true })).toMatchObject({ kind: "none", stopPropagation: true, preventDefault: false });
        expect(listKeyAction("a")).toMatchObject({ kind: "type", preventDefault: true });
        expect(listKeyAction("Delete").kind).toBe("none");
        expect(listKeyAction("Tab").kind).toBe("tab");
        expect(listKeyAction("Escape").kind).toBe("close");
    });
    it("space chooses unless a search is active", () => {
        expect(listKeyAction(" ").kind).toBe("choose");
        expect(listKeyAction(" ", { searching: true }).kind).toBe("type");
    });
});
