package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIgnoreWarningSilentWhenIgnored(t *testing.T) {
	dir := t.TempDir()
	gitInitRepo(t, dir)
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if w := IgnoreWarning(dir); w != "" {
		t.Fatal(w)
	}
}

func TestIgnoreWarningNamesUnignoredPath(t *testing.T) {
	dir := t.TempDir()
	gitInitRepo(t, dir)
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".gitignore")
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	next := strings.Replace(string(text), ".arch/local.json\n", "", 1)
	if next == string(text) {
		t.Fatalf("gitignore:\n%s", text)
	}
	if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
		t.Fatal(err)
	}
	w := IgnoreWarning(dir)
	if !strings.Contains(w, ".arch/local.json") {
		t.Fatal(w)
	}
	if strings.Contains(w, ".arch/.cache/") || strings.Contains(w, ".arch/.proposals/") {
		t.Fatal(w)
	}
}

func TestIgnoreWarningMissingRepo(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	w := IgnoreWarning(dir)
	if !strings.Contains(w, "could not confirm") {
		t.Fatal(w)
	}
	if strings.Contains(w, "git is not ignoring") {
		t.Fatal(w)
	}
}
