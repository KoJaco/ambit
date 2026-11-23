package core

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDriftReportsUnownedUnmatchedAndOverlap(t *testing.T) {
	dir := commitTwoNodeRepo(t)
	idx := reopen(t, dir)
	// Overlap with orders-service (non-nested), plus a dead glob.
	if _, err := idx.Create(CreateInput{
		Name: "Order Audit", Type: "service",
		Implementation: []string{"src/orders/**", "src/audit/**"},
	}); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "audit")
	idx = reopen(t, dir)

	r, err := Drift(idx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(r.Unowned, ",") != ".gitignore,docs/notes.md" {
		t.Fatalf("unowned %v", r.Unowned)
	}
	if len(r.Unmatched) != 2 {
		// src/payments/** has no files; src/audit/** has no files.
		t.Fatalf("unmatched %+v", r.Unmatched)
	}
	if len(r.Overlaps) != 1 || r.Overlaps[0].Path != "src/orders/create.ts" || len(r.Overlaps[0].Owners) != 2 {
		t.Fatalf("overlaps %+v", r.Overlaps)
	}
	if r.Findings() != 5 || !strings.Contains(FormatDrift(r), "docs/notes.md") {
		t.Fatalf("findings %d\n%s", r.Findings(), FormatDrift(r))
	}
}

func TestDriftNestedOwnershipIsNotOverlap(t *testing.T) {
	dir := commitTwoNodeRepo(t)
	idx := reopen(t, dir)
	if _, err := idx.Create(CreateInput{
		Name: "Order Create", Type: "module", ParentID: "orders-service",
		Implementation: []string{"src/orders/create.ts"},
	}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "docs", "more.md"), "x\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "nested")
	idx = reopen(t, dir)
	r, err := Drift(idx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Overlaps) != 0 {
		t.Fatalf("overlaps %+v", r.Overlaps)
	}
}

func TestDriftIgnoresArchAndRequiresGit(t *testing.T) {
	dir := commitTwoNodeRepo(t)
	idx := reopen(t, dir)
	r, err := Drift(idx, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range r.Unowned {
		if strings.HasPrefix(p, ".arch/") {
			t.Fatalf("model file reported: %s", p)
		}
	}
	if _, err := Drift(idx, t.TempDir()); err == nil {
		t.Fatal("expected git error outside a repository")
	}
}
