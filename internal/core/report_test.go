package core

import (
	"strings"
	"testing"
)

func TestFormatReportNamesNodeAndRule(t *testing.T) {
	text := FormatReport([]FileResult{
		{
			Path: "src/orders/create.ts",
			Kind: KindViolation,
			Hits: []Hit{{Rule: RuleOutsideScope, Node: "orders-service"}},
		},
		{
			Path: "src/billing/fee.ts",
			Kind: KindViolation,
			Hits: []Hit{
				{Rule: RuleProtected, Node: "billing"},
				{Rule: RuleOutsideScope, Node: "payments-service"},
			},
		},
		{Path: "src/payments/authorise.ts", Kind: KindAllowed},
	})
	for _, want := range []string{
		"src/orders/create.ts outside_scope orders-service",
		"src/billing/fee.ts protected billing",
		"src/billing/fee.ts outside_scope payments-service",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
	if strings.Contains(text, "authorise.ts") {
		t.Fatalf("printed an allowed path:\n%s", text)
	}
	if strings.Contains(text, "no assignment is active") {
		t.Fatalf("assignment copy on a violation report:\n%s", text)
	}
}

func TestFormatReportNoAssignment(t *testing.T) {
	text := FormatReport([]FileResult{
		{Path: "docs/notes.md", Kind: KindInformational},
	})
	if !strings.Contains(text, "no assignment is active") || !strings.Contains(text, "docs/notes.md informational") {
		t.Fatalf("report:\n%s", text)
	}
}
