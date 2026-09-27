package core

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchRebuild(t *testing.T) {
	idx, dir := newModel(t)
	id, err := idx.Create(CreateInput{Name: "Orders", Type: "service", Spec: "s"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got := make(chan error, 1)
	if err := idx.Watch(ctx, func(err error) {
		select {
		case got <- err:
		default:
		}
	}); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, ".arch", "nodes", string(id)+".json")
	obj := readObj(t, path)
	obj["name"] = "Orders Two"
	writeObj(t, path, obj)

	select {
	case err := <-got:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for reload")
	}
	if idx.Nodes[id].Name != "Orders Two" {
		t.Fatalf("name %q", idx.Nodes[id].Name)
	}
}
