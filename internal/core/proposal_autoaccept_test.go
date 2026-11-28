package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// autoFixture is a design-first project in the build stage with auto-accept on: platform
// (container) holding svc and dep, plus a protected vault.
func autoFixture(t *testing.T) (*Index, string) {
	t.Helper()
	idx, dir := newModel(t)
	gitInitRepo(t, dir)
	mk := func(in CreateInput) NodeID {
		t.Helper()
		id, err := idx.Create(in)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	platform := mk(CreateInput{Name: "Platform", Type: "boundary", Spec: "# platform\n"})
	mk(CreateInput{Name: "Svc", Type: "service", ParentID: platform, Spec: "# svc\n", Implementation: []string{"src/svc/**"},
		NodeExtras: NodeExtras{Deliverables: []string{"src/svc/main.go"}}})
	mk(CreateInput{Name: "Dep", Type: "service", ParentID: platform, Spec: "# dep\n", Implementation: []string{"src/dep/**"}})
	mk(CreateInput{Name: "Vault", Type: "service", ParentID: platform, Spec: "# vault\n", Implementation: []string{"src/vault/**"}, Protected: true})
	if err := idx.SetConfigKeys(map[string]any{
		"method": "design-first", "stage": "build", "proposal_policy": map[string]any{"auto_accept": true},
	}); err != nil {
		t.Fatal(err)
	}
	return idx, dir
}

func stageUpdate(t *testing.T, idx *Index, origin string, id NodeID, u UpdateInput) string {
	t.Helper()
	pid, err := idx.Stage(StageInput{Source: OpUpdateNode, Origin: origin, Ops: []StageOp{{Op: OpUpdateNode, NodeID: id, Update: u}}})
	if err != nil {
		t.Fatal(err)
	}
	return pid
}

func strp(s string) *string { return &s }

func TestAutoAcceptGating(t *testing.T) {
	idx, dir := autoFixture(t)
	pid := stageUpdate(t, idx, "mcp-writer", "svc", UpdateInput{Spec: strp("# svc v2\n")})

	// Policy off.
	if _, err := idx.SetProposalPolicy(ProposalPolicy{}); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.AutoAcceptProposal(pid, "mcp-architect"); !errors.Is(err, ErrPolicyRefused) || !strings.Contains(err.Error(), "off") {
		t.Fatalf("policy off: %v", err)
	}
	// Policy on but stage is design.
	if _, err := idx.SetProposalPolicy(ProposalPolicy{AutoAccept: true}); err != nil {
		t.Fatal(err)
	}
	if err := idx.SetStage("design"); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.AutoAcceptProposal(pid, "mcp-architect"); !errors.Is(err, ErrPolicyRefused) || !strings.Contains(err.Error(), "build") {
		t.Fatalf("design stage: %v", err)
	}
	if err := idx.SetStage("build"); err != nil {
		t.Fatal(err)
	}
	// A policy change made by another process is honoured immediately.
	other := reopen(t, dir)
	if _, err := other.SetProposalPolicy(ProposalPolicy{}); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.AutoAcceptProposal(pid, "mcp-architect"); !errors.Is(err, ErrPolicyRefused) {
		t.Fatalf("stale in-memory policy was used: %v", err)
	}
	if _, err := other.SetProposalPolicy(ProposalPolicy{AutoAccept: true}); err != nil {
		t.Fatal(err)
	}
	// Nothing was applied by any refusal.
	if got := idx.Nodes["svc"].Spec; got != "# svc\n" {
		t.Fatalf("spec changed: %q", got)
	}
	res, err := idx.AutoAcceptProposal(pid, "mcp-architect")
	if err != nil || !res.Accepted {
		t.Fatalf("%+v %v", res, err)
	}
	if got := idx.Nodes["svc"].Spec; got != "# svc v2\n" {
		t.Fatalf("spec %q", got)
	}
}

func TestAutoAcceptRefusesSelfAndAnonymous(t *testing.T) {
	idx, _ := autoFixture(t)
	pid := stageUpdate(t, idx, "mcp-same", "svc", UpdateInput{Spec: strp("# x\n")})
	if _, err := idx.AutoAcceptProposal(pid, "mcp-same"); !errors.Is(err, ErrPolicyRefused) || !strings.Contains(err.Error(), "staged") {
		t.Fatalf("self accept: %v", err)
	}
	if _, err := idx.AutoAcceptProposal(pid, ""); !errors.Is(err, ErrPolicyRefused) {
		t.Fatalf("anonymous accepter: %v", err)
	}
	if got := idx.Nodes["svc"].Spec; got != "# svc\n" {
		t.Fatal("self-accept applied the change")
	}
	// A different session may accept it.
	if res, err := idx.AutoAcceptProposal(pid, "mcp-other"); err != nil || !res.Accepted {
		t.Fatalf("%+v %v", res, err)
	}
	// A UI-staged proposal (empty origin) is fine for any session.
	ui := stageUpdate(t, idx, "", "svc", UpdateInput{Spec: strp("# y\n")})
	if res, err := idx.AutoAcceptProposal(ui, "mcp-same"); err != nil || !res.Accepted {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestAutoAcceptWritesAuditAndDigest(t *testing.T) {
	idx, dir := autoFixture(t)
	before := time.Now().UTC().Add(-time.Second).Format(time.RFC3339Nano)
	deps := []NodeID{"dep"}
	pid, err := idx.Stage(StageInput{Source: OpUpdateNode, Origin: "mcp-secret-origin", Ops: []StageOp{
		{Op: OpUpdateNode, NodeID: "svc", Update: UpdateInput{Spec: strp("# svc v2\n"), DependsOn: &deps}},
		{Op: OpCreateNode, Create: CreateInput{ID: "worker", Name: "Worker", Type: "module", ParentID: "platform", Spec: "# w\n", Implementation: []string{"src/worker/*.go"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := idx.AutoAcceptProposal(pid, "mcp-architect-origin")
	if err != nil || !res.Accepted || len(res.Applied) != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	if idx.Nodes["worker"] == nil || len(idx.Nodes["svc"].DependsOn) != 1 {
		t.Fatal("changes not applied")
	}
	entries, err := idx.ReadAudit(AuditQuery{})
	if err != nil || len(entries) != 1 {
		t.Fatalf("%v %+v", err, entries)
	}
	e := entries[0]
	if e.Actor != ActorArchitectAgent || e.ProposalID != pid || len(e.Ops) != 2 || e.OriginHash == "" || e.AccepterHash == "" {
		t.Fatalf("%+v", e)
	}
	for _, op := range e.Ops {
		if op.Rule == "" || op.DiffSummary == "" || op.Op == "" {
			t.Fatalf("op %+v", op)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".arch", ".proposals", AuditFile))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "mcp-secret-origin") || strings.Contains(string(raw), "mcp-architect-origin") {
		t.Fatalf("raw origin leaked into the audit log:\n%s", raw)
	}
	if strings.Count(string(raw), "\n") != 1 || !json.Valid(raw[:len(raw)-1]) {
		t.Fatalf("audit file is not one JSON line:\n%s", raw)
	}

	// A second auto-accept appends; the first line is untouched.
	pid2 := stageUpdate(t, idx, "mcp-writer", "dep", UpdateInput{Spec: strp("# dep v2\n")})
	if _, err := idx.AutoAcceptProposal(pid2, "mcp-architect-origin"); err != nil {
		t.Fatal(err)
	}
	raw2, _ := os.ReadFile(filepath.Join(dir, ".arch", ".proposals", AuditFile))
	if !strings.HasPrefix(string(raw2), string(raw)) || strings.Count(string(raw2), "\n") != 2 {
		t.Fatalf("log must be append-only:\n%s", raw2)
	}

	d, err := idx.AuditDigestFor(AuditQuery{Since: before, Actor: ActorArchitectAgent})
	if err != nil || d.Proposals != 2 || d.Ops != 3 {
		t.Fatalf("%+v %v", d, err)
	}
	counts := map[string]int{}
	for _, r := range d.ByRule {
		counts[r.Rule] = r.Count
	}
	if counts[RuleSpecText] != 2 || counts[RuleDependencyAdd] != 1 || counts[RuleCreateContainer] != 1 {
		t.Fatalf("%+v", d.ByRule)
	}
	later := time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)
	if got, _ := idx.ReadAudit(AuditQuery{Since: later}); len(got) != 0 {
		t.Fatalf("since filter: %+v", got)
	}
	if _, err := idx.ReadAudit(AuditQuery{Since: "yesterday"}); !errors.Is(err, ErrBadOperation) {
		t.Fatalf("bad since: %v", err)
	}
	if got, _ := idx.ReadAudit(AuditQuery{Actor: ActorHuman}); len(got) != 0 {
		t.Fatalf("actor filter: %+v", got)
	}
}

func TestAutoAcceptBlocksAndAppliesNothing(t *testing.T) {
	idx, dir := autoFixture(t)
	pid, err := idx.Stage(StageInput{Source: OpUpdateNode, Origin: "mcp-writer", Ops: []StageOp{
		{Op: OpUpdateNode, NodeID: "svc", Update: UpdateInput{Spec: strp("# would apply\n")}},
		{Op: OpUpdateNode, NodeID: "vault", Update: UpdateInput{Spec: strp("# vault v2\n")}},
		{Op: OpDeleteNode, NodeID: "dep"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := idx.AutoAcceptProposal(pid, "mcp-architect")
	if err != nil {
		t.Fatal(err)
	}
	if res.Accepted || len(res.Blocking) != 2 || len(res.Ops) != 3 {
		t.Fatalf("%+v", res)
	}
	rules := map[string]bool{}
	for _, b := range res.Blocking {
		rules[b.Rule] = true
		if b.Reason == "" {
			t.Fatalf("no reason: %+v", b)
		}
	}
	if !rules[BlockProtected] || !rules[BlockDelete] {
		t.Fatalf("%+v", res.Blocking)
	}
	if idx.Nodes["svc"].Spec != "# svc\n" {
		t.Fatal("the auto-eligible op was applied although the proposal is blocked")
	}
	if info, _ := idx.ProposalState(pid); info.Status != ProposalPending {
		t.Fatalf("proposal should stay pending: %+v", info)
	}
	if _, err := os.Stat(filepath.Join(dir, ".arch", ".proposals", AuditFile)); !os.IsNotExist(err) {
		t.Fatal("a blocked proposal must not write an audit entry")
	}
}

func TestAutoAcceptStageRefusalsForStructuralOps(t *testing.T) {
	idx, _ := autoFixture(t)
	check := func(name string, in StageInput, wantRule string) {
		t.Helper()
		in.Origin = "mcp-writer"
		pid, err := idx.Stage(in)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		res, err := idx.AutoAcceptProposal(pid, "mcp-architect")
		if err != nil || res.Accepted || len(res.Blocking) == 0 || res.Blocking[0].Rule != wantRule {
			t.Fatalf("%s: %+v %v", name, res, err)
		}
		if err := idx.DeleteProposal(pid); err != nil {
			t.Fatal(err)
		}
	}
	check("rename", StageInput{Source: OpRenameNode, Ops: []StageOp{{Op: OpRenameNode, NodeID: "dep", NewID: "dep-two"}}}, BlockRename)
	check("constraints", StageInput{Source: OpSetConstraints, Ops: []StageOp{{Op: OpSetConstraints, Constraints: []Constraint{{ID: "c", Text: "t"}}}}}, BlockConstraints)
	check("part", StageInput{Source: OpSetPart, Ops: []StageOp{{Op: OpSetPart, Part: &Part{ID: "p"}}}}, BlockPart)
	check("external node", StageInput{Source: OpCreateNode, Ops: []StageOp{{Op: OpCreateNode, Create: CreateInput{Name: "Stripe", Type: "external", ParentID: "platform", Spec: "# s\n"}}}}, BlockExternal)
	check("risk", StageInput{Source: OpUpdateNode, Ops: []StageOp{{Op: OpUpdateNode, NodeID: "svc", Update: UpdateInput{Risk: strp("high")}}}}, BlockSensitive)
}

func TestAutoAcceptBlocksNewEnforcedLintFinding(t *testing.T) {
	idx, _ := autoFixture(t)
	// A leaf without implementation or spec trips enforced rules under design-first, even
	// though every operation is individually auto-eligible.
	pid, err := idx.Stage(StageInput{Source: OpCreateNode, Origin: "mcp-writer", Ops: []StageOp{
		{Op: OpCreateNode, Create: CreateInput{ID: "bare", Name: "Bare", Type: "module", ParentID: "platform"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := idx.AutoAcceptProposal(pid, "mcp-architect")
	if err != nil {
		t.Fatal(err)
	}
	if res.Accepted || len(res.Blocking) != 1 || res.Blocking[0].Rule != BlockLint || !strings.Contains(res.Blocking[0].Reason, "leaf-implementation") {
		t.Fatalf("%+v", res)
	}
	if idx.Nodes["bare"] != nil {
		t.Fatal("applied despite the lint block")
	}
	// Pre-existing enforced findings do not block unrelated safe edits.
	if _, err := idx.Create(CreateInput{Name: "Legacy", Type: "module", ParentID: "platform"}); err != nil {
		t.Fatal(err)
	}
	ok := stageUpdate(t, idx, "mcp-writer", "svc", UpdateInput{Spec: strp("# fine\n")})
	if res, err := idx.AutoAcceptProposal(ok, "mcp-architect"); err != nil || !res.Accepted {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestAutoAcceptStaleOpIsBlocking(t *testing.T) {
	idx, _ := autoFixture(t)
	pid := stageUpdate(t, idx, "mcp-writer", "svc", UpdateInput{Spec: strp("# agent\n")})
	if err := idx.Update("svc", UpdateInput{Spec: strp("# human edit\n")}); err != nil {
		t.Fatal(err)
	}
	res, err := idx.AutoAcceptProposal(pid, "mcp-architect")
	if err != nil || res.Accepted || len(res.Blocking) != 1 || res.Blocking[0].Rule != BlockStale {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestAcceptAllAsHumanWritesAudit(t *testing.T) {
	idx, _ := autoFixture(t)
	pid := stageUpdate(t, idx, "mcp-writer", "vault", UpdateInput{Spec: strp("# vault v2\n")})
	res, err := idx.AcceptAllAs(pid, ActorHuman)
	if err != nil || len(res.Applied) != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	entries, _ := idx.ReadAudit(AuditQuery{Actor: ActorHuman})
	if len(entries) != 1 || entries[0].Ops[0].Rule != "human" || entries[0].Ops[0].Node != "vault" || entries[0].OriginHash == "" {
		t.Fatalf("%+v", entries)
	}
	// The default digest is the architect agent's, so a human accept is not in it.
	d, _ := idx.AuditDigestFor(AuditQuery{Actor: ActorArchitectAgent})
	if d.Proposals != 0 {
		t.Fatalf("%+v", d)
	}
}
