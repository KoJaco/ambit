package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func planRepo(t *testing.T) (*Index, string) {
	t.Helper()
	dir := t.TempDir()
	gitInitRepo(t, dir)
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "README.md"), "x\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "base")
	return reopen(t, dir), dir
}

func TestCheckDeliverablesExistsNonEmptyAndSafe(t *testing.T) {
	idx, dir := planRepo(t)
	mustCreate(t, idx, CreateInput{ID: "docs", Name: "Docs", Type: "service", NodeExtras: NodeExtras{
		Deliverables: []string{"docs/a.md", "docs/empty.md", "docs/missing.md", "out", "docs/link.md"},
	}})
	writeFile(t, filepath.Join(dir, "docs", "a.md"), "hello\n")
	writeFile(t, filepath.Join(dir, "docs", "empty.md"), "")
	writeFile(t, filepath.Join(dir, "out", "nested", "f.txt"), "data")
	outside := filepath.Join(t.TempDir(), "secret.md")
	writeFile(t, outside, "secret")
	if err := os.Symlink(outside, filepath.Join(dir, "docs", "link.md")); err != nil {
		t.Skip("symlinks unavailable")
	}
	rep, err := CheckDeliverables(idx, DeliverablesOptions{Root: dir, NodeID: "docs"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, it := range rep.Items {
		got[it.Path] = it.Status
	}
	want := map[string]string{"docs/a.md": "ok", "docs/empty.md": "empty", "docs/missing.md": "missing", "out": "ok", "docs/link.md": "unsafe"}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s: got %q want %q (%+v)", k, got[k], v, rep)
		}
	}
	if rep.Pass || len(rep.Problems) != 3 {
		t.Fatalf("report %+v", rep)
	}
	if _, err := CheckDeliverables(idx, DeliverablesOptions{Root: dir, NodeID: "ghost"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown node: %v", err)
	}
}

func TestCheckDoneDeliverablesGlobsAndFinishStatus(t *testing.T) {
	idx, dir := planRepo(t)
	mustCreate(t, idx, CreateInput{ID: "adr", Name: "ADR", Type: "service",
		Implementation: []string{"docs/decisions/0003-*.md", "src/adr/**"},
		NodeExtras:     NodeExtras{Deliverables: []string{"docs/decisions/0003-choice.md"}}})
	wt := filepath.Join(t.TempDir(), "wt")
	gitCmd(t, dir, "worktree", "add", "-b", "feat", wt)
	idx = reopen(t, dir)

	// Nothing written yet: the glob that names the deliverable is not an unmatched-glob
	// failure; the deliverable itself is the problem.
	rep, err := CheckDone(idx, DoneOptions{Root: dir, FilesDir: wt, NodeID: "adr"})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range rep.Problems {
		if strings.Contains(p, "0003-*.md") {
			t.Fatalf("deliverable glob reported as unmatched: %v", rep.Problems)
		}
	}
	joined := strings.Join(rep.Problems, "|")
	if rep.Pass || !strings.Contains(joined, "deliverable docs/decisions/0003-choice.md does not exist") || !strings.Contains(joined, "src/adr/**") {
		t.Fatalf("problems %v", rep.Problems)
	}

	// Written (untracked) in the worktree: the glob matches it and everything passes but src.
	writeFile(t, filepath.Join(wt, "docs", "decisions", "0003-choice.md"), "# 3\n")
	writeFile(t, filepath.Join(wt, "src", "adr", "x.go"), "package adr\n")
	rep, err = CheckDone(idx, DoneOptions{Root: dir, FilesDir: wt, NodeID: "adr"})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Pass || rep.Deliverables == nil || !rep.Deliverables.Pass || rep.Completion.FinishStatus != "done" {
		t.Fatalf("after writing: %+v", rep)
	}
	for _, f := range rep.Files {
		if f == "docs/decisions/0003-choice.md" {
			return
		}
	}
	t.Fatalf("untracked deliverable not in files: %v", rep.Files)
}

func TestCheckDoneFinishStatusMilestonesAndRisk(t *testing.T) {
	idx, dir := planRepo(t)
	mustCreate(t, idx, CreateInput{ID: "part", Name: "Part", Type: "service", Implementation: []string{"src/part/**"},
		NodeExtras: NodeExtras{Milestones: []Milestone{{ID: "api", Status: "done"}, {ID: "ui", Status: "in_progress"}}}})
	mustCreate(t, idx, CreateInput{ID: "risky", Name: "Risky", Type: "service", Implementation: []string{"src/risky/**"}, Risk: "high"})
	mustCreate(t, idx, CreateInput{ID: "plain", Name: "Plain", Type: "service", Implementation: []string{"src/plain/**"}})
	for _, p := range []string{"part", "risky", "plain"} {
		writeFile(t, filepath.Join(dir, "src", p, "x.go"), "package x\n")
	}
	check := func(id NodeID) DoneCompletion {
		rep, err := CheckDone(idx, DoneOptions{Root: dir, NodeID: id})
		if err != nil {
			t.Fatal(err)
		}
		return rep.Completion
	}
	if c := check("part"); c.FinishStatus != "needs_review" || !c.Partial || !strings.Contains(c.Reason, "ui") {
		t.Fatalf("partial: %+v", c)
	}
	if c := check("risky"); c.FinishStatus != "needs_review" || c.Partial || !c.ReviewRequired {
		t.Fatalf("risky: %+v", c)
	}
	if c := check("plain"); c.FinishStatus != "done" {
		t.Fatalf("plain: %+v", c)
	}
	// Once every milestone is done the node may finish normally.
	all := []Milestone{{ID: "api", Status: "done"}, {ID: "ui", Status: "done"}}
	if err := idx.Update("part", UpdateInput{NodeExtrasPatch: NodeExtrasPatch{Milestones: &all}}); err != nil {
		t.Fatal(err)
	}
	if c := check("part"); c.FinishStatus != "done" || c.Partial {
		t.Fatalf("all milestones done: %+v", c)
	}
}

func TestCheckDoneBaseAttributesIntegrationBranch(t *testing.T) {
	idx, dir := planRepo(t)
	mustCreate(t, idx, CreateInput{ID: "a", Name: "A", Type: "service", Implementation: []string{"src/a/**"}})
	mustCreate(t, idx, CreateInput{ID: "b", Name: "B", Type: "service", Implementation: []string{"src/b/**"}})
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "model")
	gitCmd(t, dir, "checkout", "-b", "integration")
	writeFile(t, filepath.Join(dir, "src", "b", "other.go"), "package b\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "b work")
	base := strings.TrimSpace(gitCmd(t, dir, "rev-parse", "HEAD"))
	writeFile(t, filepath.Join(dir, "src", "a", "mine.go"), "package a\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "a work")
	gitCmd(t, dir, "branch", "-f", "main", "HEAD~2")

	rep, err := CheckDone(idx, DoneOptions{Root: dir, NodeID: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Pass || len(rep.Violations) != 1 || rep.Violations[0].Path != "src/b/other.go" {
		t.Fatalf("auto base sees b's work as a's: %+v", rep)
	}
	rep, err = CheckDone(idx, DoneOptions{Root: dir, NodeID: "a", Base: base})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Pass || rep.Base != base || len(rep.Files) != 1 || rep.Files[0] != "src/a/mine.go" {
		t.Fatalf("explicit base: %+v", rep)
	}
}

func TestRunVerifyStatusesTimeoutAndNotRunnable(t *testing.T) {
	idx, dir := planRepo(t)
	t.Setenv("AMBIT_TEST_SET", "yes")
	os.Unsetenv("AMBIT_TEST_UNSET")
	t.Setenv("AMBIT_TEST_SECRET_NOT_REFERENCED", "hunter2")
	mustCreate(t, idx, CreateInput{ID: "v", Name: "V", Type: "service", Verify: []string{
		"echo hello && test -n \"$AMBIT_TEST_SET\"",
		"echo oops; exit 3",
		"definitely-not-a-binary --flag",
		"echo $AMBIT_TEST_UNSET",
		"echo ${AMBIT_TEST_UNSET:-fallback}",
		"sleep 5",
		"env | grep -c AMBIT_TEST_SECRET_NOT_REFERENCED; true",
	}})
	rep, err := RunVerify(context.Background(), idx, VerifyOptions{Root: dir, NodeID: "v", TimeoutS: 1})
	if err != nil {
		t.Fatal(err)
	}
	statuses := []string{VerifyPassed, VerifyFailed, VerifyNotRunnable, VerifyNotRunnable, VerifyPassed, VerifyTimeout, VerifyPassed}
	for i, want := range statuses {
		if rep.Results[i].Status != want {
			t.Fatalf("%d %q: got %q want %q (%+v)", i, rep.Results[i].Command, rep.Results[i].Status, want, rep.Results[i])
		}
	}
	if rep.Results[1].ExitCode != 3 || !strings.Contains(rep.Results[1].Output, "oops") {
		t.Fatalf("failed result %+v", rep.Results[1])
	}
	if !strings.Contains(rep.Results[2].Reason, "definitely-not-a-binary") || !strings.Contains(rep.Results[3].Reason, "AMBIT_TEST_UNSET") {
		t.Fatalf("reasons %+v %+v", rep.Results[2], rep.Results[3])
	}
	if !strings.Contains(rep.Results[4].Output, "fallback") {
		t.Fatalf("default expansion: %+v", rep.Results[4])
	}
	if strings.TrimSpace(rep.Results[6].Output) != "0" {
		t.Fatalf("unreferenced secret leaked into the command environment: %+v", rep.Results[6])
	}
	if rep.Pass || rep.Passed != 3 || rep.Failed != 1 || rep.TimedOut != 1 || rep.NotRunnable != 2 {
		t.Fatalf("counts %+v", rep)
	}
}

func TestRunVerifyTruncatesOutputRunsInWorktreeAndCapsTimeout(t *testing.T) {
	idx, dir := planRepo(t)
	mustCreate(t, idx, CreateInput{ID: "v", Name: "V", Type: "service", Verify: []string{
		"head -c 50000 /dev/zero | tr '\\0' 'x'; echo END",
		"pwd",
	}})
	wt := filepath.Join(t.TempDir(), "wt")
	gitCmd(t, dir, "worktree", "add", "-b", "feat", wt)
	start := time.Now()
	rep, err := RunVerify(context.Background(), idx, VerifyOptions{Root: dir, FilesDir: wt, NodeID: "v", TimeoutS: 100000})
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 30*time.Second {
		t.Fatal("slow")
	}
	r := rep.Results[0]
	if !r.OutputTruncated || len(r.Output) > verifyOutputCap || !strings.HasSuffix(r.Output, "END") {
		t.Fatalf("truncation: truncated=%v len=%d", r.OutputTruncated, len(r.Output))
	}
	wantDir, _ := filepath.EvalSymlinks(wt)
	gotDir, _ := filepath.EvalSymlinks(strings.TrimSpace(rep.Results[1].Output))
	if gotDir != wantDir {
		t.Fatalf("ran in %q, want worktree %q", gotDir, wantDir)
	}
}

func TestRunVerifyOnlyReadsCommandsFromTheModelNotProposals(t *testing.T) {
	idx, dir := planRepo(t)
	mustCreate(t, idx, CreateInput{ID: "v", Name: "V", Type: "service"})
	marker := filepath.Join(t.TempDir(), "ran")
	cmds := []string{"touch " + marker}
	if _, err := idx.Stage(StageInput{Source: OpUpdateNode, Ops: []StageOp{{Op: OpUpdateNode, NodeID: "v", Update: UpdateInput{Verify: &cmds}}}}); err != nil {
		t.Fatal(err)
	}
	rep, err := RunVerify(context.Background(), idx, VerifyOptions{Root: dir, NodeID: "v"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Results) != 0 {
		t.Fatalf("ran a pending proposal's command: %+v", rep)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("pending command executed")
	}
	if _, err := RunVerify(context.Background(), idx, VerifyOptions{Root: dir, NodeID: "ghost"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown node: %v", err)
	}
}

func TestCheckDependentsRunsDirectDependentsOnly(t *testing.T) {
	idx, dir := planRepo(t)
	mustCreate(t, idx, CreateInput{ID: "base", Name: "Base", Type: "service"})
	mustCreate(t, idx, CreateInput{ID: "user", Name: "User", Type: "service", DependsOn: []NodeID{"base"}, Verify: []string{"echo user-ok"}})
	mustCreate(t, idx, CreateInput{ID: "quiet", Name: "Quiet", Type: "service", DependsOn: []NodeID{"base"}})
	mustCreate(t, idx, CreateInput{ID: "far", Name: "Far", Type: "service", DependsOn: []NodeID{"user"}, Verify: []string{"exit 1"}})
	rep, err := CheckDependents(context.Background(), idx, VerifyOptions{Root: dir, NodeID: "base"})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Pass || len(rep.Dependents) != 1 || rep.Dependents[0].NodeID != "user" || len(rep.Skipped) != 1 || rep.Skipped[0].NodeID != "quiet" {
		t.Fatalf("dependents %+v", rep)
	}
	// A spike's dependents are the nodes it gates.
	mustCreate(t, idx, CreateInput{ID: "spike", Name: "Spike", Type: "service", NodeExtras: NodeExtras{Gates: []NodeID{"far"}}})
	rep, err = CheckDependents(context.Background(), idx, VerifyOptions{Root: dir, NodeID: "spike"})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Pass || len(rep.Dependents) != 1 || rep.Dependents[0].NodeID != "far" {
		t.Fatalf("gated dependents %+v", rep)
	}
}

func TestCheckContractSync(t *testing.T) {
	idx, dir := planRepo(t)
	mustCreate(t, idx, CreateInput{ID: "api", Name: "API", Type: "service", NodeExtras: NodeExtras{References: []string{"docs/api.md", "docs/gone.md"}}})
	mustCreate(t, idx, CreateInput{ID: "web", Name: "Web", Type: "service"})
	if _, err := idx.SetRelationship(SetRelationshipInput{From: "web", To: "api", Label: "calls", Kind: KindSync,
		Contract: &EdgeContract{Format: "openapi", Ref: "docs/openapi.yaml"}}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "docs", "api.md"), "doc\n")
	rep, err := CheckContractSync(idx, ContractSyncOptions{Root: dir, NodeID: "api"})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Pass || !rep.HasContract || len(rep.References) != 3 {
		t.Fatalf("report %+v", rep)
	}
	joined := strings.Join(rep.Problems, "|")
	if !strings.Contains(joined, "docs/gone.md does not exist") || !strings.Contains(joined, "docs/openapi.yaml does not exist") || strings.Contains(joined, "docs/api.md") {
		t.Fatalf("problems %v", rep.Problems)
	}
	// Contract changed after the node was marked in progress.
	st := StatusInProgress
	if err := idx.Update("api", UpdateInput{Status: &st}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "docs", "gone.md"), "x\n")
	writeFile(t, filepath.Join(dir, "docs", "openapi.yaml"), "x\n")
	rep, _ = CheckContractSync(idx, ContractSyncOptions{Root: dir, NodeID: "api"})
	if !rep.Pass || rep.ContractChanged {
		t.Fatalf("clean: %+v", rep)
	}
	c := &NodeContract{Payloads: "new shape"}
	if err := idx.Update("api", UpdateInput{Contract: c}); err != nil {
		t.Fatal(err)
	}
	rep, _ = CheckContractSync(idx, ContractSyncOptions{Root: dir, NodeID: "api"})
	if rep.Pass || !rep.ContractChanged {
		t.Fatalf("changed: %+v", rep)
	}
}

func TestLintSharedManifest(t *testing.T) {
	idx, dir := lintIdx(t, "advisory", RuleSharedManifest)
	gitInitRepo(t, dir)
	writeFile(t, filepath.Join(dir, "go.mod"), "module x\n")
	writeFile(t, filepath.Join(dir, "web", "package.json"), "{}\n")
	writeFile(t, filepath.Join(dir, "api", "package.json"), "{}\n")
	writeFile(t, filepath.Join(dir, "pnpm-lock.yaml"), "x\n")
	mk(t, idx, CreateInput{ID: "web", Name: "Web", Type: "service", Implementation: []string{"web/**", "pnpm-lock.yaml"}})
	mk(t, idx, CreateInput{ID: "api", Name: "API", Type: "service", Implementation: []string{"api/**"}})
	mk(t, idx, CreateInput{ID: "lock2", Name: "Lock2", Type: "service", Implementation: []string{"*.yaml"}})
	res := findingsFor(t, idx, CheckModelOptions{})
	f := onlyRule(res, RuleSharedManifest)
	text := ""
	for _, x := range f {
		text += x.Message + "\n"
	}
	if strings.Contains(text, "go.mod") || !strings.Contains(text, "pnpm-lock.yaml is owned by several nodes") ||
		strings.Contains(text, "web/package.json") || strings.Contains(text, "api/package.json") {
		t.Fatalf("findings:\n%s", text)
	}
	for _, x := range f {
		if x.Severity != LintAdvisory {
			t.Fatalf("not advisory: %+v", x)
		}
	}
	// A shared owner clears it.
	mk(t, idx, CreateInput{ID: "root", Name: "Root", Type: "service", Implementation: []string{"go.mod", "pnpm-lock.yaml"}, NodeExtras: NodeExtras{Shared: true}})
	f = onlyRule(findingsFor(t, idx, CheckModelOptions{}), RuleSharedManifest)
	if len(f) != 0 {
		t.Fatalf("shared owner should clear: %+v", f)
	}
}

func TestLintSharedManifestNestedOwnersAreFine(t *testing.T) {
	idx, dir := lintIdx(t, "advisory", RuleSharedManifest)
	gitInitRepo(t, dir)
	writeFile(t, filepath.Join(dir, "go.mod"), "module x\n")
	writeFile(t, filepath.Join(dir, "go.sum"), "x\n")
	mk(t, idx, CreateInput{ID: "app", Name: "App", Type: "service", Implementation: []string{"go.mod"}})
	mk(t, idx, CreateInput{ID: "core", Name: "Core", Type: "service", ParentID: "app", Implementation: []string{"go.mod"}})
	f := onlyRule(findingsFor(t, idx, CheckModelOptions{}), RuleSharedManifest)
	if len(f) != 0 {
		t.Fatalf("nested owners and unowned go.sum must not be flagged: %+v", f)
	}
}

func TestBriefShowsBuildPlanFields(t *testing.T) {
	idx, dir := newModel(t)
	mustCreate(t, idx, CreateInput{ID: "n", Name: "N", Type: "service", Implementation: []string{"n/**"}, Risk: "high", Verify: []string{"make test"},
		NodeExtras: NodeExtras{
			HumanGates:   []HumanGate{{Kind: "credential", Note: "live key"}, {Kind: "decision"}},
			Deliverables: []string{"docs/n.md"}, References: []string{"docs/ref.md"},
			Milestones: []Milestone{{ID: "m1", Label: "first", Status: "done"}},
		}})
	if err := idx.SetGateCleared("n", 1, true); err != nil {
		t.Fatal(err)
	}
	idx = reopen(t, dir)
	b, err := idx.Brief("n")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"make test", "Deliverables", "docs/n.md", "docs/ref.md", "m1: first [done]", "credential: live key [NOT cleared]", "decision [cleared]",
		"Risk and review: risk is high, so you finish at needs_review", "read-only tools", "write documentation deliverables yourself"} {
		if !strings.Contains(b, want) {
			t.Fatalf("brief missing %q:\n%s", want, b)
		}
	}
}
