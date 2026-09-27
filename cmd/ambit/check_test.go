package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KoJaco/ambit/internal/core"
)

func TestCheckExitsZeroWithViolations(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	run(t, dir, binPath, "init", dir)
	idx, err := core.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Create(core.CreateInput{
		Name:           "Payments Service",
		Type:           "service",
		Implementation: []string{"src/payments/**"},
	}); err != nil {
		t.Fatal(err)
	}
	idx, err = core.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Create(core.CreateInput{
		Name:           "Orders Service",
		Type:           "service",
		Implementation: []string{"src/orders/**"},
	}); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, filepath.Join(dir, "src", "orders", "create.ts"), "base\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "model")
	idx, err = core.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.SetAssignment("payments-service", time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, filepath.Join(dir, "src", "orders", "create.ts"), "changed\n")

	cmd := exec.Command(binPath, "check")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exit: %v\n%s", err, out)
	}
	text := string(out)
	if !strings.Contains(text, "src/orders/create.ts") || !strings.Contains(text, "outside_scope") || !strings.Contains(text, "orders-service") {
		t.Fatalf("report:\n%s", text)
	}
}

func TestCheckMissingArch(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(binPath, "check")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected missing .arch to fail")
	}
	if !strings.Contains(string(out), ".arch") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestCheckIgnoreWarningExitsZero(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	run(t, dir, binPath, "init", dir)
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "init")
	path := filepath.Join(dir, ".gitignore")
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	next := strings.Replace(string(text), ".arch/local.json\n", "", 1)
	if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binPath, "check")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exit: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), ".arch/local.json") {
		t.Fatalf("output:\n%s", out)
	}
}

func writeRepoFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
