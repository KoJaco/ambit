package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLayoutPutLeavesCanonicalFilesAlone(t *testing.T) {
	idx, dir := newModel(t)
	id := coreCreate(t, idx, "Payments Service")
	nodePath := filepath.Join(dir, ".arch", "nodes", string(id)+".json")
	indexPath := filepath.Join(dir, ".arch", "index.json")
	beforeNode := string(mustRead(t, nodePath))
	beforeIndex := string(mustRead(t, indexPath))

	missing, err := idx.Layout("no-such-level")
	if err != nil {
		t.Fatal(err)
	}
	if len(missing.Positions) != 0 {
		t.Fatalf("missing layout %#v", missing)
	}

	err = idx.PutLayout(RootLayoutKey, Layout{Positions: []Position{{ID: string(id), X: 12, Y: 40}}})
	if err != nil {
		t.Fatal(err)
	}
	if string(mustRead(t, nodePath)) != beforeNode || string(mustRead(t, indexPath)) != beforeIndex {
		t.Fatal("layout put modified canonical files")
	}
	cache := filepath.Join(dir, ".arch", ".cache", "layout", RootLayoutKey+".json")
	if _, err := os.Stat(cache); err != nil {
		t.Fatal(err)
	}
	got, err := idx.Layout(RootLayoutKey)
	if err != nil || len(got.Positions) != 1 || got.Positions[0].X != 12 || got.Positions[0].ID != string(id) {
		t.Fatalf("layout %#v err %v", got, err)
	}
}

func TestStructuralChangeInvalidatesLayout(t *testing.T) {
	idx, _ := newModel(t)
	platform, err := idx.Create(CreateInput{Name: "Platform", Type: "boundary"})
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.PutLayout(RootLayoutKey, Layout{Positions: []Position{{ID: string(platform), X: 1, Y: 2}}}); err != nil {
		t.Fatal(err)
	}
	if err := idx.PutLayout(string(platform), Layout{Positions: []Position{{ID: "child", X: 3, Y: 4}}}); err != nil {
		t.Fatal(err)
	}

	name := "Platform Renamed"
	if err := idx.Update(platform, UpdateInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	root, err := idx.Layout(RootLayoutKey)
	if err != nil || len(root.Positions) != 1 {
		t.Fatalf("name change dropped root layout %#v %v", root, err)
	}
	nested, err := idx.Layout(string(platform))
	if err != nil || len(nested.Positions) != 1 {
		t.Fatalf("name change dropped nested layout %#v %v", nested, err)
	}

	child, err := idx.Create(CreateInput{Name: "Child", Type: "service", ParentID: platform})
	if err != nil {
		t.Fatal(err)
	}
	nested, err = idx.Layout(string(platform))
	if err != nil || len(nested.Positions) != 0 {
		t.Fatalf("create left nested layout %#v", nested)
	}
	root, err = idx.Layout(RootLayoutKey)
	if err != nil || len(root.Positions) != 1 {
		t.Fatalf("create under a parent cleared the root %#v", root)
	}

	if err := idx.PutLayout(string(platform), Layout{Positions: []Position{{ID: string(child), X: 5, Y: 6}}}); err != nil {
		t.Fatal(err)
	}
	if err := idx.PutLayout(RootLayoutKey, Layout{Positions: []Position{{ID: string(platform), X: 1, Y: 2}}}); err != nil {
		t.Fatal(err)
	}
	empty := NodeID("")
	if err := idx.Update(child, UpdateInput{Parent: &empty}); err != nil {
		t.Fatal(err)
	}
	nested, _ = idx.Layout(string(platform))
	root, _ = idx.Layout(RootLayoutKey)
	if len(nested.Positions) != 0 || len(root.Positions) != 0 {
		t.Fatalf("reparent left caches nested=%#v root=%#v", nested, root)
	}

	if err := idx.PutLayout(RootLayoutKey, Layout{Positions: []Position{{ID: string(child), X: 8, Y: 9}}}); err != nil {
		t.Fatal(err)
	}
	if err := idx.Delete(child); err != nil {
		t.Fatal(err)
	}
	root, err = idx.Layout(RootLayoutKey)
	if err != nil || len(root.Positions) != 0 {
		t.Fatalf("delete left root layout %#v %v", root, err)
	}
}

func coreCreate(t *testing.T, idx *Index, name string) NodeID {
	t.Helper()
	id, err := idx.Create(CreateInput{Name: name, Type: "service"})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestInvalidateLayoutRemovesHashedVariantsOnly(t *testing.T) {
	idx, dir := newModel(t)
	layoutDir := filepath.Join(dir, ".arch", ".cache", "layout")
	keys := []string{RootLayoutKey, RootLayoutKey + "~abc", RootLayoutKey + "~def", "_rootx", "_rootx~abc", "other~abc"}
	for _, key := range keys {
		if err := idx.PutLayout(key, Layout{}); err != nil {
			t.Fatal(err)
		}
	}
	// A file next to the layout dir must never be touched.
	outside := filepath.Join(dir, ".arch", ".cache", RootLayoutKey+"~abc.json")
	if err := os.WriteFile(outside, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx.mu.Lock()
	idx.invalidateLayoutLocked("")
	idx.mu.Unlock()
	gone := []string{RootLayoutKey, RootLayoutKey + "~abc", RootLayoutKey + "~def"}
	kept := []string{"_rootx", "_rootx~abc", "other~abc"}
	for _, key := range gone {
		if _, err := os.Stat(filepath.Join(layoutDir, key+".json")); !os.IsNotExist(err) {
			t.Fatalf("%s should be removed: %v", key, err)
		}
	}
	for _, key := range kept {
		if _, err := os.Stat(filepath.Join(layoutDir, key+".json")); err != nil {
			t.Fatalf("%s should remain: %v", key, err)
		}
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("file outside layout dir removed: %v", err)
	}
}
