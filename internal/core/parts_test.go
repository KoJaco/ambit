package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".arch", "config.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func warningsText(idx *Index) string {
	var b strings.Builder
	for _, w := range idx.Warnings() {
		b.WriteString(w.String() + "\n")
	}
	return b.String()
}

func TestScopeDefaultsToWhole(t *testing.T) {
	idx, _ := newModel(t)
	if idx.Scope != ScopeWhole || len(idx.Parts) != 0 {
		t.Fatalf("%q %v", idx.Scope, idx.Parts)
	}
	if got := idx.ScopeInfo(); got.Scope != ScopeWhole {
		t.Fatal(got)
	}
}

func TestPartsLoadValidationAndWarnings(t *testing.T) {
	_, dir := newModel(t)
	writeConfig(t, dir, `{
  "scope": "partial",
  "parts": [
    {"id": "billing", "name": "Billing", "include": ["internal/billing/**"], "root_node_ids": ["ghost"]},
    {"id": "billing", "name": "dup", "include": []},
    {"id": "Bad Id", "include": []},
    {"id": "badglob", "include": ["[unclosed"]},
    "nope"
  ]
}`)
	idx := reopen(t, dir)
	if idx.Scope != ScopePartial || len(idx.Parts) != 1 || idx.Parts[0].ID != "billing" {
		t.Fatalf("%q %+v", idx.Scope, idx.Parts)
	}
	w := warningsText(idx)
	for _, want := range []string{`root node "ghost" does not exist`, "duplicate id", "invalid glob", "parts[2] skipped", "parts[4] skipped"} {
		if !strings.Contains(w, want) {
			t.Fatalf("missing warning %q in:\n%s", want, w)
		}
	}
	writeConfig(t, dir, `{"scope": "sideways"}`)
	idx = reopen(t, dir)
	if idx.Scope != ScopeWhole || !strings.Contains(warningsText(idx), "scope") {
		t.Fatalf("%q %s", idx.Scope, warningsText(idx))
	}
}

func TestPartBounds(t *testing.T) {
	tooMany := make([]string, MaxPartGlobs+1)
	for i := range tooMany {
		tooMany[i] = "a/**"
	}
	if _, err := normalizePart(Part{ID: "x", Include: tooMany}); !errors.Is(err, ErrBadPart) {
		t.Fatalf("globs: %v", err)
	}
	if _, err := normalizePart(Part{ID: "x", Include: []string{strings.Repeat("a", MaxPartGlobLen+1)}}); !errors.Is(err, ErrBadPart) {
		t.Fatalf("glob length: %v", err)
	}
	if _, err := normalizePart(Part{ID: "X Y"}); !errors.Is(err, ErrBadPart) {
		t.Fatal("id")
	}
	if _, err := normalizePart(Part{ID: "x", RootNodeIDs: []NodeID{"Not Slug"}}); !errors.Is(err, ErrBadPart) {
		t.Fatal("root")
	}
	p, err := normalizePart(Part{ID: "x", Include: []string{"a/**", " a/** ", ""}, RootNodeIDs: []NodeID{"r", "r"}})
	if err != nil || len(p.Include) != 1 || len(p.RootNodeIDs) != 1 || p.Name != "x" {
		t.Fatalf("%+v %v", p, err)
	}
	idx, _ := newModel(t)
	for i := 0; i < MaxParts; i++ {
		if _, err := idx.SetPart(Part{ID: "p" + string(rune('a'+i))}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := idx.SetPart(Part{ID: "overflow"}); !errors.Is(err, ErrBadPart) {
		t.Fatalf("max parts: %v", err)
	}
}

func TestSetPartDeletePartRoundTrip(t *testing.T) {
	idx, dir := newModel(t)
	writeConfig(t, dir, `{"stage":"build","unknown_key":{"x":1}}`)
	idx = reopen(t, dir)
	if err := idx.SetScope(ScopePartial); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.SetPart(Part{ID: "api", Name: "API", Include: []string{"api/**"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.SetPart(Part{ID: "api", Name: "API v2", Include: []string{"api/**", "cmd/api/**"}}); err != nil {
		t.Fatal(err)
	}
	idx = reopen(t, dir)
	if idx.Scope != ScopePartial || len(idx.Parts) != 1 || idx.Parts[0].Name != "API v2" || len(idx.Parts[0].Include) != 2 {
		t.Fatalf("%+v", idx.Parts)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".arch", "config.json"))
	if !strings.Contains(string(data), "unknown_key") || !strings.Contains(string(data), `"stage"`) {
		t.Fatalf("other keys lost:\n%s", data)
	}
	if err := idx.DeletePart("missing"); !errors.Is(err, ErrBadPart) {
		t.Fatalf("delete missing: %v", err)
	}
	if err := idx.DeletePart("api"); err != nil {
		t.Fatal(err)
	}
	if err := idx.SetScope(ScopeWhole); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(dir, ".arch", "config.json"))
	if strings.Contains(string(data), `"parts"`) || strings.Contains(string(data), `"scope"`) {
		t.Fatalf("keys not removed:\n%s", data)
	}
	if err := idx.SetScope("nope"); !errors.Is(err, ErrBadPart) {
		t.Fatal(err)
	}
}

// partialRepo has src/orders owned by orders-service, an unowned file inside the billing
// part, and unowned files outside every part.
func partialRepo(t *testing.T) (string, *Index) {
	t.Helper()
	dir := commitTwoNodeRepo(t)
	writeFile(t, filepath.Join(dir, "src", "billing", "fee.ts"), "x\n")
	writeFile(t, filepath.Join(dir, "scripts", "tool.sh"), "x\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "more")
	idx := reopen(t, dir)
	if err := idx.SetScope(ScopePartial); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.SetPart(Part{ID: "billing", Name: "Billing", Include: []string{"src/billing/**"}}); err != nil {
		t.Fatal(err)
	}
	return dir, idx
}

func TestPartialDriftReportsOnlyInsideParts(t *testing.T) {
	dir, idx := partialRepo(t)
	r, err := Drift(idx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(r.Unowned, ",") != "src/billing/fee.ts" {
		t.Fatalf("unowned %v", r.Unowned)
	}
	// docs/notes.md and scripts/tool.sh are outside every part.
	if r.OutsideParts != 2 {
		t.Fatalf("outside_parts %d", r.OutsideParts)
	}
	partialIgnored := r.Ignored
	// Owned, overlap and unmatched-glob checks are unchanged: payments has no files.
	if len(r.Unmatched) != 1 || r.Unmatched[0].Glob != "src/payments/**" {
		t.Fatalf("unmatched %+v", r.Unmatched)
	}
	if !strings.Contains(FormatDrift(r), "outside the model's parts") {
		t.Fatalf("format:\n%s", FormatDrift(r))
	}

	// Whole scope reports all three unowned files.
	if err := idx.SetScope(ScopeWhole); err != nil {
		t.Fatal(err)
	}
	r, _ = Drift(idx, dir)
	if len(r.Unowned) != 3 || r.OutsideParts != 0 {
		t.Fatalf("whole: %v %d", r.Unowned, r.OutsideParts)
	}
	// ignored_files counts drift_ignore matches only; partial scope does not inflate it.
	if r.Ignored != partialIgnored {
		t.Fatalf("ignored_files %d (partial) vs %d (whole)", partialIgnored, r.Ignored)
	}
}

func TestPartialDriftWithNoPartsReportsNothingUnowned(t *testing.T) {
	dir := commitTwoNodeRepo(t)
	idx := reopen(t, dir)
	if err := idx.SetScope(ScopePartial); err != nil {
		t.Fatal(err)
	}
	r, err := Drift(idx, dir)
	if err != nil || len(r.Unowned) != 0 || r.OutsideParts != 1 {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestDriftPartOptionRestrictsToOnePart(t *testing.T) {
	dir, idx := partialRepo(t)
	if _, err := idx.SetPart(Part{ID: "scripts", Include: []string{"scripts/**"}}); err != nil {
		t.Fatal(err)
	}
	all, _ := DriftWith(idx, DriftOptions{Root: dir})
	if len(all.Unowned) != 2 {
		t.Fatalf("%v", all.Unowned)
	}
	one, err := DriftWith(idx, DriftOptions{Root: dir, Part: "scripts"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(one.Unowned, ",") != "scripts/tool.sh" || one.OutsideParts != 2 {
		t.Fatalf("%v %d", one.Unowned, one.OutsideParts)
	}
	if _, err := DriftWith(idx, DriftOptions{Root: dir, Part: "nope"}); err == nil {
		t.Fatal("unknown part should fail")
	}
	// check_model passes the part through.
	on := true
	res, err := CheckModel(idx, CheckModelOptions{IncludeDrift: &on, Part: "billing"})
	if err != nil || res.Drift == nil || strings.Join(res.Drift.Unowned, ",") != "src/billing/fee.ts" {
		t.Fatalf("%+v %v", res.Drift, err)
	}
}

func TestPartialScopeKeepsOwnedFilesVisible(t *testing.T) {
	dir, idx := partialRepo(t)
	// A node owning a file outside every part still has it checked (overlap) and its globs
	// still count as matched.
	if _, err := idx.Create(CreateInput{Name: "Tool", Type: "module", Implementation: []string{"scripts/**"}}); err != nil {
		t.Fatal(err)
	}
	r, _ := Drift(idx, dir)
	for _, u := range r.Unmatched {
		if u.Glob == "scripts/**" {
			t.Fatalf("scripts glob reported unmatched: %+v", r.Unmatched)
		}
	}
	if r.OutsideParts != 1 {
		t.Fatalf("outside %d (only docs/notes.md)", r.OutsideParts)
	}
}

func TestGraphViewPartScopesRoots(t *testing.T) {
	idx, _ := newModel(t)
	mk := func(in CreateInput) NodeID {
		id, err := idx.Create(in)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	a := mk(CreateInput{Name: "A", Type: "service"})
	a1 := mk(CreateInput{Name: "A One", Type: "module", ParentID: a})
	b := mk(CreateInput{Name: "B", Type: "service"})
	if _, err := idx.SetRelationship(SetRelationshipInput{From: a1, To: b, Label: "calls", Kind: KindSync}); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.SetPart(Part{ID: "pa", RootNodeIDs: []NodeID{a, "ghost"}}); err != nil {
		t.Fatal(err)
	}
	v, err := idx.GraphViewPart(false, "pa")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Nodes) != 2 || len(v.Edges) != 0 {
		t.Fatalf("%+v", v)
	}
	full, _ := idx.GraphViewPart(false, "")
	if len(full.Nodes) != 3 || len(full.Edges) != 1 {
		t.Fatalf("%+v", full)
	}
	if _, err := idx.GraphViewPart(false, "nope"); !errors.Is(err, ErrBadPart) {
		t.Fatal(err)
	}
}

func TestBriefMentionsPartialScopeOnlyWhenPartial(t *testing.T) {
	idx, _ := newModel(t)
	id, err := idx.Create(CreateInput{Name: "A", Type: "service", Implementation: []string{"a/**"}})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := idx.Brief(id)
	if strings.Contains(b, PartialScopeGuidance) {
		t.Fatal("whole scope brief must not mention parts")
	}
	if err := idx.SetScope(ScopePartial); err != nil {
		t.Fatal(err)
	}
	b, _ = idx.Brief(id)
	if !strings.Contains(b, PartialScopeGuidance) {
		t.Fatal("partial scope brief should mention parts")
	}
}
