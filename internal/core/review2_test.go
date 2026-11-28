package core

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func gateCleared(t *testing.T, idx *Index, id NodeID) []bool {
	t.Helper()
	r := mustOrder(t, idx, "")
	for _, it := range r.Order {
		if it.ID == id {
			out := []bool{}
			for _, g := range it.HumanGates {
				out = append(out, g.Cleared)
			}
			return out
		}
	}
	t.Fatalf("no %s", id)
	return nil
}

func TestClearedGatesFollowContentNotIndex(t *testing.T) {
	idx, dir := planRepo(t)
	mkFull(t, idx, CreateInput{ID: "web", NodeExtras: NodeExtras{HumanGates: []HumanGate{{Kind: "decision", Note: "pick"}, {Kind: "phone", Note: "call"}}}})
	if err := idx.SetGateCleared("web", 1, true); err != nil {
		t.Fatal(err)
	}
	// Reorder: the cleared "phone" gate moves to index 0 and stays cleared.
	swapped := []HumanGate{{Kind: "phone", Note: "call"}, {Kind: "decision", Note: "pick"}}
	if err := idx.Update("web", UpdateInput{NodeExtrasPatch: NodeExtrasPatch{HumanGates: &swapped}}); err != nil {
		t.Fatal(err)
	}
	eq(t, "after reorder", gateCleared(t, idx, "web"), []bool{true, false})
	// Edit the note: it is a different gate now, so it is not cleared.
	edited := []HumanGate{{Kind: "phone", Note: "call someone else"}, {Kind: "decision", Note: "pick"}}
	if err := idx.Update("web", UpdateInput{NodeExtrasPatch: NodeExtrasPatch{HumanGates: &edited}}); err != nil {
		t.Fatal(err)
	}
	eq(t, "after edit", gateCleared(t, idx, "web"), []bool{false, false})
	// Legacy index-keyed entries are ignored.
	idx.localUnknown[gatesClearedKey] = []byte(`["web#0","web#1"]`)
	eq(t, "legacy", gateCleared(t, idx, "web"), []bool{false, false})
	// Identical gates are cleared independently.
	dup := []HumanGate{{Kind: "decision", Note: "x"}, {Kind: "decision", Note: "x"}}
	if err := idx.Update("web", UpdateInput{NodeExtrasPatch: NodeExtrasPatch{HumanGates: &dup}}); err != nil {
		t.Fatal(err)
	}
	if err := idx.SetGateCleared("web", 0, true); err != nil {
		t.Fatal(err)
	}
	eq(t, "dup", gateCleared(t, idx, "web"), []bool{true, false})
	_ = dir
}

func TestDeleteDropsClearedGates(t *testing.T) {
	idx, dir := planRepo(t)
	g := []HumanGate{{Kind: "decision", Note: "pick"}}
	mkFull(t, idx, CreateInput{ID: "web", NodeExtras: NodeExtras{HumanGates: g}})
	if err := idx.SetGateCleared("web", 0, true); err != nil {
		t.Fatal(err)
	}
	if err := idx.Delete("web"); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(dir + "/.arch/local.json"); err == nil && strings.Contains(string(b), "web#") {
		t.Fatal("cleared entry survived delete")
	}
	mkFull(t, idx, CreateInput{ID: "web", NodeExtras: NodeExtras{HumanGates: g}})
	eq(t, "recreated", gateCleared(t, idx, "web"), []bool{false})
}

func TestDeleteRefusedWhileGatesNamesIt(t *testing.T) {
	idx, _ := planRepo(t)
	mkFull(t, idx, CreateInput{ID: "api"})
	mkFull(t, idx, CreateInput{ID: "spike", NodeExtras: NodeExtras{Gates: []NodeID{"api"}}})
	err := idx.Delete("api")
	if !errors.Is(err, ErrHasDependents) || !strings.Contains(err.Error(), "spike (spike)") {
		t.Fatalf("delete: %v", err)
	}
	_, err = idx.Stage(StageInput{Source: OpDeleteNode, Ops: []StageOp{{Op: OpDeleteNode, NodeID: "api"}}})
	if !errors.Is(err, ErrHasDependents) || !strings.Contains(err.Error(), "spike (spike)") {
		t.Fatalf("stage delete: %v", err)
	}
	// Removing the gated node together with the gate-holder is fine, and a reparent still validates.
	mkFull(t, idx, CreateInput{ID: "grp"})
	if _, err := idx.StageReparent("api", "grp"); err != nil {
		t.Fatalf("reparent: %v", err)
	}
}
