package main

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/KoJaco/ambit/internal/core"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func initWith(t *testing.T, stdin string, interactive bool, args ...string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	var out, errw bytes.Buffer
	code := cmdInit(append([]string{dir}, args...), strings.NewReader(stdin), interactive, &out, &errw)
	return dir, out.String() + errw.String(), code
}

func TestInitScopeFlags(t *testing.T) {
	dir, out, code := initWith(t, "", false, "--scope", "partial", "--part", "Billing API=internal/billing/**,cmd/billing/{a,b}/**", "--no-mcp")
	if code != 0 {
		t.Fatalf("%d %s", code, out)
	}
	idx, err := core.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if idx.Scope != core.ScopePartial || len(idx.Parts) != 1 {
		t.Fatalf("%q %+v", idx.Scope, idx.Parts)
	}
	p := idx.Parts[0]
	if p.ID != "billing-api" || p.Name != "Billing API" || len(p.Include) != 2 || p.Include[1] != "cmd/billing/{a,b}/**" {
		t.Fatalf("%+v", p)
	}

	// Default is whole and nothing is written.
	dir, _, code = initWith(t, "", false, "--no-mcp")
	idx, _ = core.Open(dir)
	if code != 0 || idx.Scope != core.ScopeWhole || len(idx.Parts) != 0 {
		t.Fatalf("%d %q", code, idx.Scope)
	}
	// --scope whole is explicit and valid.
	if _, out, code := initWith(t, "", false, "--scope=whole", "--no-mcp"); code != 0 {
		t.Fatalf("%s", out)
	}
	// --part alone implies partial.
	dir, _, code = initWith(t, "", false, "--part", "x=a/**", "--no-mcp")
	idx, _ = core.Open(dir)
	if code != 0 || idx.Scope != core.ScopePartial {
		t.Fatalf("%d %q", code, idx.Scope)
	}
}

func TestInitScopeFlagErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--scope", "sideways"},
		{"--scope"},
		{"--part", "noequals"},
		{"--part", "=a/**"},
		{"--part", "x="},
		{"--part", "x=[bad"},
		{"--scope", "whole", "--part", "x=a/**"},
		{"--part", "!!!=a/**"},
	} {
		if _, out, code := initWith(t, "", false, append(args, "--no-mcp")...); code != 1 {
			t.Fatalf("%v: code %d %s", args, code, out)
		}
	}
}

func TestInitPromptsForScopeOnFreshInteractiveInit(t *testing.T) {
	// method prompt (1 = freeform), scope prompt (2 = partial), first part.
	dir, out, code := initWith(t, "1\n2\napi=src/api/**\n", true, "--no-mcp")
	if code != 0 {
		t.Fatalf("%d %s", code, out)
	}
	if !strings.Contains(out, "Model scope:") {
		t.Fatalf("no scope prompt:\n%s", out)
	}
	idx, _ := core.Open(dir)
	if idx.Scope != core.ScopePartial || len(idx.Parts) != 1 || idx.Parts[0].ID != "api" {
		t.Fatalf("%q %+v", idx.Scope, idx.Parts)
	}
	// Empty answers keep whole.
	dir, _, code = initWith(t, "\n\n", true, "--no-mcp")
	idx, _ = core.Open(dir)
	if code != 0 || idx.Scope != core.ScopeWhole {
		t.Fatalf("%d %q", code, idx.Scope)
	}
	// An explicit --scope skips the scope prompt.
	_, out, _ = initWith(t, "\n", true, "--scope", "whole", "--no-mcp")
	if strings.Contains(out, "Model scope:") {
		t.Fatalf("prompted despite --scope:\n%s", out)
	}
	// Non-interactive never prompts.
	_, out, _ = initWith(t, "2\n", false, "--no-mcp")
	if strings.Contains(out, "Model scope:") {
		t.Fatal("prompted without a terminal")
	}
}

func TestInitDoesNotPromptScopeOnExistingProject(t *testing.T) {
	dir, _, code := initWith(t, "", false, "--method", "design-first", "--no-mcp")
	if code != 0 {
		t.Fatal(code)
	}
	var out, errw bytes.Buffer
	if code := cmdInit([]string{dir, "--no-mcp"}, strings.NewReader("2\n"), true, &out, &errw); code != 0 {
		t.Fatalf("%s", errw.String())
	}
	if strings.Contains(out.String(), "Model scope:") {
		t.Fatalf("re-running init must not prompt:\n%s", out.String())
	}
	idx, _ := core.Open(dir)
	if idx.Scope != core.ScopeWhole {
		t.Fatal(idx.Scope)
	}
}

func TestParseMCPArgs(t *testing.T) {
	dir, opts, err := parseMCPArgs(nil)
	if err != nil || dir != "." || opts.Role != "" {
		t.Fatalf("%q %+v %v", dir, opts, err)
	}
	dir, opts, err = parseMCPArgs([]string{"proj", "--role", "architect"})
	if err != nil || dir != "proj" || opts.Role != "architect" {
		t.Fatalf("%q %+v %v", dir, opts, err)
	}
	if _, opts, err = parseMCPArgs([]string{"--role=architect"}); err != nil || opts.Role != "architect" {
		t.Fatal(err)
	}
	for _, bad := range [][]string{{"--role"}, {"--role", "admin"}, {"--nope"}, {"a", "b"}} {
		if _, _, err := parseMCPArgs(bad); err == nil {
			t.Fatalf("%v should fail", bad)
		}
	}
}

func TestMCPRoleFlagRegistersAcceptProposalOverStdio(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	run(t, dir, binPath, "init", dir)
	names := func(args ...string) map[string]bool {
		cmd := exec.Command(binPath, args...)
		cmd.Dir = dir
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		session, err := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test", Version: "v1"}, nil).Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer session.Close()
		res, err := session.ListTools(ctx, &sdkmcp.ListToolsParams{})
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, tool := range res.Tools {
			out[tool.Name] = true
		}
		return out
	}
	if names("mcp")["accept_proposal"] {
		t.Fatal("default role must not have accept_proposal")
	}
	if !names("mcp", "--role", "architect")["accept_proposal"] {
		t.Fatal("--role architect must register accept_proposal")
	}
}
