import { describe, expect, it } from "vitest";
import { isReviewError, normalizeVerify, reviewChanged, reviewSummary, validateVerify } from "./node-review";

describe("node review", () => {
    it("summarises", () => {
        expect(reviewSummary({})).toBe("none");
        expect(reviewSummary({ risk: "high", review: "required", verify: ["a", " ", "b"] })).toBe("risk high, review required, 2 commands");
        expect(reviewSummary({ review: "none", verify: ["a"] })).toBe("review not required, 1 command");
    });
    it("normalizes and detects change", () => {
        expect(normalizeVerify([" go test ", ""])).toEqual(["go test"]);
        expect(reviewChanged({ verify: ["a", ""] }, { verify: ["a"] })).toBe(false);
        expect(reviewChanged({ risk: "low" }, {})).toBe(true);
    });
    it("validates limits", () => {
        expect(validateVerify(["ok"])).toEqual([]);
        expect(validateVerify(Array(9).fill("x"))).toHaveLength(1);
        expect(validateVerify(["x".repeat(301)])).toHaveLength(1);
    });
    it("routes errors", () => {
        expect(isReviewError("invalid contract: at most 8 verify commands")).toBe(true);
        expect(isReviewError("name required")).toBe(false);
    });
});

import { comparable, resolveSaveErrorTarget, toPatch } from "./node-review";
import type { NodeDetail } from "./api";

describe("review error routing", () => {
    const goMessages = [
        'invalid contract: node risk "x" (valid: low, medium, high)',
        'invalid contract: node review "x" (valid: none, required)',
        "invalid contract: at most 8 verify commands",
        "invalid contract: a verify command exceeds 300 characters",
    ];
    it("routes the real Go messages to review, ahead of contract", () => {
        for (const m of goMessages) {
            expect(isReviewError(m)).toBe(true);
            expect(resolveSaveErrorTarget(m)).toBe("review");
        }
    });
    it("does not match node ids that contain review", () => {
        expect(isReviewError('node "code-review" not found')).toBe(false);
        expect(resolveSaveErrorTarget("invalid contract: payloads too long")).toBe("contract");
        expect(resolveSaveErrorTarget("dependency cycle: a -> b")).toBe("depends");
    });
});

describe("toPatch review fields", () => {
    const base = {
        id: "a", name: "A", type: "service", status: "draft", implementation: [], scope: [], protected: false, markdown: "",
    } as unknown as NodeDetail;

    it("sends only the review fields that changed", () => {
        const next = { ...base, risk: "high" } as NodeDetail;
        const patch = toPatch(next, false, base);
        expect(patch.risk).toBe("high");
        expect("review" in patch).toBe(false);
        expect("verify" in patch).toBe(false);
        const v = toPatch({ ...base, verify: [" go test ", ""] } as NodeDetail, false, base);
        expect(v.verify).toEqual(["go test"]);
        expect("risk" in v).toBe(false);
        expect("review" in v).toBe(false);
    });

    it("sends none when nothing changed", () => {
        const patch = toPatch({ ...base, verify: ["  "] } as NodeDetail, false, base);
        expect("risk" in patch || "review" in patch || "verify" in patch).toBe(false);
    });

    it("builds the undo patch from only what the save sent", () => {
        const before = { ...base, risk: "low", verify: ["a"] } as NodeDetail;
        const after = { ...before, risk: "high" } as NodeDetail;
        const patch = toPatch(after, false, before);
        const undo = toPatch(before, true, after, patch);
        expect(undo.risk).toBe("low");
        expect("review" in undo).toBe(false);
        expect("verify" in undo).toBe(false);
    });

    it("comparable ignores blank verify rows", () => {
        expect(comparable({ ...base, verify: [""] } as NodeDetail)).toEqual(comparable(base));
    });
});
