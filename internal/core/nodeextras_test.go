package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNodeExtrasRoundTripAndUnknownKeysPreserved(t *testing.T) {
	idx, dir := newModel(t)
	mkNode(t, idx, "target")
	id := mustCreate(t, idx, CreateInput{
		ID: "spike", Name: "Spike", Type: "service",
		NodeExtras: NodeExtras{
			External:     true,
			HumanGates:   []HumanGate{{Kind: "Credential", Note: " api key "}},
			Deliverables: []string{"./docs/x.md", "docs/x.md", "docs/y.md"},
			References:   []string{"docs/contracts/a.md"},
			Shared:       true,
			Milestones:   []Milestone{{ID: "m1", Label: "first"}},
			Gates:        []NodeID{"target"},
		},
	})
	n := idx.Nodes[id]
	if n.HumanGates[0].Kind != "credential" || n.HumanGates[0].Note != "api key" {
		t.Fatalf("gates %+v", n.HumanGates)
	}
	eq(t, "deliverables", n.Deliverables, []string{"docs/x.md", "docs/y.md"})
	eq(t, "milestone status default", n.Milestones[0].Status, MilestonePending)

	// An unknown key written by a newer version survives an update.
	path := filepath.Join(dir, ".arch", "nodes", "spike.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), "{\n", "{\n    \"future\": 7,\n", 1))
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	idx = reopen(t, dir)
	names := []string{"Renamed"}
	if err := idx.Update("spike", UpdateInput{Name: &names[0]}); err != nil {
		t.Fatal(err)
	}
	got := reopen(t, dir).Nodes["spike"]
	if !got.External || !got.Shared || len(got.Gates) != 1 || got.Unknown["future"] == nil || len(got.HumanGates) != 1 {
		t.Fatalf("round trip lost data: %+v", got)
	}
	out := readFile(t, path)
	if !strings.Contains(out, `"human_gates"`) || !strings.Contains(out, `"milestones"`) {
		t.Fatal(out)
	}
}

func TestNodeExtrasValidation(t *testing.T) {
	idx, _ := newModel(t)
	mkNode(t, idx, "a")
	bad := []NodeExtras{
		{HumanGates: []HumanGate{{Kind: "magic"}}},
		{Deliverables: []string{"/etc/passwd"}},
		{Deliverables: []string{"../x"}},
		{Deliverables: []string{"a/../../x"}},
		{References: []string{"C:/x"}},
		{Milestones: []Milestone{{ID: "Bad Id"}}},
		{Milestones: []Milestone{{ID: "m", Status: "weird"}}},
		{Milestones: []Milestone{{ID: "m"}, {ID: "m"}}},
	}
	for i, e := range bad {
		_, err := idx.Create(CreateInput{ID: NodeID("x" + string(rune('a'+i))), Name: "X", Type: "service", NodeExtras: e})
		if !errors.Is(err, ErrBadNodeMeta) {
			t.Fatalf("case %d: %v", i, err)
		}
	}
	many := make([]string, MaxDeliverables+1)
	for i := range many {
		many[i] = "f" + string(rune('a'+i%26)) + string(rune('a'+i/26))
	}
	if _, err := idx.Create(CreateInput{ID: "big", Name: "X", Type: "service", NodeExtras: NodeExtras{Deliverables: many}}); !errors.Is(err, ErrBadNodeMeta) {
		t.Fatalf("cap: %v", err)
	}
}

func TestGatesValidatedAndCountAsDependencies(t *testing.T) {
	idx, _ := newModel(t)
	mkNode(t, idx, "app")
	mkNode(t, idx, "lib")
	if _, err := idx.Create(CreateInput{ID: "s", Name: "S", Type: "service", NodeExtras: NodeExtras{Gates: []NodeID{"ghost"}}}); !errors.Is(err, ErrBadDependency) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := idx.Create(CreateInput{ID: "s", Name: "S", Type: "service", NodeExtras: NodeExtras{Gates: []NodeID{"s"}}}); !errors.Is(err, ErrBadDependency) {
		t.Fatalf("self: %v", err)
	}
	mustCreate(t, idx, CreateInput{ID: "spike", Name: "Spike", Type: "service", NodeExtras: NodeExtras{Gates: []NodeID{"app"}}})
	r := mustOrder(t, idx, "")
	eq(t, "order", orderIDs(r), []NodeID{"lib", "spike", "app"})
	eq(t, "waves", r.Waves, [][]NodeID{{"lib", "spike"}, {"app"}})
	eq(t, "app blocked_by", r.Order[2].BlockedBy, []NodeID{"spike"})

	// The spike cannot depend on what it gates.
	deps := []NodeID{"app"}
	if err := idx.Update("spike", UpdateInput{DependsOn: &deps}); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("spike depends on gated: %v", err)
	}
	// spike -> lib -> app -> (gate) spike is an indirect cycle.
	libDeps := []NodeID{"app"}
	if err := idx.Update("lib", UpdateInput{DependsOn: &libDeps}); err != nil {
		t.Fatal(err)
	}
	spikeDeps := []NodeID{"lib"}
	if err := idx.Update("spike", UpdateInput{DependsOn: &spikeDeps}); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("indirect cycle: %v", err)
	}
	// Making lib depend on the spike's own gate target while the spike gates lib is fine.
	gates := []NodeID{"app", "lib"}
	if err := idx.Update("spike", UpdateInput{NodeExtrasPatch: NodeExtrasPatch{Gates: &gates}}); err != nil {
		t.Fatal(err)
	}
}

func TestExtrasThroughStagingAndRename(t *testing.T) {
	idx, dir := newModel(t)
	mkNode(t, idx, "app")
	pid, err := idx.Stage(StageInput{Source: OpCreateNode, Ops: []StageOp{
		{Op: OpCreateNode, Create: CreateInput{ID: "spike", Name: "Spike", Type: "service", NodeExtras: NodeExtras{Gates: []NodeID{"app", "later"}}}},
		{Op: OpCreateNode, Create: CreateInput{ID: "later", Name: "Later", Type: "service"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	acceptAllOK(t, idx, pid)
	del := []string{"docs/a.md"}
	pid, err = idx.Stage(StageInput{Source: OpUpdateNode, Ops: []StageOp{
		{Op: OpUpdateNode, NodeID: "app", Update: UpdateInput{NodeExtrasPatch: NodeExtrasPatch{Deliverables: &del}}},
		{Op: OpRenameNode, NodeID: "later", NewID: "after"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	acceptAllOK(t, idx, pid)
	got := reopen(t, dir)
	eq(t, "deliverables", got.Nodes["app"].Deliverables, []string{"docs/a.md"})
	eq(t, "gates renamed", got.Nodes["spike"].Gates, []NodeID{"after", "app"})
	if _, err := idx.Stage(StageInput{Source: OpUpdateNode, Ops: []StageOp{
		{Op: OpUpdateNode, NodeID: "app", Update: UpdateInput{NodeExtrasPatch: NodeExtrasPatch{Gates: &[]NodeID{"ghost"}}}},
	}}); !errors.Is(err, ErrBadDependency) {
		t.Fatalf("staged bad gate: %v", err)
	}
}
