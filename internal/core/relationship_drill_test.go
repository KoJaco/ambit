package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRelationshipDrillReleaseGate(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	idx, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a, err := idx.Create(CreateInput{Name: "Orders", Type: "service"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := idx.Create(CreateInput{Name: "Payments", Type: "service"})
	if err != nil {
		t.Fatal(err)
	}
	relOne, err := idx.SetRelationship(SetRelationshipInput{From: a, To: b, Label: "calls", Kind: KindSync})
	if err != nil {
		t.Fatal(err)
	}
	relTwo, err := idx.SetRelationship(SetRelationshipInput{From: a, To: b, Label: "settles", Kind: KindAsync})
	if err != nil {
		t.Fatal(err)
	}
	if relOne.ID == relTwo.ID {
		t.Fatal("expected distinct relationship ids")
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	var gotOne, gotTwo bool
	for _, rel := range reopened.Relationships {
		if rel.ID == relOne.ID && rel.Label == "calls" {
			gotOne = true
		}
		if rel.ID == relTwo.ID && rel.Label == "settles" {
			gotTwo = true
		}
	}
	if !gotOne || !gotTwo {
		t.Fatalf("relationships after reload %#v", reopened.Relationships)
	}
	if _, err := reopened.Create(CreateInput{Name: "Step", Type: "step", RelationshipID: relOne.ID}); err != nil {
		t.Fatal(err)
	}
	inner, err := reopened.RelationshipLevel(relOne.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(inner.Members) != 1 || inner.Members[0].Name != "Step" {
		t.Fatalf("inner members %#v", inner.Members)
	}
	root, err := reopened.Level("")
	if err != nil {
		t.Fatal(err)
	}
	for _, child := range root.Children {
		if child.Name == "Step" {
			t.Fatal("member appeared on root level")
		}
	}
	if _, err := reopened.RelationshipLevel(relTwo.ID); !errors.Is(err, ErrEmptyRelationship) {
		t.Fatalf("empty interior: %v", err)
	}
}

func TestProposalMemberCreateAtomic(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	idx, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	from, _ := idx.Create(CreateInput{Name: "A", Type: "service"})
	to, _ := idx.Create(CreateInput{Name: "B", Type: "service"})
	rel, err := idx.SetRelationship(SetRelationshipInput{From: from, To: to, Label: "flow", Kind: KindSync})
	if err != nil {
		t.Fatal(err)
	}
	pid, err := idx.Stage(StageInput{
		Source: OpCreateNode,
		Ops: []StageOp{{
			Op: OpCreateNode,
			Create: CreateInput{
				Name:           "Handler",
				Type:           "step",
				RelationshipID: rel.ID,
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	applyHook = func(step string) error {
		if step == "index" {
			return os.ErrPermission
		}
		return nil
	}
	t.Cleanup(func() { applyHook = nil })
	err = idx.AcceptOperation(pid, 0, false)
	if err == nil {
		t.Fatal("expected accept failure")
	}
	if _, err := os.Stat(filepath.Join(dir, ".arch", "nodes", "handler.json")); err == nil {
		t.Fatal("partial member write on disk")
	}
	applyHook = nil
	if err := idx.AcceptOperation(pid, 0, false); err != nil {
		t.Fatal(err)
	}
	inner, err := idx.RelationshipLevel(rel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(inner.Members) != 1 {
		t.Fatalf("members %#v", inner.Members)
	}
}