package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookInstallWritesThenRefuses(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	run(t, dir, binPath, "hook", "install")
	path := filepath.Join(dir, ".git", "hooks", "pre-commit")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(first)
	if !strings.Contains(text, shellSingle(binPath)) || !strings.Contains(text, "check") || !strings.HasPrefix(text, "#!/bin/sh\n") {
		t.Fatalf("hook:\n%s", text)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("mode %v", info.Mode())
	}

	out := runFail(t, dir, binPath, "hook", "install")
	if !strings.Contains(out, "refused") {
		t.Fatalf("output %q", out)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("hook changed\n%s", second)
	}
}

func TestHookInstallLeavesExistingHook(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	path := filepath.Join(dir, ".git", "hooks", "pre-commit")
	original := []byte("#!/bin/sh\necho custom\n")
	if err := os.WriteFile(path, original, 0o755); err != nil {
		t.Fatal(err)
	}
	out := runFail(t, dir, binPath, "hook", "install")
	if !strings.Contains(out, "refused") {
		t.Fatalf("output %q", out)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, got) {
		t.Fatalf("hook changed\n%s", got)
	}
}

func runFail(t *testing.T, dir, bin string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected failure, output:\n%s", out)
	}
	return string(out)
}

func shellSingle(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
