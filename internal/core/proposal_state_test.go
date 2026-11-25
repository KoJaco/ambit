package core

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func stageTwoCreates(t *testing.T, idx *Index) string {
	t.Helper()
	id, err := idx.Stage(StageInput{Source: SourceSeedModel, Summary: "two nodes", Ops: []StageOp{
		{Op: OpCreateNode, Create: CreateInput{ID: "alpha", Name: "Alpha", Type: "service", Spec: "# a\n"}},
		{Op: OpCreateNode, Create: CreateInput{ID: "beta", Name: "Beta", Type: "service", Spec: "# b\n"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestProposalStateFollowsTheLifecycle(t *testing.T) {
	idx, _ := newModel(t)
	id := stageTwoCreates(t, idx)
	st, err := idx.ProposalState(id)
	if err != nil || st.Status != ProposalPending || len(st.Operations) != 2 {
		t.Fatalf("pending: %+v %v", st, err)
	}
	if err := idx.AcceptOperation(id, 0, false); err != nil {
		t.Fatal(err)
	}
	if st, _ = idx.ProposalState(id); st.Status != ProposalPending || st.Operations[0].Status != "accepted" {
		t.Fatalf("half done: %+v", st)
	}
	if err := idx.RejectOperation(id, 1); err != nil {
		t.Fatal(err)
	}
	if st, err = idx.ProposalState(id); err != nil || st.Status != ProposalPartial || st.ResolvedAt == "" {
		t.Fatalf("resolved: %+v %v", st, err)
	}

	id2 := stageTwoCreatesNamed(t, idx, "gamma", "delta")
	if res, err := idx.AcceptAll(id2); err != nil || len(res.Applied) != 2 {
		t.Fatalf("accept all: %+v %v", res, err)
	}
	if st, _ = idx.ProposalState(id2); st.Status != ProposalAccepted {
		t.Fatalf("accepted: %+v", st)
	}
	id3 := stageTwoCreatesNamed(t, idx, "eps", "zeta")
	if err := idx.RejectAll(id3); err != nil {
		t.Fatal(err)
	}
	if st, _ = idx.ProposalState(id3); st.Status != ProposalRejected {
		t.Fatalf("rejected: %+v", st)
	}
	id4 := stageTwoCreatesNamed(t, idx, "eta", "theta")
	if err := idx.DeleteProposal(id4); err != nil {
		t.Fatal(err)
	}
	if st, _ = idx.ProposalState(id4); st.Status != ProposalWithdrawn {
		t.Fatalf("withdrawn: %+v", st)
	}
	if _, err := idx.ProposalState("p-20200101-0000-abcd"); !errors.Is(err, ErrProposalNotFound) {
		t.Fatalf("unknown id: %v", err)
	}
}

func stageTwoCreatesNamed(t *testing.T, idx *Index, a, b string) string {
	t.Helper()
	id, err := idx.Stage(StageInput{Source: SourceSeedModel, Ops: []StageOp{
		{Op: OpCreateNode, Create: CreateInput{ID: NodeID(a), Name: a, Type: "service", Spec: "# x\n"}},
		{Op: OpCreateNode, Create: CreateInput{ID: NodeID(b), Name: b, Type: "service", Spec: "# y\n"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestWaitForProposalReturnsWhenResolved(t *testing.T) {
	idx, _ := newModel(t)
	id := stageTwoCreates(t, idx)
	go func() {
		time.Sleep(60 * time.Millisecond)
		_, _ = idx.AcceptAll(id)
	}()
	st, timedOut, err := idx.WaitForProposal(context.Background(), id, 5*time.Second, 10*time.Millisecond)
	if err != nil || timedOut || st.Status != ProposalAccepted {
		t.Fatalf("wait: %+v timedOut=%v err=%v", st, timedOut, err)
	}
	id2 := stageTwoCreatesNamed(t, idx, "p", "q")
	st, timedOut, err = idx.WaitForProposal(context.Background(), id2, 40*time.Millisecond, 10*time.Millisecond)
	if err != nil || !timedOut || st.Status != ProposalPending {
		t.Fatalf("timeout: %+v timedOut=%v err=%v", st, timedOut, err)
	}
}

func TestRestageRebuildsAcceptedOperations(t *testing.T) {
	idx, dir := newModel(t)
	id := stageTwoCreates(t, idx)
	if err := idx.AcceptOperation(id, 0, false); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Restage(id, []int{1}); !errors.Is(err, ErrBadOperation) {
		t.Fatalf("restaging a pending op: %v", err)
	}
	// Undo: remove the node the way the UI does, then put the proposal back.
	if err := idx.Delete("alpha"); err != nil {
		t.Fatal(err)
	}
	newID, err := idx.Restage(id, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	st, err := idx.ProposalState(newID)
	if err != nil || st.Status != ProposalPending || len(st.Operations) != 1 || st.Operations[0].NodeID != "alpha" || st.Operations[0].Stale {
		t.Fatalf("restaged: %+v %v", st, err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
	if err := idx.AcceptOperation(newID, 0, false); err != nil {
		t.Fatalf("accept after restage: %v", err)
	}
	if idx.Nodes["alpha"] == nil {
		t.Fatal("alpha not restored")
	}
	// A finished proposal can be restaged too.
	if err := idx.Delete("alpha"); err != nil {
		t.Fatal(err)
	}
	again, err := idx.Restage(newID, nil)
	if err != nil || again == newID {
		t.Fatalf("restage from resolved: %q %v", again, err)
	}
}

func TestSeedCreatesMayReferenceLaterCreates(t *testing.T) {
	idx, _ := newModel(t)
	id, err := idx.Stage(StageInput{Source: SourceSeedModel, Ops: []StageOp{
		{Op: OpCreateNode, Create: CreateInput{ID: "child", Name: "Child", Type: "service", ParentID: "zone", DependsOn: []NodeID{"base"}, Spec: "# c\n"}},
		{Op: OpCreateNode, Create: CreateInput{ID: "zone", Name: "Zone", Type: "container", Spec: "# z\n"}},
		{Op: OpCreateNode, Create: CreateInput{ID: "base", Name: "Base", Type: "service", Spec: "# b\n"}},
	}})
	if err != nil {
		t.Fatalf("forward references rejected: %v", err)
	}
	res, err := idx.AcceptAll(id)
	if err != nil || len(res.Applied) != 3 || len(res.Failed) != 0 || len(res.Skipped) != 0 {
		t.Fatalf("accept all: %+v %v", res, err)
	}
	if idx.Nodes["child"].ParentID != "zone" {
		t.Fatalf("parent: %+v", idx.Nodes["child"])
	}
	// A real cycle is still rejected.
	if _, err := idx.Stage(StageInput{Source: SourceSeedModel, Ops: []StageOp{
		{Op: OpCreateNode, Create: CreateInput{ID: "m", Name: "M", Type: "service", DependsOn: []NodeID{"n"}}},
		{Op: OpCreateNode, Create: CreateInput{ID: "n", Name: "N", Type: "service", DependsOn: []NodeID{"m"}}},
	}}); err == nil {
		t.Fatal("dependency cycle accepted")
	}
}
