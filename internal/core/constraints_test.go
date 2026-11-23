package core

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestConstraintsRoundTripPreservesConfigKeys(t *testing.T) {
	idx, dir := newModel(t)
	cfg := filepath.Join(dir, ".arch", "config.json")
	writeFile(t, cfg, `{"group_max": 5, "future_key": {"a": [1, 2]}}`)
	idx = reopen(t, dir)
	if len(idx.Constraints) != 0 {
		t.Fatal("unexpected constraints")
	}
	in := []Constraint{
		{ID: "scope-user", Text: "Every query scopes by user_id", Severity: "must"},
		{ID: "no-secrets", Text: "  No secrets in logs  "},
		{ID: "prefer-small", Text: "Prefer small functions", Severity: "should"},
	}
	if err := idx.SetConstraints(in); err != nil {
		t.Fatal(err)
	}
	if len(idx.Constraints) != 3 || idx.Constraints[1].Severity != "must" || idx.Constraints[1].Text != "No secrets in logs" {
		t.Fatalf("%+v", idx.Constraints)
	}
	got := reopen(t, dir)
	if len(got.Constraints) != 3 || got.Constraints[0].ID != "scope-user" || got.Constraints[2].Severity != "should" {
		t.Fatalf("%+v", got.Constraints)
	}
	if got.GroupMax != 5 {
		t.Fatalf("group_max %d", got.GroupMax)
	}
	raw := readFile(t, cfg)
	if !strings.Contains(raw, `"future_key"`) || !strings.Contains(raw, `"group_max"`) {
		t.Fatalf("keys lost: %s", raw)
	}
	// Replace semantics.
	if err := idx.SetConstraints([]Constraint{{ID: "only", Text: "one"}}); err != nil {
		t.Fatal(err)
	}
	if got := reopen(t, dir); len(got.Constraints) != 1 || got.Constraints[0].ID != "only" {
		t.Fatalf("%+v", got.Constraints)
	}
	// Empty removes the key but keeps the others.
	if err := idx.SetConstraints(nil); err != nil {
		t.Fatal(err)
	}
	raw = readFile(t, cfg)
	if strings.Contains(raw, "constraints") || !strings.Contains(raw, "future_key") {
		t.Fatalf("%s", raw)
	}
}

func TestSetConstraintsRejectsInvalid(t *testing.T) {
	idx, dir := newModel(t)
	before := readFile(t, filepath.Join(dir, ".arch", "config.json"))
	many := make([]Constraint, MaxConstraints+1)
	for i := range many {
		many[i] = Constraint{ID: "c" + strings.Repeat("x", i+1), Text: "t"}
	}
	cases := map[string][]Constraint{
		"bad slug":     {{ID: "Bad Id", Text: "t"}},
		"empty id":     {{Text: "t"}},
		"empty text":   {{ID: "a", Text: "  "}},
		"long text":    {{ID: "a", Text: strings.Repeat("x", MaxConstraintText+1)}},
		"bad severity": {{ID: "a", Text: "t", Severity: "maybe"}},
		"dup":          {{ID: "a", Text: "t"}, {ID: "a", Text: "u"}},
		"too many":     many,
	}
	for name, in := range cases {
		if err := idx.SetConstraints(in); !errors.Is(err, ErrBadConstraint) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if readFile(t, filepath.Join(dir, ".arch", "config.json")) != before {
		t.Fatal("config changed by rejected writes")
	}
	// Exactly at the limits is fine.
	ok := many[:MaxConstraints]
	ok[0].Text = strings.Repeat("x", MaxConstraintText)
	if err := idx.SetConstraints(ok); err != nil {
		t.Fatal(err)
	}
}

func TestConstraintsInvalidEntriesSkippedOnLoad(t *testing.T) {
	_, dir := newModel(t)
	writeFile(t, filepath.Join(dir, ".arch", "config.json"), `{"constraints": [
		{"id":"good","text":"ok"},
		{"id":"Bad Slug","text":"x"},
		{"id":"empty","text":""},
		{"id":"sev","text":"x","severity":"nope"},
		{"id":"good","text":"duplicate"},
		"not an object",
		{"id":"typed","text":5},
		{"id":"later","text":"fine","severity":"should"}
	]}`)
	idx := reopen(t, dir)
	if len(idx.Constraints) != 2 || idx.Constraints[0].ID != "good" || idx.Constraints[0].Severity != "must" || idx.Constraints[1].ID != "later" {
		t.Fatalf("%+v", idx.Constraints)
	}
	warns := 0
	for _, d := range idx.Diagnostics {
		if d.Severity == SeverityWarning && strings.Contains(d.Message, "constraints[") {
			warns++
		}
	}
	if warns != 6 {
		t.Fatalf("warnings %d: %v", warns, idx.Diagnostics)
	}
	// Whole-value wrong type: ignored with a warning, not a load error.
	writeFile(t, filepath.Join(dir, ".arch", "config.json"), `{"constraints": "nope"}`)
	idx = reopen(t, dir)
	if len(idx.Constraints) != 0 || len(idx.Diagnostics) == 0 {
		t.Fatalf("%+v %v", idx.Constraints, idx.Diagnostics)
	}
}

func TestConstraintsOverLimitToleratedOnLoad(t *testing.T) {
	_, dir := newModel(t)
	var b strings.Builder
	b.WriteString(`{"constraints":[`)
	for i := 0; i < MaxConstraints+2; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"id":"c` + strings.Repeat("x", i+1) + `","text":"t"}`)
	}
	b.WriteString(`]}`)
	writeFile(t, filepath.Join(dir, ".arch", "config.json"), b.String())
	idx := reopen(t, dir)
	if len(idx.Constraints) != MaxConstraints+2 {
		t.Fatalf("got %d", len(idx.Constraints))
	}
}
