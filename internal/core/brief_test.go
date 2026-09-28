package core

import (
	"strings"
	"testing"
)

func TestBriefListsScopeAndProtectedAndCheckScopeInstruction(t *testing.T) {
	idx, _ := newModel(t)
	parent, err := idx.Create(CreateInput{Name: "Platform", Type: "boundary", Spec: "platform"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Create(CreateInput{
		Name:           "Payments",
		Type:           "service",
		ParentID:       parent,
		Implementation: []string{"src/payments/**"},
		Scope:          []string{"src/payments/api/**"},
		Spec:           "Pay things",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Create(CreateInput{
		Name:           "Billing",
		Type:           "service",
		ParentID:       parent,
		Implementation: []string{"src/billing/**"},
		Protected:      true,
		Spec:           "Bill things",
	}); err != nil {
		t.Fatal(err)
	}
	text, err := idx.Brief("payments")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "src/payments/api/**") {
		t.Fatalf("missing scope glob:\n%s", text)
	}
	if !strings.Contains(text, "billing") || !strings.Contains(text, "src/billing/**") {
		t.Fatalf("missing protected sibling:\n%s", text)
	}
	if !strings.Contains(text, "check_scope") {
		t.Fatalf("missing check_scope instruction:\n%s", text)
	}
	if strings.Contains(text, "src/billing/api") {
		t.Fatalf("should not expose sibling implementation detail")
	}
}

func TestBriefSiblingSpecIndent(t *testing.T) {
	idx, _ := newModel(t)
	parent, err := idx.Create(CreateInput{Name: "Platform", Type: "boundary"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Create(CreateInput{
		Name: "Pay", Type: "service", ParentID: parent,
		Implementation: []string{"src/payments/**"},
		Scope:          []string{"src/payments/api/**"},
		Spec:           "Pay spec",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Create(CreateInput{
		Name: "Bill", Type: "service", ParentID: parent,
		Spec: "Line one\nLine two",
	}); err != nil {
		t.Fatal(err)
	}
	text, err := idx.Brief("pay")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "    Line one\n    Line two") {
		t.Fatalf("expected uniform sibling spec indent:\n%s", text)
	}
}

func TestBriefUnknownID(t *testing.T) {
	idx, _ := newModel(t)
	_, err := idx.Brief("missing-node")
	if err == nil || !strings.Contains(err.Error(), "missing-node") {
		t.Fatalf("err %v", err)
	}
}
