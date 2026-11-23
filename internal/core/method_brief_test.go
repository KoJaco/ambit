package core

import (
	"os"
	"strings"
	"testing"
)

// freeformBriefFixture builds a deterministic model and returns the brief for "payments".
func freeformBriefFixture(t *testing.T) (*Index, string) {
	t.Helper()
	idx, _ := newModel(t)
	if err := idx.SetConstraints([]Constraint{{ID: "no-orm", Text: "Use plain SQL.", Severity: "must"}}); err != nil {
		t.Fatal(err)
	}
	parent, err := idx.Create(CreateInput{Name: "Platform", Type: "boundary", Spec: "platform"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Create(CreateInput{Name: "Billing", Type: "service", ParentID: parent, Implementation: []string{"src/billing/**"}, Protected: true, Spec: "Bill things"}); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Create(CreateInput{Name: "Payments", Type: "service", ParentID: parent, Implementation: []string{"src/payments/**"}, DependsOn: []NodeID{"billing"}, Spec: "Pay things"}); err != nil {
		t.Fatal(err)
	}
	text, err := idx.Brief("payments")
	if err != nil {
		t.Fatal(err)
	}
	return idx, text
}

// The golden file was produced by the Brief implementation before methods existed.
func TestFreeformBriefByteIdentical(t *testing.T) {
	_, text := freeformBriefFixture(t)
	want, err := os.ReadFile("testdata/brief_freeform.golden")
	if err != nil {
		t.Fatal(err)
	}
	if text != string(want) {
		t.Fatalf("freeform brief changed:\n%s", text)
	}
}

func TestBriefPreambles(t *testing.T) {
	idx, base := freeformBriefFixture(t)
	if _, err := idx.SetMethod("design-first"); err != nil {
		t.Fatal(err)
	}
	got, err := idx.Brief("payments")
	if err != nil {
		t.Fatal(err)
	}
	m := idx.ActiveMethod()
	want := m.Stages[0].Preamble + "\n\n" + base
	if got != want {
		t.Fatalf("design stage brief:\n%s", got)
	}
	if err := idx.SetStage("build"); err != nil {
		t.Fatal(err)
	}
	got, _ = idx.Brief("payments")
	if !strings.HasPrefix(got, m.Stages[1].Preamble+"\n\n") {
		t.Fatalf("build stage brief:\n%s", got)
	}
}
