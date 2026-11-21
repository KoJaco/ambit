package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallHook(t *testing.T) {
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Skipf("git init: %v %s", err, out)
	}
	if err := InstallHook(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".git", "hooks", "pre-commit")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("hook not executable: %v", info.Mode())
	}
	b, _ := os.ReadFile(path)
	s := string(b)
	if !strings.HasPrefix(s, "#!/bin/sh\n") {
		t.Fatalf("missing shebang: %q", s)
	}
	for _, want := range []string{
		"command -v ambit >/dev/null 2>&1",
		"exec ambit check",
		"exec npx --yes @kojaco/ambit check",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("hook missing %q:\n%s", want, s)
		}
	}
	exe, _ := os.Executable()
	if strings.Contains(s, exe) || strings.Contains(s, filepath.Dir(exe)) {
		t.Errorf("hook embeds the binary path:\n%s", s)
	}
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "exec") && strings.Contains(line, "/") && !strings.Contains(line, "@kojaco/ambit") {
			t.Errorf("exec line contains a path: %q", line)
		}
	}

	if err := InstallHook(dir); err == nil || !strings.Contains(err.Error(), "refused to overwrite") {
		t.Fatalf("second install: %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != s {
		t.Fatal("existing hook was modified")
	}
}

func TestInstallHookKeepsExistingHook(t *testing.T) {
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Skipf("git init: %v %s", err, out)
	}
	path := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	custom := "#!/bin/sh\necho mine\n"
	if err := os.WriteFile(path, []byte(custom), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := InstallHook(dir); err == nil {
		t.Fatal("expected refusal")
	}
	b, _ := os.ReadFile(path)
	if string(b) != custom {
		t.Fatal("custom hook was changed")
	}
}
