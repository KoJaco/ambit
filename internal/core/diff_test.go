package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffNamesStagedUnstagedAndUntracked(t *testing.T) {
	dir := t.TempDir()
	gitInitRepo(t, dir)
	writeFile(t, filepath.Join(dir, "tracked.txt"), "base\n")
	gitCmd(t, dir, "add", "tracked.txt")
	gitCmd(t, dir, "commit", "-m", "base")

	writeFile(t, filepath.Join(dir, "tracked.txt"), "staged\n")
	gitCmd(t, dir, "add", "tracked.txt")
	staged := mustDiffNames(t, dir)
	if len(staged) != 1 || staged[0] != "tracked.txt" {
		t.Fatalf("staged diff: %#v", staged)
	}

	writeFile(t, filepath.Join(dir, "tracked.txt"), "unstaged\n")
	unstaged := mustDiffNames(t, dir)
	if len(unstaged) != 1 || unstaged[0] != "tracked.txt" {
		t.Fatalf("unstaged diff: %#v", unstaged)
	}

	writeFile(t, filepath.Join(dir, "new.txt"), "no\n")
	withUntracked := mustDiffNames(t, dir)
	for _, p := range withUntracked {
		if p == "new.txt" {
			t.Fatalf("untracked path included: %#v", withUntracked)
		}
	}
}

func TestDiffNamesRenameListsBothPaths(t *testing.T) {
	dir := t.TempDir()
	gitInitRepo(t, dir)
	writeFile(t, filepath.Join(dir, "old.txt"), "same\n")
	gitCmd(t, dir, "add", "old.txt")
	gitCmd(t, dir, "commit", "-m", "base")
	gitCmd(t, dir, "mv", "old.txt", "new.txt")
	got := mustDiffNames(t, dir)
	if !hasPath(got, "old.txt") || !hasPath(got, "new.txt") {
		t.Fatalf("rename diff: %#v", got)
	}
}

func TestDiffNamesRequiresCommit(t *testing.T) {
	dir := t.TempDir()
	gitInitRepo(t, dir)
	_, err := DiffNames(dir)
	if err == nil || !strings.Contains(err.Error(), "HEAD") {
		t.Fatalf("err %v", err)
	}
}

func mustDiffNames(t *testing.T, dir string) []string {
	t.Helper()
	got, err := DiffNames(dir)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func hasPath(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

func gitInitRepo(t *testing.T, dir string) {
	t.Helper()
	gitCmd(t, dir, "init")
	gitCmd(t, dir, "config", "--local", "user.name", "ambit-test")
	gitCmd(t, dir, "config", "--local", "user.email", "ambit-test@example.com")
}

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
