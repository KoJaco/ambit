package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KoJaco/ambit/internal/core"
	"github.com/KoJaco/ambit/internal/mcp"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPListsToolsOverStdio(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	run(t, dir, binPath, "init", dir)
	cmd := exec.Command(binPath, "mcp")
	cmd.Dir = dir
	transport := &sdkmcp.CommandTransport{Command: cmd}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test", Version: "v1"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	res, err := session.ListTools(ctx, &sdkmcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Tools) != 8 {
		t.Fatalf("tools %d", len(res.Tools))
	}
}

func TestCheckScopeEquivalenceGate(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	run(t, dir, binPath, "init", dir)
	idx, err := core.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Create(core.CreateInput{
		Name: "Payments", Type: "service", Implementation: []string{"src/payments/**"},
		Scope: []string{"src/payments/api/**"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Create(core.CreateInput{
		Name: "Billing", Type: "service", Implementation: []string{"src/billing/**"}, Protected: true,
	}); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, filepath.Join(dir, "src", "payments", "api", "a.ts"), "base\n")
	writeRepoFile(t, filepath.Join(dir, "src", "billing", "fee.ts"), "base\n")
	writeRepoFile(t, filepath.Join(dir, "src", "unmapped", "x.ts"), "base\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "base")
	if err := idx.SetAssignment("payments", time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, filepath.Join(dir, "src", "payments", "api", "a.ts"), "changed\n")
	writeRepoFile(t, filepath.Join(dir, "src", "billing", "fee.ts"), "changed\n")
	writeRepoFile(t, filepath.Join(dir, "src", "unmapped", "x.ts"), "changed\n")

	files := []string{
		"src/payments/api/a.ts",
		"src/billing/fee.ts",
		"src/unmapped/x.ts",
	}
	idx, err = core.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	direct, err := core.CheckScope("payments", files, idx)
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "check")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("check: %v\n%s", err, out)
	}
	report := string(out)
	for _, r := range direct {
		switch r.Kind {
		case core.KindAllowed:
			if strings.Contains(report, r.Path) {
				t.Fatalf("allowed path should not appear in report: %s", r.Path)
			}
		case core.KindViolation:
			for _, h := range r.Hits {
				if !strings.Contains(report, r.Path) || !strings.Contains(report, h.Rule) || !strings.Contains(report, string(h.Node)) {
					t.Fatalf("report missing violation for %s rule %s node %s:\n%s", r.Path, h.Rule, h.Node, report)
				}
			}
		case core.KindInformational:
			if !strings.Contains(report, r.Path) || !strings.Contains(report, "informational") {
				t.Fatalf("report missing informational for %s:\n%s", r.Path, report)
			}
		}
	}

	mcpOut, err := mcp.CheckScopeForTool(idx, "payments", files)
	if err != nil {
		t.Fatal(err)
	}
	if len(mcpOut) != len(direct) {
		t.Fatalf("mcp len %d direct %d", len(mcpOut), len(direct))
	}
	for i := range direct {
		if direct[i].Kind != mcpOut[i].Kind || direct[i].Path != mcpOut[i].Path {
			t.Fatalf("%d: direct %+v mcp %+v", i, direct[i], mcpOut[i])
		}
		if len(direct[i].Hits) != len(mcpOut[i].Hits) {
			t.Fatalf("hits mismatch at %d", i)
		}
		for j := range direct[i].Hits {
			if direct[i].Hits[j] != mcpOut[i].Hits[j] {
				t.Fatalf("hit %d/%d: %+v vs %+v", i, j, direct[i].Hits[j], mcpOut[i].Hits[j])
			}
		}
	}
}
