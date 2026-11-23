package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateStatusGatedHonoursOtherInstanceImmediately(t *testing.T) {
	a, dir := newModel(t)
	id := mk(t, a, CreateInput{Name: "Orders", Type: "service"})
	if _, err := a.SetMethod("design-first"); err != nil {
		t.Fatal(err)
	}
	if err := a.UpdateStatusGated(id, StatusDone); !errors.Is(err, ErrStageGate) {
		t.Fatalf("design stage: %v", err)
	}
	// Another process advances the stage; a's in-memory state is stale.
	b := reopen(t, dir)
	if err := b.SetStage("build"); err != nil {
		t.Fatal(err)
	}
	if err := a.UpdateStatusGated(id, StatusDone); err != nil {
		t.Fatalf("stage change by another instance not honoured: %v", err)
	}
	// And the other way: moved back to design, the gate applies at once.
	if err := b.SetStage("design"); err != nil {
		t.Fatal(err)
	}
	if err := a.UpdateStatusGated(id, StatusInProgress); !errors.Is(err, ErrStageGate) {
		t.Fatalf("regression to design not honoured: %v", err)
	}
	// Switching to freeform elsewhere lifts the gate.
	if _, err := b.SetMethod("freeform"); err != nil {
		t.Fatal(err)
	}
	if err := a.UpdateStatusGated(id, StatusInProgress); err != nil {
		t.Fatal(err)
	}
}

func TestSetConfigKeysKeepsExternalEdits(t *testing.T) {
	a, dir := newModel(t)
	b := reopen(t, dir)
	if err := b.SetConfigKeys(map[string]any{"custom_key": "from-b"}); err != nil {
		t.Fatal(err)
	}
	// a never saw custom_key; its write must not drop it.
	if _, err := a.SetMethod("design-first"); err != nil {
		t.Fatal(err)
	}
	obj := readConfigObj(t, dir)
	if string(obj["custom_key"]) != `"from-b"` || string(obj["method"]) != `"design-first"` {
		t.Fatalf("%v", obj)
	}
	// A hand edit made while a is running survives too.
	cfg := filepath.Join(dir, ".arch", "config.json")
	if err := os.WriteFile(cfg, []byte(`{"extra":1,"method":"design-first"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.SetStage("build"); err != nil {
		t.Fatal(err)
	}
	if obj := readConfigObj(t, dir); string(obj["extra"]) != "1" || string(obj["stage"]) != `"build"` {
		t.Fatalf("%v", obj)
	}
}

func TestMethodFileRejectsOversize(t *testing.T) {
	idx, dir := newModel(t)
	writeMethod(t, dir, "big.json", `{"schema_version":1,"id":"big","guidance":{"file":"big.md"}}`)
	writeMethod(t, dir, "big.md", strings.Repeat("x", maxMethodFileBytes+1))
	idx = reopen(t, dir)
	if _, err := idx.MethodByID("big"); err == nil {
		t.Fatal("oversized {file} should skip the method")
	}
	found := false
	for _, w := range idx.Warnings() {
		if strings.Contains(w.Message, "larger than") {
			found = true
		}
	}
	if !found {
		t.Fatalf("%+v", idx.Warnings())
	}
}
