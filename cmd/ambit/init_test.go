package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ambit-bin")
	if err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
	binPath = filepath.Join(dir, "ambit")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Stderr.WriteString("build ambit: " + err.Error() + "\n")
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestGitignoreGate(t *testing.T) {
	t.Run("init then git", func(t *testing.T) {
		assertIgnored(t, func(dir string) {
			out := run(t, dir, binPath, "init", dir)
			if !strings.Contains(out, "hook install") {
				t.Fatalf("stdout %q", out)
			}
			gitInit(t, dir)
		})
	})
	t.Run("git then init", func(t *testing.T) {
		assertIgnored(t, func(dir string) {
			gitInit(t, dir)
			run(t, dir, binPath, "init", dir)
		})
	})
	t.Run("existing gitignore", func(t *testing.T) {
		assertIgnored(t, func(dir string) {
			if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("# unrelated\n*.log\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			gitInit(t, dir)
			run(t, dir, binPath, "init", dir)
			text, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(text), "*.log") || strings.Count(string(text), ".arch/local.json") != 1 {
				t.Fatalf("gitignore:\n%s", text)
			}
		})
	})
}

func TestUnknownCommand(t *testing.T) {
	cmd := exec.Command(binPath, "mcp")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected unknown command to fail")
	}
}

func assertIgnored(t *testing.T, setup func(dir string)) {
	t.Helper()
	dir := t.TempDir()
	setup(dir)
	if _, err := os.Stat(filepath.Join(dir, ".git", "hooks", "pre-commit")); err == nil {
		t.Fatal("init wrote a pre-commit hook")
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, ".arch", "local.json"), "{}\n")
	write(filepath.Join(dir, ".arch", ".cache", "layout.json"), "{}\n")
	write(filepath.Join(dir, ".arch", ".proposals", "p", "manifest.json"), "{}\n")
	write(filepath.Join(dir, ".arch", "nodes", "example.json"), "{}\n")
	git(t, dir, "add", "-A")
	out := git(t, dir, "status", "--porcelain")
	for _, bad := range []string{"local.json", ".cache", ".proposals"} {
		if strings.Contains(out, bad) {
			t.Fatalf("git status contains %s:\n%s", bad, out)
		}
	}
	if !strings.Contains(out, "example.json") {
		t.Fatalf("canonical node file missing from git status:\n%s", out)
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	git(t, dir, "init")
	git(t, dir, "config", "--local", "user.name", "ambit-test")
	git(t, dir, "config", "--local", "user.email", "ambit-test@example.com")
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func run(t *testing.T, dir, bin string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ambit %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
