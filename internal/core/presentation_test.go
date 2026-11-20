package core

import (
	"errors"
	"strings"
	"testing"
)

func TestPresentationValidateRoundTripUnknown(t *testing.T) {
	if err := validateIconWrite("not-in-registry", "not-in-registry"); err != nil {
		t.Fatalf("round-trip stored unknown icon: %v", err)
	}
	if err := validateIconWrite("box", "not-in-registry"); err != nil {
		t.Fatalf("fixing unknown icon to registry id should succeed: %v", err)
	}
	if err := validateIconWrite("other-unknown", "not-in-registry"); err == nil {
		t.Fatal("changing to different unknown should fail")
	}
}

func TestPresentationRejectNewUnknown(t *testing.T) {
	if err := validateColorWrite("not-a-token", ""); !errors.Is(err, ErrInvalidPresentation) {
		t.Fatalf("expected ErrInvalidPresentation, got %v", err)
	}
}

func TestRendererRejectsNewUnknownAndClears(t *testing.T) {
	if err := validateRendererWrite("diagram", ""); !errors.Is(err, ErrInvalidPresentation) {
		t.Fatalf("expected ErrInvalidPresentation, got %v", err)
	}
	if err := validateRendererWrite("not-a-renderer", "not-a-renderer"); err != nil {
		t.Fatalf("round-trip stored unknown renderer: %v", err)
	}
	n := &Node{Renderer: "schema", Schema: []SchemaColumn{{Title: "id", Type: "uuid"}}}
	empty := ""
	if err := applyRendererPatch(n, &empty); err != nil {
		t.Fatal(err)
	}
	if n.Renderer != "" {
		t.Fatalf("renderer=%q", n.Renderer)
	}
	cleared := []SchemaColumn{}
	if err := applySchemaPatch(n, &cleared); err != nil {
		t.Fatal(err)
	}
	if len(n.Schema) != 0 {
		t.Fatalf("schema=%v", n.Schema)
	}
}

func TestSchemaRejectsBlankAndOverlong(t *testing.T) {
	if err := validateSchema([]SchemaColumn{{Title: " ", Type: "uuid"}}); !errors.Is(err, ErrInvalidPresentation) {
		t.Fatalf("blank title: %v", err)
	}
	long := strings.Repeat("a", maxSchemaField+1)
	if err := validateSchema([]SchemaColumn{{Title: "id", Type: long}}); !errors.Is(err, ErrInvalidPresentation) {
		t.Fatalf("overlong type: %v", err)
	}
}

func TestPresentationClear(t *testing.T) {
	n := &Node{Icon: "box", Color: "blue"}
	empty := ""
	if err := applyIconPatch(n, &empty); err != nil {
		t.Fatal(err)
	}
	if n.Icon != "" {
		t.Fatalf("icon=%q", n.Icon)
	}
}
