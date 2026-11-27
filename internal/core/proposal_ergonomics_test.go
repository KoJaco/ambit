package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func mustCreate(t *testing.T, idx *Index, in CreateInput) NodeID {
	t.Helper()
	id, err := idx.Create(in)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func acceptAllOK(t *testing.T, idx *Index, pid string) {
	t.Helper()
	res, err := idx.AcceptAll(pid)
	if err != nil || len(res.Failed) != 0 || len(res.Skipped) != 0 {
		t.Fatalf("accept all %s: %+v %v", pid, res, err)
	}
}

func TestSetRelationshipUpsertsOnFromToLabel(t *testing.T) {
	idx, _ := newModel(t)
	a := mustCreate(t, idx, CreateInput{Name: "A", Type: "service"})
	b := mustCreate(t, idx, CreateInput{Name: "B", Type: "service"})
	c := mustCreate(t, idx, CreateInput{Name: "C", Type: "service"})

	// Twice in one proposal: one edge.
	pid, err := idx.Stage(StageInput{Source: SourceSeedModel, Ops: []StageOp{
		{Op: OpSetRelationship, From: a, To: b, Label: "calls", Kind: KindSync},
		{Op: OpSetRelationship, From: a, To: b, Label: "calls", Kind: KindSync},
	}})
	if err != nil {
		t.Fatal(err)
	}
	acceptAllOK(t, idx, pid)
	if len(idx.Relationships) != 1 {
		t.Fatalf("want 1 edge, got %+v", idx.Relationships)
	}
	id := idx.Relationships[0].ID

	// Again in a later proposal, without a kind: same edge, kind kept.
	pid, err = idx.Stage(StageInput{Source: OpSetRelationship, Ops: []StageOp{{Op: OpSetRelationship, From: a, To: b, Label: "calls"}}})
	if err != nil {
		t.Fatal(err)
	}
	acceptAllOK(t, idx, pid)
	if len(idx.Relationships) != 1 || idx.Relationships[0].ID != id || idx.Relationships[0].Kind != KindSync {
		t.Fatalf("upsert changed edge: %+v", idx.Relationships)
	}

	// A different label is a different edge.
	pid, err = idx.Stage(StageInput{Source: OpSetRelationship, Ops: []StageOp{{Op: OpSetRelationship, From: a, To: b, Label: "notifies", Kind: KindAsync}}})
	if err != nil {
		t.Fatal(err)
	}
	acceptAllOK(t, idx, pid)
	if len(idx.Relationships) != 2 {
		t.Fatalf("want 2 edges, got %+v", idx.Relationships)
	}

	// Changing endpoints through the id updates in place.
	pid, err = idx.Stage(StageInput{Source: OpSetRelationship, Ops: []StageOp{{Op: OpSetRelationship, RelID: id, From: a, To: c, Label: "calls", Kind: KindSync}}})
	if err != nil {
		t.Fatal(err)
	}
	acceptAllOK(t, idx, pid)
	rel, _ := idx.relByID(id)
	if len(idx.Relationships) != 2 || rel == nil || rel.To != c {
		t.Fatalf("endpoint change: %+v", idx.Relationships)
	}
}

func TestDirectSetRelationshipUpserts(t *testing.T) {
	idx, _ := newModel(t)
	a := mustCreate(t, idx, CreateInput{Name: "A", Type: "service"})
	b := mustCreate(t, idx, CreateInput{Name: "B", Type: "service"})
	first, err := idx.SetRelationship(SetRelationshipInput{From: a, To: b, Label: "calls", Kind: KindData})
	if err != nil {
		t.Fatal(err)
	}
	second, err := idx.SetRelationship(SetRelationshipInput{From: a, To: b, Label: "calls"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || len(idx.Relationships) != 1 || second.Kind != KindData {
		t.Fatalf("%+v %+v %+v", first, second, idx.Relationships)
	}
}

func TestRenameNodeRewritesReferences(t *testing.T) {
	idx, dir := newModel(t)
	p := mustCreate(t, idx, CreateInput{ID: "platform", Name: "Platform", Type: "boundary", Spec: "platform spec\n", Implementation: []string{"platform/**"}})
	child := mustCreate(t, idx, CreateInput{ID: "child", Name: "Child", Type: "service", ParentID: p})
	dep := mustCreate(t, idx, CreateInput{ID: "dep", Name: "Dep", Type: "service", DependsOn: []NodeID{p}})
	other := mustCreate(t, idx, CreateInput{ID: "other", Name: "Other", Type: "service"})
	rel, err := idx.SetRelationship(SetRelationshipInput{From: p, To: other, Label: "calls"})
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.SetAssignment(p, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if err := idx.PutLayout("platform", Layout{Positions: []Position{{ID: "child", X: 1, Y: 2}}}); err != nil {
		t.Fatal(err)
	}
	if err := idx.PutLayout(RootLayoutKey, Layout{
		Positions:   []Position{{ID: "platform", X: 3, Y: 4}, {ID: "dep", X: 5, Y: 6}},
		Ports:       map[string]NodePorts{"platform": {Left: 1, Right: 2}},
		Attachments: map[string]EdgeAttachment{"calls": {Source: "s", Target: "t"}},
	}); err != nil {
		t.Fatal(err)
	}

	pid, err := idx.Stage(StageInput{Source: OpRenameNode, Ops: []StageOp{{Op: OpRenameNode, NodeID: p, NewID: "core"}}})
	if err != nil {
		t.Fatal(err)
	}
	acceptAllOK(t, idx, pid)

	got := reopen(t, dir)
	if got.Nodes["platform"] != nil || got.Nodes["core"] == nil {
		t.Fatalf("nodes %v", got.Order)
	}
	if got.Nodes["core"].Spec != "platform spec\n" || got.Nodes["core"].Implementation[0] != "platform/**" {
		t.Fatalf("node body %+v", got.Nodes["core"])
	}
	if got.Nodes[child].ParentID != "core" {
		t.Fatalf("parent_id %q", got.Nodes[child].ParentID)
	}
	if len(got.Nodes[dep].DependsOn) != 1 || got.Nodes[dep].DependsOn[0] != "core" {
		t.Fatalf("depends_on %v", got.Nodes[dep].DependsOn)
	}
	r, _ := got.relByID(rel.ID)
	if r == nil || r.From != "core" || r.To != other {
		t.Fatalf("relationship %+v", r)
	}
	if got.Assignment == nil || got.Assignment.NodeID != "core" {
		t.Fatalf("assignment %+v", got.Assignment)
	}
	for _, name := range []string{"platform.json", "platform.md"} {
		if _, err := os.Stat(filepath.Join(dir, ".arch", "nodes", name)); !os.IsNotExist(err) {
			t.Fatalf("%s remains", name)
		}
	}
	if l, _ := got.Layout("core"); len(l.Positions) != 1 {
		t.Fatalf("container layout not moved: %+v", l)
	}
	if l, _ := got.Layout("platform"); len(l.Positions) != 0 {
		t.Fatalf("old layout key remains: %+v", l)
	}
	root, _ := got.Layout(RootLayoutKey)
	if root.Positions[0].ID != "core" || root.Positions[1].ID != "dep" || root.Positions[0].X != 3 {
		t.Fatalf("root layout %+v", root)
	}
	if _, ok := root.Ports["core"]; !ok || len(root.Ports) != 1 {
		t.Fatalf("ports %+v", root.Ports)
	}
	if len(got.Warnings()) != 0 {
		t.Fatalf("warnings %v", got.Warnings())
	}
}

func TestRenameNodeRejectsTakenReservedAndMissing(t *testing.T) {
	idx, _ := newModel(t)
	mustCreate(t, idx, CreateInput{ID: "a", Name: "A", Type: "service"})
	mustCreate(t, idx, CreateInput{ID: "b", Name: "B", Type: "service"})
	mustCreate(t, idx, CreateInput{ID: "c", Name: "C", Type: "service"})
	if _, err := idx.SetRelationship(SetRelationshipInput{From: "a", To: "b", Label: "calls", Kind: KindSync}); err != nil {
		t.Fatal(err)
	}
	stage := func(old, nw NodeID) error {
		_, err := idx.Stage(StageInput{Source: OpRenameNode, Ops: []StageOp{{Op: OpRenameNode, NodeID: old, NewID: nw}}})
		return err
	}
	if err := stage("a", "b"); !errors.Is(err, ErrExists) {
		t.Fatalf("taken: %v", err)
	}
	if err := stage("zzz", "q"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if err := stage("a", "Bad_ID"); !errors.Is(err, ErrBadOperation) {
		t.Fatalf("bad slug: %v", err)
	}
	if err := stage("a", "a"); !errors.Is(err, ErrBadOperation) {
		t.Fatalf("same id: %v", err)
	}
	if err := stage("a", "calls"); !errors.Is(err, ErrExists) {
		t.Fatalf("relationship id: %v", err)
	}
	// Reserved by another pending proposal, create or rename.
	if _, err := idx.Stage(StageInput{Source: OpCreateNode, Ops: []StageOp{{Op: OpCreateNode, Create: CreateInput{ID: "fresh", Name: "Fresh", Type: "service"}}}}); err != nil {
		t.Fatal(err)
	}
	if err := stage("a", "fresh"); !errors.Is(err, ErrExists) {
		t.Fatalf("reserved by create: %v", err)
	}
	if err := stage("a", "renamed"); err != nil {
		t.Fatal(err)
	}
	if err := stage("c", "renamed"); !errors.Is(err, ErrExists) {
		t.Fatalf("reserved by rename: %v", err)
	}
}

func TestRenameAppliesLastInProposal(t *testing.T) {
	idx, _ := newModel(t)
	mustCreate(t, idx, CreateInput{ID: "a", Name: "A", Type: "service"})
	mustCreate(t, idx, CreateInput{ID: "b", Name: "B", Type: "service"})
	spec := "edited"
	deps := []NodeID{"a"}
	// The rename is written first, but the update of "a" and the dependency on "a" still use the old id.
	pid, err := idx.Stage(StageInput{Source: SourceSeedModel, Ops: []StageOp{
		{Op: OpRenameNode, NodeID: "a", NewID: "alpha"},
		{Op: OpUpdateNode, NodeID: "a", Update: UpdateInput{Spec: &spec}},
		{Op: OpUpdateNode, NodeID: "b", Update: UpdateInput{DependsOn: &deps}},
		{Op: OpSetRelationship, From: "a", To: "b", Label: "calls", Kind: KindSync},
	}})
	if err != nil {
		t.Fatal(err)
	}
	acceptAllOK(t, idx, pid)
	if idx.Nodes["a"] != nil || idx.Nodes["alpha"] == nil || idx.Nodes["alpha"].Spec != "edited" {
		t.Fatalf("nodes: %v", idx.Order)
	}
	if idx.Nodes["b"].DependsOn[0] != "alpha" || idx.Relationships[0].From != "alpha" {
		t.Fatalf("references not rewritten: %+v %+v", idx.Nodes["b"].DependsOn, idx.Relationships)
	}
}

func TestRenameRollsBackWhenAcceptFails(t *testing.T) {
	idx, dir := newModel(t)
	mustCreate(t, idx, CreateInput{ID: "a", Name: "A", Type: "service"})
	mustCreate(t, idx, CreateInput{ID: "b", Name: "B", Type: "service", ParentID: "a"})
	pid, err := idx.Stage(StageInput{Source: OpRenameNode, Ops: []StageOp{{Op: OpRenameNode, NodeID: "a", NewID: "alpha"}}})
	if err != nil {
		t.Fatal(err)
	}
	applyHook = func(step string) error {
		if step == "index" {
			return errors.New("boom")
		}
		return nil
	}
	defer func() { applyHook = nil }()
	if err := idx.AcceptOperation(pid, 0, false); err == nil {
		t.Fatal("expected failure")
	}
	applyHook = nil
	got := reopen(t, dir)
	if got.Nodes["a"] == nil || got.Nodes["alpha"] != nil || got.Nodes["b"].ParentID != "a" {
		t.Fatalf("not restored: %v", got.Order)
	}
}

func TestReparentAndDependsOnInOneProposalUseFinalTree(t *testing.T) {
	idx, _ := newModel(t)
	mustCreate(t, idx, CreateInput{ID: "parent", Name: "Parent", Type: "boundary"})
	mustCreate(t, idx, CreateInput{ID: "child", Name: "Child", Type: "service", ParentID: "parent"})
	mustCreate(t, idx, CreateInput{ID: "elsewhere", Name: "Elsewhere", Type: "boundary"})
	// parent depends on its own child: a cycle while child is inside parent.
	deps := []NodeID{"child"}
	out := NodeID("elsewhere")
	// Written dependency first, reparent second: must still be accepted.
	pid, err := idx.Stage(StageInput{Source: SourceSeedModel, Ops: []StageOp{
		{Op: OpUpdateNode, NodeID: "parent", Update: UpdateInput{DependsOn: &deps}},
		{Op: OpUpdateNode, NodeID: "child", Update: UpdateInput{Parent: &out}},
	}})
	if err != nil {
		t.Fatalf("reparent + depends_on in one proposal: %v", err)
	}
	acceptAllOK(t, idx, pid)
	if idx.Nodes["child"].ParentID != "elsewhere" || len(idx.Nodes["parent"].DependsOn) != 1 {
		t.Fatalf("final state: %+v %+v", idx.Nodes["child"], idx.Nodes["parent"])
	}

	// A change that is a cycle in the final tree is still refused.
	back := NodeID("parent")
	dep2 := []NodeID{"parent"}
	_, err = idx.Stage(StageInput{Source: SourceSeedModel, Ops: []StageOp{
		{Op: OpUpdateNode, NodeID: "child", Update: UpdateInput{Parent: &back}},
		{Op: OpUpdateNode, NodeID: "elsewhere", Update: UpdateInput{DependsOn: &dep2}},
	}})
	if err == nil {
		t.Fatal("cycle in the final tree should be rejected")
	}
}

func TestOrderOpsKeepsSlotsAndMovesRenamesLast(t *testing.T) {
	p := NodeID("x")
	ops := []StageOp{
		{Op: OpUpdateNode, NodeID: "a", Update: UpdateInput{Spec: new(string)}},
		{Op: OpDeleteNode, NodeID: "d"},
		{Op: OpRenameNode, NodeID: "r", NewID: "r2"},
		{Op: OpUpdateNode, NodeID: "b", Update: UpdateInput{Parent: &p}},
	}
	got := orderOps(ops)
	want := []string{"b", "d", "a", "r"}
	for i, op := range got {
		if string(op.NodeID) != want[i] {
			t.Fatalf("position %d: %s, want %s (%v)", i, op.NodeID, want[i], got)
		}
	}
}

func TestUpdateErrorsNameTheRightTool(t *testing.T) {
	idx, _ := newModel(t)
	mustCreate(t, idx, CreateInput{ID: "a", Name: "A", Type: "service"})
	done := StatusDone
	_, err := idx.Stage(StageInput{Source: OpUpdateNode, Ops: []StageOp{{Op: OpUpdateNode, NodeID: "a", Update: UpdateInput{Status: &done}}}})
	if !errors.Is(err, ErrBadOperation) || !strings.Contains(err.Error(), "update_node_status") {
		t.Fatalf("status: %v", err)
	}
	rid := RelID("nope")
	_, err = idx.Stage(StageInput{Source: OpUpdateNode, Ops: []StageOp{{Op: OpUpdateNode, NodeID: "a", Update: UpdateInput{Relationship: &rid}}}})
	if !errors.Is(err, ErrRelationshipNotFound) || !strings.Contains(err.Error(), "set_relationship") {
		t.Fatalf("relationship_id: %v", err)
	}
	_, err = idx.Stage(StageInput{Source: OpSetRelationship, Ops: []StageOp{{Op: OpSetRelationship, RelID: "nope", From: "a", To: "a"}}})
	if !errors.Is(err, ErrRelationshipNotFound) {
		t.Fatalf("set_relationship id: %v", err)
	}
}

func stageSpecEdit(t *testing.T, idx *Index, id NodeID, spec string, in StageInput) string {
	t.Helper()
	in.Source = OpUpdateNode
	in.Ops = []StageOp{{Op: OpUpdateNode, NodeID: id, Update: UpdateInput{Spec: &spec}}}
	pid, err := idx.Stage(in)
	if err != nil {
		t.Fatal(err)
	}
	return pid
}

func TestWithdrawProposalOnlyBySameOrigin(t *testing.T) {
	idx, _ := newModel(t)
	mustCreate(t, idx, CreateInput{ID: "a", Name: "A", Type: "service"})
	pid := stageSpecEdit(t, idx, "a", "x", StageInput{Origin: "mcp-1"})
	if err := idx.WithdrawProposal(pid, "mcp-2"); !errors.Is(err, ErrBadOperation) {
		t.Fatalf("other origin: %v", err)
	}
	if err := idx.WithdrawProposal(pid, ""); !errors.Is(err, ErrBadOperation) {
		t.Fatalf("empty origin: %v", err)
	}
	uiPid := stageSpecEdit(t, idx, "a", "y", StageInput{})
	if err := idx.WithdrawProposal(uiPid, ""); !errors.Is(err, ErrBadOperation) {
		t.Fatalf("UI proposal must not be withdrawable: %v", err)
	}
	if err := idx.WithdrawProposal(pid, "mcp-1"); err != nil {
		t.Fatal(err)
	}
	st, err := idx.ProposalState(pid)
	if err != nil || st.Status != ProposalWithdrawn {
		t.Fatalf("state %+v %v", st, err)
	}
	list, _ := idx.ListProposals()
	if len(list) != 1 || list[0].ID != uiPid {
		t.Fatalf("pending %+v", list)
	}
	if err := idx.WithdrawProposal(pid, "mcp-1"); !errors.Is(err, ErrProposalNotFound) {
		t.Fatalf("second withdraw: %v", err)
	}
}

func TestSupersedesWithdrawsPredecessorAndFreesIDs(t *testing.T) {
	idx, _ := newModel(t)
	create := func(in StageInput) (string, error) {
		in.Source = SourceSeedModel
		in.Ops = []StageOp{{Op: OpCreateNode, Create: CreateInput{ID: "orders", Name: "Orders", Type: "service"}}}
		return idx.Stage(in)
	}
	first, err := create(StageInput{Origin: "mcp-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := create(StageInput{Origin: "mcp-1"}); !errors.Is(err, ErrExists) {
		t.Fatalf("id should be reserved: %v", err)
	}
	if _, err := create(StageInput{Origin: "mcp-2", Supersedes: first}); !errors.Is(err, ErrBadOperation) {
		t.Fatalf("other origin must not supersede: %v", err)
	}
	second, err := create(StageInput{Origin: "mcp-1", Supersedes: first, ChangeSet: "orders-v2"})
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := idx.ProposalState(first); st.Status != ProposalWithdrawn {
		t.Fatalf("predecessor %+v", st)
	}
	list, _ := idx.ListProposals()
	if len(list) != 1 || list[0].ID != second || list[0].ChangeSet != "orders-v2" {
		t.Fatalf("pending %+v", list)
	}
	// A failed staging leaves the predecessor in place.
	if _, err := idx.Stage(StageInput{Source: OpDeleteNode, Origin: "mcp-1", Supersedes: second, Ops: []StageOp{{Op: OpDeleteNode, NodeID: "ghost"}}}); err == nil {
		t.Fatal("expected error")
	}
	if st, _ := idx.ProposalState(second); st.Status != ProposalPending {
		t.Fatalf("predecessor must survive a failed stage: %+v", st)
	}
}

func TestDependsOnProposalValidatesAgainstPendingState(t *testing.T) {
	idx, _ := newModel(t)
	first, err := idx.Stage(StageInput{Source: SourceSeedModel, Ops: []StageOp{
		{Op: OpCreateNode, Create: CreateInput{ID: "svc", Name: "Svc", Type: "service"}},
		{Op: OpCreateNode, Create: CreateInput{ID: "db", Name: "Db", Type: "service"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	ops := []StageOp{
		{Op: OpSetRelationship, From: "svc", To: "db", Label: "reads", Kind: KindData},
		{Op: OpUpdateNode, NodeID: "svc", Update: UpdateInput{DependsOn: &[]NodeID{"db"}}},
	}
	if _, err := idx.Stage(StageInput{Source: SourceSeedModel, Ops: ops}); err == nil {
		t.Fatal("without depends_on_proposal the pending nodes do not exist")
	}
	second, err := idx.Stage(StageInput{Source: SourceSeedModel, Ops: ops, DependsOnProposal: first})
	if err != nil {
		t.Fatalf("depends_on_proposal: %v", err)
	}
	if err := idx.AcceptOperation(second, 0, false); !errors.Is(err, ErrStale) || !strings.Contains(err.Error(), first) {
		t.Fatalf("accept while predecessor pending: %v", err)
	}
	if res, err := idx.AcceptAll(second); !errors.Is(err, ErrStale) {
		t.Fatalf("accept all while predecessor pending: %+v %v", res, err)
	}
	list, _ := idx.ListProposals()
	for _, p := range list {
		if p.ID == second && p.DependsOnProposal != first {
			t.Fatalf("summary %+v", p)
		}
	}
	acceptAllOK(t, idx, first)
	acceptAllOK(t, idx, second)
	if len(idx.Relationships) != 1 || len(idx.Nodes["svc"].DependsOn) != 1 {
		t.Fatalf("not applied: %+v", idx.Relationships)
	}
}

func TestChangeSetValidatedAndExposed(t *testing.T) {
	idx, _ := newModel(t)
	mustCreate(t, idx, CreateInput{ID: "a", Name: "A", Type: "service"})
	spec := "x"
	bad := StageInput{Source: OpUpdateNode, ChangeSet: "Not A Slug", Ops: []StageOp{{Op: OpUpdateNode, NodeID: "a", Update: UpdateInput{Spec: &spec}}}}
	if _, err := idx.Stage(bad); !errors.Is(err, ErrBadOperation) {
		t.Fatalf("bad change_set: %v", err)
	}
	pid := stageSpecEdit(t, idx, "a", "x", StageInput{ChangeSet: "auth-rework"})
	diff, err := idx.ProposalDiff(pid)
	if err != nil || diff.ChangeSet != "auth-rework" {
		t.Fatalf("diff %+v %v", diff, err)
	}
	st, _ := idx.ProposalState(pid)
	if st.ChangeSet != "auth-rework" {
		t.Fatalf("state %+v", st)
	}
	restaged := ""
	acceptAllOK(t, idx, pid)
	if restaged, err = idx.Restage(pid, nil); err != nil {
		t.Fatal(err)
	}
	if st, _ := idx.ProposalState(restaged); st.ChangeSet != "auth-rework" {
		t.Fatalf("restage lost change_set: %+v", st)
	}
}

func TestRenameNodeNewIDInListAndDiff(t *testing.T) {
	idx, _ := newModel(t)
	a := mustCreate(t, idx, CreateInput{ID: "a", Name: "A", Type: "service"})
	pid, err := idx.Stage(StageInput{Source: OpRenameNode, Ops: []StageOp{{Op: OpRenameNode, NodeID: a, NewID: "alpha"}}})
	if err != nil {
		t.Fatal(err)
	}
	list, err := idx.ListProposals()
	if err != nil || len(list) != 1 || list[0].Operations[0].NewID != "alpha" {
		t.Fatalf("list %+v err %v", list, err)
	}
	d, err := idx.ProposalDiff(pid)
	if err != nil || d.Operations[0].NewID != "alpha" {
		t.Fatalf("diff %+v err %v", d, err)
	}
}
