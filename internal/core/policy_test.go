package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompletionPolicyViaMethodOverrides(t *testing.T) {
	idx, dir := newModel(t)
	id := mk(t, idx, CreateInput{Name: "Svc", Type: "service", Implementation: []string{"svc/**"}, Status: StatusInProgress})
	// Default: agents may mark done.
	if err := idx.UpdateStatusGated(id, StatusDone); err != nil {
		t.Fatalf("default policy: %v", err)
	}
	if err := idx.SetConfigKeys(map[string]any{"method_overrides": map[string]any{"completion": map[string]any{"review_required": true}}}); err != nil {
		t.Fatal(err)
	}
	idx = reopen(t, dir)
	if err := idx.UpdateStatusGated(id, StatusDone); !errors.Is(err, ErrCompletionGate) || !strings.Contains(err.Error(), "needs_review") {
		t.Fatalf("done should be refused with review required: %v", err)
	}
	if err := idx.UpdateStatusGated(id, StatusNeedsReview); err != nil {
		t.Fatalf("needs_review should be allowed: %v", err)
	}
	// A node can opt out of review for itself.
	none := ReviewNone
	if err := idx.Update(id, UpdateInput{Review: &none}); err != nil {
		t.Fatal(err)
	}
	if err := idx.UpdateStatusGated(id, StatusDone); err != nil {
		t.Fatalf("node review none should win: %v", err)
	}
	// Agents may be barred from finishing altogether; humans (Update) still can.
	if err := idx.SetConfigKeys(map[string]any{"method_overrides": map[string]any{"completion": map[string]any{"agent_may_complete": false}}}); err != nil {
		t.Fatal(err)
	}
	idx = reopen(t, dir)
	for _, s := range []Status{StatusDone, StatusNeedsReview} {
		if err := idx.UpdateStatusGated(id, s); !errors.Is(err, ErrCompletionGate) {
			t.Fatalf("%s should be refused: %v", s, err)
		}
	}
	if err := idx.UpdateStatusGated(id, StatusBlocked); err != nil {
		t.Fatalf("non-completion statuses stay open: %v", err)
	}
	done := StatusDone
	if err := idx.Update(id, UpdateInput{Status: &done}); err != nil {
		t.Fatalf("human path must not be gated: %v", err)
	}
}

func TestMethodOverridesLintParamAndBadInput(t *testing.T) {
	idx, dir := newModel(t)
	if err := idx.SetConfigKeys(map[string]any{
		"method": "design-first",
		"method_overrides": map[string]any{
			"lint": map[string]any{
				"max-depth":     map[string]any{"severity": "advisory", "params": map[string]any{"max_depth": 1}},
				"no-such":       "advisory",
				"leaf-contract": map[string]any{"severity": "bogus"},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	idx = reopen(t, dir)
	l0 := mk(t, idx, CreateInput{Name: "L0", Type: "boundary"})
	l1 := mk(t, idx, CreateInput{Name: "L1", Type: "boundary", ParentID: l0})
	mk(t, idx, CreateInput{Name: "L2", Type: "service", ParentID: l1})
	if got := onlyRule(findingsFor(t, idx, CheckModelOptions{}), RuleMaxDepth); len(got) != 1 {
		t.Fatalf("override max_depth 1 not applied: %+v", got)
	}
	if len(idx.Warnings()) < 2 {
		t.Fatalf("bad overrides should warn: %v", idx.Warnings())
	}
	// The built-in method itself is untouched.
	b, _ := BuiltinMethod("design-first")
	if b.Lint[RuleMaxDepth].Params != nil {
		t.Fatalf("built-in method was mutated: %+v", b.Lint[RuleMaxDepth])
	}
}

func TestNodeMetaValidationAndRoundTrip(t *testing.T) {
	idx, dir := newModel(t)
	if _, err := idx.Create(CreateInput{Name: "Bad", Type: "service", Risk: "extreme"}); !errors.Is(err, ErrBadNodeMeta) {
		t.Fatalf("bad risk: %v", err)
	}
	if _, err := idx.Create(CreateInput{Name: "Bad2", Type: "service", Review: "maybe"}); !errors.Is(err, ErrBadNodeMeta) {
		t.Fatalf("bad review: %v", err)
	}
	id := mk(t, idx, CreateInput{Name: "Auth", Type: "service", Risk: "High", Review: "required", Verify: []string{" go test ./auth/... ", ""}})
	idx = reopen(t, dir)
	n := idx.Nodes[id]
	if n.Risk != "high" || n.Review != "required" || len(n.Verify) != 1 || n.Verify[0] != "go test ./auth/..." {
		t.Fatalf("round trip: %+v", n)
	}
	empty := []string{}
	if err := idx.Update(id, UpdateInput{Verify: &empty}); err != nil {
		t.Fatal(err)
	}
	if len(idx.Nodes[id].Verify) != 0 || idx.Nodes[id].Risk != "high" {
		t.Fatalf("clearing verify: %+v", idx.Nodes[id])
	}
	many := make([]string, MaxVerifyCommands+1)
	for i := range many {
		many[i] = "x"
	}
	if err := idx.Update(id, UpdateInput{Verify: &many}); !errors.Is(err, ErrBadNodeMeta) {
		t.Fatalf("too many verify commands: %v", err)
	}
	// Proposals carry the metadata too.
	pid, err := idx.Stage(StageInput{Source: OpUpdateNode, Ops: []StageOp{{Op: OpUpdateNode, NodeID: id, Update: UpdateInput{Risk: ptr("low")}}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.AcceptOperation(pid, 0, false); err != nil || idx.Nodes[id].Risk != "low" {
		t.Fatalf("proposal update: %v %+v", err, idx.Nodes[id])
	}
}

func ptr[T any](v T) *T { return &v }

func TestBriefFollowsCompletionPolicyAndShowsVerify(t *testing.T) {
	idx, dir := newModel(t)
	id := mk(t, idx, CreateInput{Name: "Svc", Type: "service", Implementation: []string{"svc/**"}, Risk: "high", Verify: []string{"make test"}})
	b, _ := idx.Brief(id)
	if !strings.Contains(b, "make test") || !strings.Contains(b, "risk high") || !strings.HasSuffix(strings.TrimSpace(b), BriefClosing) {
		t.Fatalf("default brief:\n%s", b)
	}
	if err := idx.SetConfigKeys(map[string]any{"method_overrides": map[string]any{"completion": map[string]any{"review_required": true}}}); err != nil {
		t.Fatal(err)
	}
	idx = reopen(t, dir)
	if b, _ = idx.Brief(id); !strings.Contains(b, "needs_review for your node (not done)") {
		t.Fatalf("review brief:\n%s", b)
	}
	if err := idx.SetConfigKeys(map[string]any{"method_overrides": map[string]any{"completion": map[string]any{"agent_may_complete": false}}}); err != nil {
		t.Fatal(err)
	}
	idx = reopen(t, dir)
	if b, _ = idx.Brief(id); !strings.Contains(b, "report your result to the architect") {
		t.Fatalf("no-complete brief:\n%s", b)
	}
}

func TestCheckDoneUsesBranchDiffInWorktree(t *testing.T) {
	dir := commitTwoNodeRepo(t)
	idx := reopen(t, dir)
	wt := filepath.Join(t.TempDir(), "wt")
	gitCmd(t, dir, "worktree", "add", "-b", "feat", wt)
	writeFile(t, filepath.Join(wt, "src", "payments", "pay.ts"), "x\n")
	gitCmd(t, wt, "add", "-A")
	gitCmd(t, wt, "commit", "-m", "pay")
	rep, err := CheckDone(idx, DoneOptions{Root: dir, FilesDir: wt, NodeID: "payments-service"})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Pass || len(rep.Files) != 1 || rep.Files[0] != "src/payments/pay.ts" || rep.Completion.FinishStatus != "done" {
		t.Fatalf("clean branch: %+v", rep)
	}
	// Committed and uncommitted edits outside scope both count.
	writeFile(t, filepath.Join(wt, "src", "orders", "create.ts"), "changed\n")
	rep, err = CheckDone(idx, DoneOptions{Root: dir, FilesDir: wt, NodeID: "payments-service"})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Pass || len(rep.Violations) != 1 || rep.Violations[0].Path != "src/orders/create.ts" {
		t.Fatalf("scope violation missed: %+v", rep)
	}
	// The main checkout has no payments file yet, so its drift problem shows there.
	rep, err = CheckDone(idx, DoneOptions{Root: dir, NodeID: "payments-service"})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Pass || len(rep.Drift.Unmatched) == 0 {
		t.Fatalf("main checkout should show unmatched glob: %+v", rep)
	}
}

func TestBranchFilesBeforeFirstCommit(t *testing.T) {
	dir := t.TempDir()
	gitInitRepo(t, dir)
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")
	files, _, err := BranchFiles(dir, "")
	if err != nil || len(files) != 1 || files[0] != "a.go" {
		t.Fatalf("files %v err %v", files, err)
	}
}

func TestBranchFilesIgnoresModelFiles(t *testing.T) {
	dir := commitTwoNodeRepo(t)
	idx := reopen(t, dir)
	st := StatusInProgress
	if err := idx.Update("orders-service", UpdateInput{Status: &st}); err != nil {
		t.Fatal(err)
	}
	files, _, err := BranchFiles(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("status update leaked into the branch diff: %v", files)
	}
}

func TestBranchFilesBaseHandling(t *testing.T) {
	dir := commitTwoNodeRepo(t)
	base := strings.TrimSpace(gitCmd(t, dir, "rev-parse", "HEAD"))
	writeFile(t, filepath.Join(dir, "src", "orders", "new.ts"), "x\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "later")
	writeFile(t, filepath.Join(dir, "src", "orders", "wip.ts"), "x\n")

	files, resolved, err := BranchFiles(dir, base)
	if err != nil || resolved != base || strings.Join(files, ",") != "src/orders/new.ts,src/orders/wip.ts" {
		t.Fatalf("explicit base: %v %q %v", files, resolved, err)
	}
	for _, bad := range []string{"--output=/tmp/ambit-should-not-exist", "docs", "no-such-ref"} {
		if _, _, err := BranchFiles(dir, bad); err == nil {
			t.Fatalf("base %q should be refused", bad)
		}
	}
	if _, err := os.Stat("/tmp/ambit-should-not-exist"); err == nil {
		t.Fatal("option-like base wrote a file")
	}
}

func TestCheckDoneWarnsWhenOnlyUncommittedWorkWasCompared(t *testing.T) {
	dir := commitTwoNodeRepo(t) // single branch named master/main with no trunk to diverge from
	gitCmd(t, dir, "branch", "-m", "develop")
	idx := reopen(t, dir)
	rep, err := CheckDone(idx, DoneOptions{Root: dir, NodeID: "orders-service"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range rep.Warnings {
		found = found || strings.Contains(w, "no trunk branch")
	}
	if !found {
		t.Fatalf("fallback to HEAD should warn: %+v", rep.Warnings)
	}
}

func TestDesignFirstGateRefusesNeedsReviewInDesign(t *testing.T) {
	idx, dir := newModel(t)
	id := mk(t, idx, CreateInput{Name: "Svc", Type: "service", Implementation: []string{"svc/**"}})
	if err := idx.SetConfigKeys(map[string]any{"method": "design-first", "stage": "design"}); err != nil {
		t.Fatal(err)
	}
	idx = reopen(t, dir)
	if err := idx.UpdateStatusGated(id, StatusNeedsReview); !errors.Is(err, ErrStageGate) {
		t.Fatalf("needs_review during design: %v", err)
	}
}

func TestMethodOverrideKeepsBaseParamsWhenOnlySeverityGiven(t *testing.T) {
	idx, dir := newModel(t)
	if err := idx.SetConfigKeys(map[string]any{
		"method": "design-first",
		"method_overrides": map[string]any{"lint": map[string]any{
			"max-depth": map[string]any{"params": map[string]any{"max_depth": 1}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	idx = reopen(t, dir)
	set := idx.ActiveMethod().Lint[RuleMaxDepth]
	if set.Severity != LintAdvisory || set.intParam("max_depth", 0) != 1 {
		t.Fatalf("params-only override: %+v", set)
	}
	// A bare severity later keeps those params.
	if err := idx.SetConfigKeys(map[string]any{"method_overrides": map[string]any{"lint": map[string]any{"max-depth": "enforced"}}}); err != nil {
		t.Fatal(err)
	}
	idx = reopen(t, dir)
	if set = idx.ActiveMethod().Lint[RuleMaxDepth]; set.Severity != LintEnforced {
		t.Fatalf("severity override: %+v", set)
	}
}

func TestOrderCreatesEdgeCases(t *testing.T) {
	c := func(id, parent string, deps ...NodeID) StageOp {
		return StageOp{Op: OpCreateNode, Create: CreateInput{ID: NodeID(id), ParentID: NodeID(parent), DependsOn: deps}}
	}
	ids := func(ops []StageOp) string {
		var out []string
		for _, o := range ops {
			if o.Op == OpCreateNode {
				out = append(out, string(o.Create.ID))
			} else {
				out = append(out, "<"+o.Op+">")
			}
		}
		return strings.Join(out, ",")
	}
	// Already ordered input is unchanged, and an existing parent does not move anything.
	if got := ids(orderCreates([]StageOp{c("a", "existing"), c("b", "a")})); got != "a,b" {
		t.Fatalf("ordered input: %s", got)
	}
	// Children listed first come after their parent and dependencies.
	if got := ids(orderCreates([]StageOp{c("child", "zone", "base"), c("zone", ""), c("base", "")})); got != "zone,base,child" && got != "base,zone,child" {
		t.Fatalf("reordered: %s", got)
	}
	// Duplicate ids and cycles keep a stable order for validation to report.
	if got := ids(orderCreates([]StageOp{c("x", ""), c("x", "")})); got != "x,x" {
		t.Fatalf("duplicates: %s", got)
	}
	if got := ids(orderCreates([]StageOp{c("m", "", "n"), c("n", "", "m")})); got == "" {
		t.Fatal("cycle lost operations")
	}
	// Other operations keep their slots.
	rel := StageOp{Op: OpSetRelationship}
	if got := ids(orderCreates([]StageOp{c("child", "zone"), rel, c("zone", "")})); got != "zone,<set_relationship>,child" {
		t.Fatalf("interleaved: %s", got)
	}
}

func TestPruneResolvedKeepsMostRecentlyResolved(t *testing.T) {
	idx, _ := newModel(t)
	old := maxResolvedProposals
	maxResolvedProposals = 2
	defer func() { maxResolvedProposals = old }()
	first := stageTwoCreatesNamed(t, idx, "a1", "a2")
	second := stageTwoCreatesNamed(t, idx, "b1", "b2")
	third := stageTwoCreatesNamed(t, idx, "c1", "c2")
	// Resolve the oldest proposal last: it must survive pruning.
	for _, id := range []string{third, second, first} {
		if err := idx.RejectAll(id); err != nil {
			t.Fatal(err)
		}
		time.Sleep(1100 * time.Millisecond)
	}
	if _, err := idx.ProposalState(first); err != nil {
		t.Fatalf("most recently resolved proposal was pruned: %v", err)
	}
	if _, err := idx.ProposalState(third); !errors.Is(err, ErrProposalNotFound) {
		t.Fatalf("oldest resolution should be pruned: %v", err)
	}
}

func TestRestageRefusesTamperedManifest(t *testing.T) {
	idx, dir := newModel(t)
	id := stageTwoCreates(t, idx)
	if err := idx.AcceptOperation(id, 0, false); err != nil {
		t.Fatal(err)
	}
	mpath := filepath.Join(dir, ".arch", ".proposals", id, "manifest.json")
	raw, err := os.ReadFile(mpath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mpath, []byte(strings.Replace(string(raw), `"node_id": "alpha"`, `"node_id": "../../x"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Restage(id, []int{0}); !errors.Is(err, ErrUnreadableProposal) {
		t.Fatalf("tampered node id: %v", err)
	}
	// A missing staged file is refused up front, not at accept time.
	if err := os.WriteFile(mpath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, ".arch", ".proposals", id, "nodes", "alpha.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Restage(id, []int{0}); !errors.Is(err, ErrUnreadableProposal) {
		t.Fatalf("missing staged file: %v", err)
	}
}

func TestNodeMetaKeyOrderOnDisk(t *testing.T) {
	idx, dir := newModel(t)
	id := mk(t, idx, CreateInput{Name: "Svc", Type: "service", Protected: true, Risk: "low", Review: "none", Verify: []string{"make test"}})
	data, err := os.ReadFile(filepath.Join(dir, ".arch", "nodes", string(id)+".json"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	order := []string{`"protected"`, `"risk"`, `"review"`, `"verify"`, `"status"`}
	last := -1
	for _, k := range order {
		i := strings.Index(s, k)
		if i < 0 || i < last {
			t.Fatalf("key %s missing or out of order:\n%s", k, s)
		}
		last = i
	}
}

func TestAgentCannotSetDoneWithOpenMilestonesOrHighRisk(t *testing.T) {
	idx, _ := newModel(t)
	ms := []Milestone{{ID: "a", Status: MilestoneDone}, {ID: "b"}}
	id := mk(t, idx, CreateInput{Name: "Svc", Type: "service", Status: StatusInProgress, NodeExtras: NodeExtras{Milestones: ms}})
	if err := idx.UpdateStatusGated(id, StatusDone); !errors.Is(err, ErrCompletionGate) || !strings.Contains(err.Error(), "b") {
		t.Fatalf("open milestone: %v", err)
	}
	if err := idx.UpdateStatusGated(id, StatusNeedsReview); err != nil {
		t.Fatalf("needs_review allowed: %v", err)
	}
	// Humans are unaffected.
	d := StatusDone
	if err := idx.Update(id, UpdateInput{Status: &d}); err != nil {
		t.Fatal(err)
	}
	all := []Milestone{{ID: "a", Status: MilestoneDone}, {ID: "b", Status: MilestoneDone}}
	if err := idx.Update(id, UpdateInput{NodeExtrasPatch: NodeExtrasPatch{Milestones: &all}}); err != nil {
		t.Fatal(err)
	}
	if err := idx.UpdateStatusGated(id, StatusDone); err != nil {
		t.Fatalf("all milestones done: %v", err)
	}
	hi := mk(t, idx, CreateInput{Name: "Risky", Type: "service", Status: StatusInProgress, Risk: RiskHigh})
	if err := idx.UpdateStatusGated(hi, StatusDone); !errors.Is(err, ErrCompletionGate) || !strings.Contains(err.Error(), "needs_review") {
		t.Fatalf("high risk: %v", err)
	}
	if err := idx.UpdateStatusGated(hi, StatusNeedsReview); err != nil {
		t.Fatalf("high risk needs_review: %v", err)
	}
}
