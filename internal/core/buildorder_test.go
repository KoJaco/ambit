package core

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func mkFull(t *testing.T, idx *Index, in CreateInput) {
	t.Helper()
	if in.Name == "" {
		in.Name = string(in.ID)
	}
	in.Type = "service"
	if _, err := idx.Create(in); err != nil {
		t.Fatalf("create %s: %v", in.ID, err)
	}
}

func orderIDs(r BuildOrderResult) []NodeID {
	out := []NodeID{}
	for _, it := range r.Order {
		out = append(out, it.ID)
	}
	return out
}

func mustOrder(t *testing.T, idx *Index, scope NodeID) BuildOrderResult {
	t.Helper()
	r, err := BuildOrder(idx, scope)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func eq(t *testing.T, what string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: got %v want %v", what, got, want)
	}
}

func TestBuildOrderDiamondAndWaves(t *testing.T) {
	idx, _ := newModel(t)
	mkNode(t, idx, "a")
	mkNode(t, idx, "b", "a")
	mkNode(t, idx, "c", "a")
	mkNode(t, idx, "d", "b", "c")
	r := mustOrder(t, idx, "")
	eq(t, "order", orderIDs(r), []NodeID{"a", "b", "c", "d"})
	eq(t, "waves", r.Waves, [][]NodeID{{"a"}, {"b", "c"}, {"d"}})
	eq(t, "d depends", r.Order[3].DependsOn, []NodeID{"b", "c"})
	eq(t, "d blocked", r.Order[3].BlockedBy, []NodeID{"b", "c"})
	eq(t, "a blocked", r.Order[0].BlockedBy, []NodeID{})
	if len(r.Cycles) != 0 || len(r.Unordered) != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestBuildOrderWavesAreLongestPath(t *testing.T) {
	idx, _ := newModel(t)
	mkNode(t, idx, "a")
	mkNode(t, idx, "b", "a")
	mkNode(t, idx, "c", "a", "b") // direct and transitive: wave 3, not 2
	mkNode(t, idx, "d")
	r := mustOrder(t, idx, "")
	eq(t, "waves", r.Waves, [][]NodeID{{"a", "d"}, {"b"}, {"c"}})
	eq(t, "c wave", r.Order[2].Wave, 3)
}

func TestBuildOrderTieBreakFollowsModelOrder(t *testing.T) {
	idx, _ := newModel(t)
	mkNode(t, idx, "zeta")
	mkNode(t, idx, "alpha")
	mkNode(t, idx, "mid", "zeta")
	r := mustOrder(t, idx, "")
	// Ready set after zeta is {alpha, mid}: alpha was created earlier than mid.
	eq(t, "order", orderIDs(r), []NodeID{"zeta", "alpha", "mid"})
	eq(t, "waves", r.Waves, [][]NodeID{{"zeta", "alpha"}, {"mid"}})
	again := mustOrder(t, idx, "")
	eq(t, "deterministic", orderIDs(again), orderIDs(r))
}

func TestBuildOrderCycleReportedAndExcluded(t *testing.T) {
	idx, dir := newModel(t)
	for _, id := range []string{"p", "q", "r", "s"} {
		mkNode(t, idx, id)
	}
	nodes := filepath.Join(dir, ".arch", "nodes")
	put := func(id, deps string) {
		writeFile(t, filepath.Join(nodes, id+".json"),
			`{"id":"`+id+`","name":"`+id+`","type":"service","status":"draft","depends_on":[`+deps+`]}`)
	}
	put("p", `"q"`)
	put("q", `"p"`)
	put("r", `"q"`)
	idx = reopen(t, dir)
	res := mustOrder(t, idx, "")
	eq(t, "order", orderIDs(res), []NodeID{"s"})
	eq(t, "cycles", res.Cycles, [][]NodeID{{"p", "q"}})
	eq(t, "unordered", res.Unordered, []NodeID{"p", "q", "r"})
	eq(t, "waves", res.Waves, [][]NodeID{{"s"}})
}

func TestBuildOrderScoped(t *testing.T) {
	idx, _ := newModel(t)
	mkFull(t, idx, CreateInput{ID: "outside"})
	mkFull(t, idx, CreateInput{ID: "svc"})
	mkFull(t, idx, CreateInput{ID: "x", ParentID: "svc"})
	mkFull(t, idx, CreateInput{ID: "y", ParentID: "svc", DependsOn: []NodeID{"x", "outside"}})
	mkFull(t, idx, CreateInput{ID: "z", DependsOn: []NodeID{"y"}})
	r := mustOrder(t, idx, "svc")
	eq(t, "order", orderIDs(r), []NodeID{"x", "y"})
	eq(t, "waves", r.Waves, [][]NodeID{{"x"}, {"y"}})
	// The out-of-scope dependency does not order y but still blocks it.
	eq(t, "blocked", r.Order[1].BlockedBy, []NodeID{"outside", "x"})
	if r.Scope != "svc" {
		t.Fatalf("scope %q", r.Scope)
	}
	// A buildable scope node orders itself.
	single := mustOrder(t, idx, "y")
	eq(t, "single", orderIDs(single), []NodeID{"y"})
	if _, err := BuildOrder(idx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestBuildOrderBlockedByIsStatusAware(t *testing.T) {
	idx, _ := newModel(t)
	mkFull(t, idx, CreateInput{ID: "a", Status: StatusDone})
	mkFull(t, idx, CreateInput{ID: "b", Status: StatusInProgress})
	mkFull(t, idx, CreateInput{ID: "c", DependsOn: []NodeID{"a", "b"}})
	r := mustOrder(t, idx, "")
	eq(t, "blocked", r.Order[2].BlockedBy, []NodeID{"b"})
	if !r.Order[0].Done || r.Order[1].Done {
		t.Fatalf("%+v", r.Order)
	}
	done := StatusDone
	if err := idx.Update("b", UpdateInput{Status: &done}); err != nil {
		t.Fatal(err)
	}
	r = mustOrder(t, idx, "")
	eq(t, "blocked after", r.Order[2].BlockedBy, []NodeID{})
}

func TestBuildOrderContainerDoneFromChildren(t *testing.T) {
	idx, _ := newModel(t)
	mkFull(t, idx, CreateInput{ID: "grp"})
	mkFull(t, idx, CreateInput{ID: "g1", ParentID: "grp"})
	mkFull(t, idx, CreateInput{ID: "g2", ParentID: "grp"})
	mkFull(t, idx, CreateInput{ID: "user", DependsOn: []NodeID{"grp"}})
	r := mustOrder(t, idx, "")
	// The container itself is not buildable; a dependency on it means all its parts.
	eq(t, "order", orderIDs(r), []NodeID{"g1", "g2", "user"})
	eq(t, "waves", r.Waves, [][]NodeID{{"g1", "g2"}, {"user"}})
	eq(t, "blocked", r.Order[2].BlockedBy, []NodeID{"grp"})

	done := StatusDone
	if err := idx.Update("g1", UpdateInput{Status: &done}); err != nil {
		t.Fatal(err)
	}
	r = mustOrder(t, idx, "")
	eq(t, "half done", r.Order[2].BlockedBy, []NodeID{"grp"})
	if err := idx.Update("g2", UpdateInput{Status: &done}); err != nil {
		t.Fatal(err)
	}
	r = mustOrder(t, idx, "")
	eq(t, "all done", r.Order[2].BlockedBy, []NodeID{})
}

func TestBuildOrderContainerWithImplementationIsBuildable(t *testing.T) {
	idx, _ := newModel(t)
	mkFull(t, idx, CreateInput{ID: "svc", Implementation: []string{"cmd/svc/**"}})
	mkFull(t, idx, CreateInput{ID: "mod", ParentID: "svc"})
	r := mustOrder(t, idx, "")
	eq(t, "order", orderIDs(r), []NodeID{"svc", "mod"})
	if r.Order[0].Done {
		t.Fatal("svc done with unfinished child")
	}
	// Done derives from children even though svc's own status is draft.
	done := StatusDone
	if err := idx.Update("mod", UpdateInput{Status: &done}); err != nil {
		t.Fatal(err)
	}
	r = mustOrder(t, idx, "")
	if !r.Order[0].Done || r.Order[0].Status != StatusDraft {
		t.Fatalf("%+v", r.Order[0])
	}
}

func TestBuildOrderPureContainerDependencyCascadesToChildren(t *testing.T) {
	idx, _ := newModel(t)
	mkFull(t, idx, CreateInput{ID: "api"})
	mkFull(t, idx, CreateInput{ID: "front", DependsOn: []NodeID{"api"}})
	mkFull(t, idx, CreateInput{ID: "ui", ParentID: "front"})
	r := mustOrder(t, idx, "")
	eq(t, "order", orderIDs(r), []NodeID{"api", "ui"})
	eq(t, "blocked", r.Order[1].BlockedBy, []NodeID{"api"})
	eq(t, "waves", r.Waves, [][]NodeID{{"api"}, {"ui"}})
}
