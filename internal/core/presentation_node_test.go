package core

import (
	"errors"
	"testing"
)

func TestPresentationPersistAndChildCount(t *testing.T) {
	idx, _ := newModel(t)
	platform, err := idx.Create(CreateInput{Name: "Platform", Type: "package", Spec: "p"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := idx.Create(CreateInput{Name: "Child", Type: "module", ParentID: platform, Spec: "c"})
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := idx.Create(CreateInput{Name: "Leaf", Type: "component", ParentID: child, Spec: "l"})
	if err != nil {
		t.Fatal(err)
	}
	blue := "blue"
	if err := idx.Update(platform, UpdateInput{
		Icon:  strPtr("server"),
		Color: &blue,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := idx.Node(platform)
	if err != nil {
		t.Fatal(err)
	}
	if got.Icon != "server" || got.Color != "blue" {
		t.Fatalf("presentation %+v", got)
	}
	lvl, err := idx.Level("", nil)
	if err != nil {
		t.Fatal(err)
	}
	var platformView *NodeView
	for _, c := range lvl.Children {
		if c.ID == platform {
			platformView = &c
			break
		}
	}
	if platformView == nil || platformView.ChildCount != 1 {
		t.Fatalf("platform child_count want 1, got %+v", platformView)
	}
	inner, err := idx.Level(platform, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range inner.Children {
		if c.ID == child && c.ChildCount != 1 {
			t.Fatalf("child child_count want 1, got %d", c.ChildCount)
		}
		if c.ID == leaf {
			t.Fatal("leaf should not appear on platform level")
		}
	}
	leafLvl, err := idx.Level(child, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range leafLvl.Children {
		if c.ID == leaf && c.ChildCount != 0 {
			t.Fatalf("leaf child_count want 0, got %d", c.ChildCount)
		}
	}
}

func TestRendererAndSchemaPersist(t *testing.T) {
	idx, _ := newModel(t)
	id, err := idx.Create(CreateInput{
		Name:     "Products",
		Type:     "data store",
		Spec:     "s",
		Renderer: "schema",
		Schema:   []SchemaColumn{{Title: "id", Type: "uuid"}, {Title: " name ", Type: " varchar "}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := idx.Node(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Renderer != "schema" || len(got.Schema) != 2 || got.Schema[1].Title != "name" || got.Schema[1].Type != "varchar" {
		t.Fatalf("schema %+v", got)
	}
	next := "group"
	empty := []SchemaColumn{}
	if err := idx.Update(id, UpdateInput{Renderer: &next, Schema: &empty}); err != nil {
		t.Fatal(err)
	}
	got, err = idx.Node(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Renderer != "group" || len(got.Schema) != 0 {
		t.Fatalf("after update %+v", got)
	}
	bad := "diagram"
	if err := idx.Update(id, UpdateInput{Renderer: &bad}); !errors.Is(err, ErrInvalidPresentation) {
		t.Fatalf("expected ErrInvalidPresentation, got %v", err)
	}
}

func TestPresentationRejectUnknownWrite(t *testing.T) {
	idx, _ := newModel(t)
	id, err := idx.Create(CreateInput{Name: "X", Type: "service", Spec: "s"})
	if err != nil {
		t.Fatal(err)
	}
	bad := "not-an-icon"
	if err := idx.Update(id, UpdateInput{Icon: &bad}); !errors.Is(err, ErrInvalidPresentation) {
		t.Fatalf("expected ErrInvalidPresentation, got %v", err)
	}
}

func strPtr(s string) *string { return &s }
