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
	if strings.Join(r.Unowned, ",") != "docs/notes.md" {
		t.Fatalf("unowned %v", r.Unowned)
	}
	if len(r.Unmatched) != 2 {
		// src/payments/** has no files; src/audit/** has no files.
		t.Fatalf("unmatched %+v", r.Unmatched)
	}
	if len(r.Overlaps) != 1 || r.Overlaps[0].Path != "src/orders/create.ts" || len(r.Overlaps[0].Owners) != 2 {
		t.Fatalf("overlaps %+v", r.Overlaps)
	}
	if r.Findings() != 4 || !strings.Contains(FormatDrift(r), "docs/notes.md") {
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

func TestTrackedFilesKeepsNonASCIIPaths(t *testing.T) {
	dir := t.TempDir()
	gitInitRepo(t, dir)
	writeFile(t, filepath.Join(dir, "docs", "café.md"), "x\n")
	gitCmd(t, dir, "add", "-A")
	files, err := TrackedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "docs/café.md" {
		t.Fatalf("files %q", files)
	}
}

func TestDriftIgnoreDefaultsAndConfig(t *testing.T) {
	dir := commitTwoNodeRepo(t)
	writeFile(t, filepath.Join(dir, "go.mod"), "module x\n")
	writeFile(t, filepath.Join(dir, "web", "tsconfig.app.json"), "{}\n")
	writeFile(t, filepath.Join(dir, "tools", "gen.sh"), "#!/bin/sh\n")
	idx := reopen(t, dir)
	r, err := Drift(idx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(r.Unowned, ",") != "docs/notes.md,tools/gen.sh" || r.Ignored < 3 {
		t.Fatalf("unowned %v ignored %d", r.Unowned, r.Ignored)
	}
	if err := idx.SetConfigKeys(map[string]any{"drift_ignore": []string{"tools/**", "docs/**"}}); err != nil {
		t.Fatal(err)
	}
	r, err = Drift(idx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Unowned) != 0 {
		t.Fatalf("unowned after drift_ignore: %v", r.Unowned)
	}
}

func TestDriftCountsUntrackedFilesButNotGitignored(t *testing.T) {
	dir := commitTwoNodeRepo(t)
	writeFile(t, filepath.Join(dir, "src", "orders", "new.ts"), "x\n")
	writeFile(t, filepath.Join(dir, "scratch.txt"), "x\n")
	writeFile(t, filepath.Join(dir, ".gitignore"), "scratch.txt\n")
	idx := reopen(t, dir)
	r, err := Drift(idx, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range r.Unowned {
		if p == "scratch.txt" {
			t.Fatalf("gitignored file reported: %v", r.Unowned)
		}
	}
	// New file under orders-service is owned, so the node's glob is satisfied
	// without committing it.
	r, err = DriftWith(idx, DriftOptions{Root: dir, NodeID: "orders-service"})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Unmatched) != 0 {
		t.Fatalf("unmatched %+v", r.Unmatched)
	}
}

func TestDriftNodeFilter(t *testing.T) {
	dir := commitTwoNodeRepo(t)
	idx := reopen(t, dir)
	r, err := DriftWith(idx, DriftOptions{Root: dir, NodeID: "orders-service"})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Unowned) != 0 {
		t.Fatalf("unowned leaked into node report: %v", r.Unowned)
	}
	for _, u := range r.Unmatched {
		if u.Node != "orders-service" {
			t.Fatalf("other node's glob: %+v", u)
		}
	}
	if _, err := DriftWith(idx, DriftOptions{Root: dir, NodeID: "nope"}); err == nil {
		t.Fatal("expected unknown node error")
	}
}

func TestDriftReadsFilesFromWorktree(t *testing.T) {
	dir := commitTwoNodeRepo(t)
	wt := filepath.Join(t.TempDir(), "wt")
	gitCmd(t, dir, "worktree", "add", "-b", "feat", wt)
	writeFile(t, filepath.Join(wt, "src", "payments", "pay.ts"), "x\n")
	idx := reopen(t, dir)
	main, err := DriftWith(idx, DriftOptions{Root: dir, NodeID: "payments-service"})
	if err != nil {
		t.Fatal(err)
	}
	inWT, err := DriftWith(idx, DriftOptions{Root: dir, FilesDir: wt, NodeID: "payments-service"})
	if err != nil {
		t.Fatal(err)
	}
	if len(main.Unmatched) == 0 || len(inWT.Unmatched) != 0 {
		t.Fatalf("main %+v worktree %+v", main.Unmatched, inWT.Unmatched)
	}
}

func TestDriftWorktreeArgumentIsChecked(t *testing.T) {
	dir := commitTwoNodeRepo(t)
	idx := reopen(t, dir)
	other := t.TempDir()
	gitInitRepo(t, other)
	if _, err := DriftWith(idx, DriftOptions{Root: dir, FilesDir: other}); err == nil || !strings.Contains(err.Error(), "different repository") {
		t.Fatalf("other repository accepted: %v", err)
	}
	if _, err := DriftWith(idx, DriftOptions{Root: dir, FilesDir: t.TempDir()}); err == nil {
		t.Fatal("non-repository accepted")
	}
	// A subdirectory resolves to its checkout's top level, so paths stay repo-relative.
	sub, err := DriftWith(idx, DriftOptions{Root: dir, FilesDir: "src"})
	if err != nil {
		t.Fatal(err)
	}
	whole, err := DriftWith(idx, DriftOptions{Root: dir})
	if err != nil {
		t.Fatal(err)
	}
	if sub.Findings() != whole.Findings() {
		t.Fatalf("subdirectory changed the report: %+v vs %+v", sub, whole)
	}
}

func TestDriftIgnoreInvalidGlobIsWarnedNotFatal(t *testing.T) {
	dir := commitTwoNodeRepo(t)
	idx := reopen(t, dir)
	if err := idx.SetConfigKeys(map[string]any{"drift_ignore": []string{"[", "docs/**"}}); err != nil {
		t.Fatal(err)
	}
	r, err := Drift(idx, dir)
	if err != nil {
		t.Fatalf("invalid drift_ignore broke drift: %v", err)
	}
	if len(r.Unowned) != 0 {
		t.Fatalf("valid entry not applied: %v", r.Unowned)
	}
}

func TestFormatDriftFooterMentionsIgnored(t *testing.T) {
	out := FormatDrift(DriftReport{Unowned: []string{"a.txt"}, Ignored: 3})
	if !strings.Contains(out, "a.txt") || !strings.Contains(out, "3 file(s) skipped by drift_ignore") {
		t.Fatalf("%s", out)
	}
}

func TestDriftIgnoreHintOnlyWhenManyUnowned(t *testing.T) {
	few := make([]string, 25)
	if out := FormatDrift(DriftReport{Unowned: few}); strings.Contains(out, "drift_ignore in .arch/config.json") {
		t.Fatalf("hint shown for 25 unowned files:\n%s", out)
	}
	many := make([]string, 26)
	if out := FormatDrift(DriftReport{Unowned: many}); !strings.Contains(out, "drift_ignore in .arch/config.json") {
		t.Fatalf("hint missing for 26 unowned files:\n%s", out)
	}
	if driftIgnoreHint(26) == "" || driftIgnoreHint(25) != "" {
		t.Fatal("threshold")
	}
}

func TestAddDriftIgnore(t *testing.T) {
	idx, _ := newModel(t)
	changed, err := idx.AddDriftIgnore("vendor/**")
	if err != nil || !changed {
		t.Fatalf("%v %v", changed, err)
	}
	if changed, err = idx.AddDriftIgnore("vendor/**"); err != nil || changed {
		t.Fatalf("duplicate: %v %v", changed, err)
	}
	if len(idx.DriftIgnore) != 1 || idx.DriftIgnore[0] != "vendor/**" {
		t.Fatalf("%v", idx.DriftIgnore)
	}
	if _, err := idx.AddDriftIgnore("[bad"); err == nil {
		t.Fatal("invalid glob accepted")
	}
}
