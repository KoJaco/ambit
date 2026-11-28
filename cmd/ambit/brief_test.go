package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KoJaco/ambit/internal/core"
)

func briefFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := core.Init(dir); err != nil {
		t.Fatal(err)
	}
	idx, err := core.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range []core.CreateInput{
		{ID: "api", Name: "API", Type: "service", Implementation: []string{"api/**"}, Verify: []string{"make test"}},
		{ID: "web", Name: "Web", Type: "service", Implementation: []string{"web/**"}, DependsOn: []core.NodeID{"api"}},
		{ID: "stripe", Name: "Stripe", Type: "external", NodeExtras: core.NodeExtras{External: true}},
	} {
		if _, err := idx.Create(in); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestBriefExportNodeAndAll(t *testing.T) {
	dir := briefFixture(t)
	var out, errw bytes.Buffer
	if code := cmdBrief(dir, []string{"export", "--node", "api"}, &out, &errw); code != 0 {
		t.Fatalf("code %d %s", code, errw.String())
	}
	body, err := os.ReadFile(filepath.Join(dir, ".arch", "briefs", "api.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), briefMarker) || !strings.Contains(string(body), "make test") || !strings.Contains(string(body), "rely on you") {
		t.Fatalf("brief:\n%s", body)
	}
	out.Reset()
	custom := filepath.Join(t.TempDir(), "briefs")
	if code := cmdBrief(dir, []string{"export", "--all", "--out", custom}, &out, &errw); code != 0 {
		t.Fatalf("all: %d %s", code, errw.String())
	}
	entries, _ := os.ReadDir(custom)
	if len(entries) != 2 {
		t.Fatalf("expected api and web only (externals skipped), got %v", entries)
	}
}

func TestBriefExportDoesNotOverwriteHandWrittenWithoutForce(t *testing.T) {
	dir := briefFixture(t)
	path := filepath.Join(dir, ".arch", "briefs", "api.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("my own notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errw bytes.Buffer
	if code := cmdBrief(dir, []string{"export", "--node", "api"}, &out, &errw); code != 1 || !strings.Contains(errw.String(), "--force") {
		t.Fatalf("code %d %s", code, errw.String())
	}
	if got, _ := os.ReadFile(path); string(got) != "my own notes\n" {
		t.Fatalf("overwritten: %s", got)
	}
	// --all keeps going past a protected file and still reports failure.
	errw.Reset()
	if code := cmdBrief(dir, []string{"export", "--all"}, &out, &errw); code != 1 {
		t.Fatalf("all with a blocked file: %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, ".arch", "briefs", "web.md")); err != nil {
		t.Fatalf("web not written: %v", err)
	}
	if code := cmdBrief(dir, []string{"export", "--node", "api", "--force"}, &out, &errw); code != 0 {
		t.Fatalf("force: %d %s", code, errw.String())
	}
	if got, _ := os.ReadFile(path); !strings.Contains(string(got), briefMarker) {
		t.Fatal("force did not overwrite")
	}
	// Generated files are replaced without --force.
	if code := cmdBrief(dir, []string{"export", "--node", "api"}, &out, &errw); code != 0 {
		t.Fatalf("regenerate: %d", code)
	}
}

func TestBriefExportUsageAndUnknownNode(t *testing.T) {
	dir := briefFixture(t)
	var out, errw bytes.Buffer
	for _, args := range [][]string{{}, {"export"}, {"export", "--all", "--node", "api"}, {"export", "--bogus"}, {"list"}} {
		errw.Reset()
		if code := cmdBrief(dir, args, &out, &errw); code != 1 || !strings.Contains(errw.String(), "usage") {
			t.Fatalf("%v: %d %s", args, code, errw.String())
		}
	}
	errw.Reset()
	if code := cmdBrief(dir, []string{"export", "--node", "../escape"}, &out, &errw); code != 1 {
		t.Fatalf("unknown node: %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, ".arch", "escape.md")); err == nil {
		t.Fatal("path traversal wrote a file")
	}
}
