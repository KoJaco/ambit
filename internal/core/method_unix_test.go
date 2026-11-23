//go:build unix

package core

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func hasWarning(idx *Index, sub string) bool {
	for _, w := range idx.Warnings() {
		if strings.Contains(w.Message, sub) {
			return true
		}
	}
	return false
}

func TestMethodFileSymlinkAndFIFORejected(t *testing.T) {
	_, dir := newModel(t)
	outside := filepath.Join(t.TempDir(), "secret.md")
	if err := os.WriteFile(outside, []byte("outside the model"), 0o644); err != nil {
		t.Fatal(err)
	}
	mdir := filepath.Join(dir, ".arch", "methods")
	if err := os.MkdirAll(mdir, 0o755); err != nil {
		t.Fatal(err)
	}
	// {file} that is a symlink to a file outside .arch.
	writeMethod(t, dir, "linked.json", `{"schema_version":1,"id":"linked","guidance":{"file":"g.md"}}`)
	if err := os.Symlink(outside, filepath.Join(mdir, "g.md")); err != nil {
		t.Skip("symlinks unavailable")
	}
	// The method JSON itself a symlink.
	real := filepath.Join(t.TempDir(), "real.json")
	_ = os.WriteFile(real, []byte(`{"schema_version":1,"id":"jsonlink"}`), 0o644)
	if err := os.Symlink(real, filepath.Join(mdir, "jsonlink.json")); err != nil {
		t.Fatal(err)
	}
	// {file} that is a FIFO: must not block.
	writeMethod(t, dir, "piped.json", `{"schema_version":1,"id":"piped","guidance":{"file":"p.md"}}`)
	if err := syscall.Mkfifo(filepath.Join(mdir, "p.md"), 0o644); err != nil {
		t.Skip("mkfifo unavailable")
	}
	done := make(chan *Index, 1)
	go func() { done <- reopen(t, dir) }()
	var idx *Index
	select {
	case idx = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("loading blocked on a FIFO")
	}
	for _, id := range []string{"linked", "jsonlink", "piped"} {
		if _, err := idx.MethodByID(id); err == nil {
			t.Errorf("method %s should have been skipped", id)
		}
	}
	if !hasWarning(idx, "not a regular file") {
		t.Fatalf("%+v", idx.Warnings())
	}
	if idx.ActiveMethod().ID != "freeform" {
		t.Fatal("must still load")
	}
}
