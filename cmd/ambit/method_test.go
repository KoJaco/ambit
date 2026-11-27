package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KoJaco/ambit/internal/core"
)

func initDir(t *testing.T, args ...string) string {
	t.Helper()
	dir := t.TempDir()
	var out, errw bytes.Buffer
	if code := cmdInit(append([]string{dir}, args...), strings.NewReader(""), false, &out, &errw); code != 0 {
		t.Fatalf("init %d: %s", code, errw.String())
	}
	return dir
}

func TestParseInitArgs(t *testing.T) {
	cases := []struct {
		args        []string
		dir, method string
		bad         bool
	}{
		{nil, ".", "", false},
		{[]string{"x"}, "x", "", false},
		{[]string{"--method", "design-first"}, ".", "design-first", false},
		{[]string{"x", "--method=map-existing"}, "x", "map-existing", false},
		{[]string{"--method", "design-first", "x"}, "x", "design-first", false},
		{[]string{"--method"}, "", "", true},
		{[]string{"a", "b"}, "", "", true},
		{[]string{"--nope"}, "", "", true},
	}
	for _, c := range cases {
		dir, method, _, err := parseInitArgs(c.args)
		if (err != nil) != c.bad || dir != c.dir || method != c.method {
			t.Errorf("%v: %q %q %v", c.args, dir, method, err)
		}
	}
}

func TestInitNonInteractiveDefaultsToFreeform(t *testing.T) {
	dir := initDir(t)
	data, _ := os.ReadFile(filepath.Join(dir, ".arch", "config.json"))
	if strings.Contains(string(data), "method") {
		t.Fatalf("config %s", data)
	}
	idx, _ := core.Open(dir)
	if idx.ActiveMethod().ID != "freeform" {
		t.Fatal(idx.ActiveMethod().ID)
	}
}

func TestInitWithMethodFlag(t *testing.T) {
	dir := initDir(t, "--method", "design-first")
	idx, err := core.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if idx.ActiveMethod().ID != "design-first" || idx.EffectiveStage() != "design" {
		t.Fatalf("%s %s", idx.ActiveMethod().ID, idx.EffectiveStage())
	}
	var out, errw bytes.Buffer
	if code := cmdInit([]string{t.TempDir(), "--method", "nope"}, nil, false, &out, &errw); code != 1 || !strings.Contains(errw.String(), "unknown method") {
		t.Fatalf("%d %s", code, errw.String())
	}
}

func TestInitPromptsWhenInteractive(t *testing.T) {
	dir := t.TempDir()
	var out, errw bytes.Buffer
	if code := cmdInit([]string{dir}, strings.NewReader("2\n"), true, &out, &errw); code != 0 {
		t.Fatalf("%d %s", code, errw.String())
	}
	if strings.Contains(out.String(), "\x1b") || !strings.Contains(out.String(), "1) freeform") || !strings.Contains(out.String(), "(default)") {
		t.Fatalf("non-terminal output must have no ANSI:\n%q", out.String())
	}
	var hl bytes.Buffer
	if _, err := promptMethod(strings.NewReader("\n"), &hl, []core.MethodSummary{{ID: "freeform", Name: "Freeform"}}, true); err != nil || !strings.Contains(hl.String(), "\x1b[1m") {
		t.Fatalf("highlight: %v %q", err, hl.String())
	}
	idx, _ := core.Open(dir)
	if idx.ActiveMethod().ID != "design-first" {
		t.Fatal(idx.ActiveMethod().ID)
	}
	// Empty answer keeps freeform.
	dir2 := t.TempDir()
	out.Reset()
	if code := cmdInit([]string{dir2}, strings.NewReader("\n"), true, &out, &errw); code != 0 {
		t.Fatal(errw.String())
	}
	idx, _ = core.Open(dir2)
	if idx.ActiveMethod().ID != "freeform" {
		t.Fatal(idx.ActiveMethod().ID)
	}
}

func TestMethodAndStageCommands(t *testing.T) {
	dir := initDir(t)
	runCmd := func(f func(string, []string, *bytes.Buffer, *bytes.Buffer) int, args ...string) (int, string, string) {
		var o, e bytes.Buffer
		code := f(dir, args, &o, &e)
		return code, o.String(), e.String()
	}
	method := func(d string, a []string, o, e *bytes.Buffer) int { return cmdMethod(d, a, o, e) }
	stage := func(d string, a []string, o, e *bytes.Buffer) int { return cmdStage(d, a, o, e) }

	code, out, _ := runCmd(method, "list")
	if code != 0 || !strings.Contains(out, "* freeform") || !strings.Contains(out, "design-first") || !strings.Contains(out, "map-existing") {
		t.Fatalf("list %d\n%s", code, out)
	}
	code, out, _ = runCmd(method, "show", "design-first")
	if code != 0 || !strings.Contains(out, "stages: design, build") || !strings.Contains(out, "gate: while design") || !strings.Contains(out, "lint leaf-implementation: enforced") {
		t.Fatalf("show %d\n%s", code, out)
	}
	if code, _, e := runCmd(method, "show", "nope"); code != 1 || !strings.Contains(e, "unknown method") {
		t.Fatalf("%d %s", code, e)
	}
	if code, out, _ := runCmd(stage); code != 0 || !strings.Contains(out, "no stages") {
		t.Fatalf("%d %s", code, out)
	}
	if code, _, e := runCmd(stage, "set", "build"); code != 1 || !strings.Contains(e, "no stages") {
		t.Fatalf("%d %s", code, e)
	}
	if code, out, _ := runCmd(method, "set", "design-first"); code != 0 || !strings.Contains(out, "stage: design") {
		t.Fatalf("%d %s", code, out)
	}
	if code, out, _ := runCmd(stage, "show"); code != 0 || !strings.Contains(out, "stage: design") {
		t.Fatalf("%d %s", code, out)
	}
	if code, _, e := runCmd(stage, "set", "ship"); code != 1 || !strings.Contains(e, "unknown stage") {
		t.Fatalf("%d %s", code, e)
	}
	if code, _, _ := runCmd(stage, "set", "build"); code != 0 {
		t.Fatal(code)
	}
	if code, out, _ := runCmd(method, "set", "map-existing"); code != 0 || !strings.Contains(out, `stage "build" cleared`) {
		t.Fatalf("%d %s", code, out)
	}
	if code, _, _ := runCmd(method, "bogus"); code != 1 {
		t.Fatal(code)
	}
}

func TestCheckModelExitCodes(t *testing.T) {
	dir := initDir(t, "--method", "design-first")
	idx, err := core.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	// A leaf with no implementation is an enforced finding under design-first.
	if _, err := idx.Create(core.CreateInput{Name: "Orders", Type: "service"}); err != nil {
		t.Fatal(err)
	}
	var out, errw bytes.Buffer
	if code := cmdCheckModel(dir, nil, &out, &errw); code != 0 {
		t.Fatalf("non-strict %d", code)
	}
	if !strings.Contains(out.String(), "enforced (") || !strings.Contains(out.String(), "[leaf-implementation] orders") {
		t.Fatalf("out:\n%s", out.String())
	}
	out.Reset()
	if code := cmdCheckModel(dir, []string{"--strict"}, &out, &errw); code != 1 {
		t.Fatalf("strict %d\n%s", code, out.String())
	}
	out.Reset()
	// --all downgrades everything to advisory, so strict passes.
	if code := cmdCheckModel(dir, []string{"--strict", "--all"}, &out, &errw); code != 0 || strings.Contains(out.String(), "enforced (") || !strings.Contains(out.String(), "advisory (") {
		t.Fatalf("--all %d\n%s", code, out.String())
	}
	// Freeform enforces nothing.
	free := initDir(t)
	fidx, _ := core.Open(free)
	_, _ = fidx.Create(core.CreateInput{Name: "Orders", Type: "service"})
	out.Reset()
	if code := cmdCheckModel(free, []string{"--strict"}, &out, &errw); code != 0 || !strings.Contains(out.String(), "no findings") {
		t.Fatalf("freeform %d\n%s", code, out.String())
	}
	if code := cmdCheckModel(dir, []string{"--bogus"}, &out, &errw); code != 1 {
		t.Fatal(code)
	}
}

func TestExportSkillGolden(t *testing.T) {
	for _, c := range []struct{ method, target, golden string }{
		{"design-first", "claude", "export_design_first_claude.golden"},
		{"design-first", "cursor", "export_design_first_cursor.golden"},
		{"freeform", "claude", "export_freeform_claude.golden"},
	} {
		m, ok := core.BuiltinMethod(c.method)
		if !ok {
			t.Fatal(c.method)
		}
		got, err := formatSkill(m, c.target)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join("testdata", c.golden)
		if os.Getenv("UPDATE_GOLDEN") != "" {
			_ = os.MkdirAll("testdata", 0o755)
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got != string(want) {
			t.Errorf("%s differs:\n%s", c.golden, got)
		}
		if !strings.Contains(got, "MCP server is the source of truth") {
			t.Errorf("%s lacks generated header", c.golden)
		}
	}
	if _, err := formatSkill(&core.Method{ID: "x", Name: "X"}, "vim"); err == nil {
		t.Fatal("expected unknown target error")
	}
}

func TestExportSkillWritesFiles(t *testing.T) {
	dir := initDir(t, "--method", "map-existing")
	var out, errw bytes.Buffer
	if code := cmdMethod(dir, []string{"export-skill"}, &out, &errw); code != 0 {
		t.Fatal(errw.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", "ambit-method", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if code := cmdMethod(dir, []string{"export-skill", "--target", "cursor"}, &out, &errw); code != 0 {
		t.Fatal(errw.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".cursor", "rules", "ambit-method.mdc")); err != nil {
		t.Fatal(err)
	}
	custom := filepath.Join(dir, "out", "x.md")
	if code := cmdMethod(dir, []string{"export-skill", "--out", custom}, &out, &errw); code != 0 {
		t.Fatal(errw.String())
	}
	if _, err := os.Stat(custom); err != nil {
		t.Fatal(err)
	}
	if code := cmdMethod(dir, []string{"export-skill", "--target", "vim"}, &out, &errw); code != 1 {
		t.Fatal(code)
	}
}

func TestInitRerunKeepsExistingMethodAndStage(t *testing.T) {
	dir := initDir(t, "--method", "design-first")
	idx, _ := core.Open(dir)
	if err := idx.SetStage("build"); err != nil {
		t.Fatal(err)
	}
	var out, errw bytes.Buffer
	// Enter at a prompt would have picked freeform; an existing method means no prompt.
	if code := cmdInit([]string{dir}, strings.NewReader("\n"), true, &out, &errw); code != 0 {
		t.Fatal(errw.String())
	}
	if strings.Contains(out.String(), "Choose a workflow method") {
		t.Fatalf("prompted on an existing project:\n%s", out.String())
	}
	idx, _ = core.Open(dir)
	if idx.ActiveMethod().ID != "design-first" || idx.EffectiveStage() != "build" {
		t.Fatalf("%s %s", idx.ActiveMethod().ID, idx.EffectiveStage())
	}
	// A populated freeform project is not prompted either.
	free := initDir(t)
	fidx, _ := core.Open(free)
	_, _ = fidx.Create(core.CreateInput{Name: "Orders", Type: "service"})
	out.Reset()
	cmdInit([]string{free}, strings.NewReader("2\n"), true, &out, &errw)
	if strings.Contains(out.String(), "Choose") {
		t.Fatal("prompted on a populated project")
	}
	// An explicit --method on an existing project says what changed.
	out.Reset()
	if code := cmdInit([]string{dir, "--method", "map-existing"}, nil, false, &out, &errw); code != 0 {
		t.Fatal(errw.String())
	}
	if !strings.Contains(out.String(), "method changed: design-first -> map-existing") || !strings.Contains(out.String(), "cleared") {
		t.Fatalf("%s", out.String())
	}
}

func TestInitUnknownMethodCreatesNothing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errw bytes.Buffer
	if code := cmdInit([]string{dir, "--method", "nope"}, nil, false, &out, &errw); code != 1 {
		t.Fatalf("%d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, ".arch")); !os.IsNotExist(err) {
		t.Fatalf(".arch was scaffolded: %v", err)
	}
}

func brokenMethodDir(t *testing.T) string {
	t.Helper()
	dir := initDir(t)
	if err := os.MkdirAll(filepath.Join(dir, ".arch", "methods"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".arch", "methods", "mine.json"), []byte(`{broken`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".arch", "config.json"), []byte(`{"method":"mine"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCheckModelStrictFailsWhenMethodDidNotLoad(t *testing.T) {
	dir := brokenMethodDir(t)
	var out, errw bytes.Buffer
	if code := cmdCheckModel(dir, nil, &out, &errw); code != 0 {
		t.Fatalf("non-strict %d %s", code, errw.String())
	}
	if !strings.Contains(errw.String(), "mine") || !strings.Contains(errw.String(), "skipped") {
		t.Fatalf("warnings not printed:\n%s", errw.String())
	}
	errw.Reset()
	if code := cmdCheckModel(dir, []string{"--strict"}, &out, &errw); code != 1 || !strings.Contains(errw.String(), "did not load") {
		t.Fatalf("strict %d\n%s", code, errw.String())
	}
	errw.Reset()
	if code := cmdStage(dir, []string{"show"}, &out, &errw); code != 0 || !strings.Contains(errw.String(), "mine") {
		t.Fatalf("stage warnings %d\n%s", code, errw.String())
	}
}

func TestCheckModelNoDriftOverridesMethod(t *testing.T) {
	dir := initDir(t, "--method", "map-existing")
	var out, errw bytes.Buffer
	cmdCheckModel(dir, nil, &out, &errw)
	if !strings.Contains(out.String()+errw.String(), "drift") {
		t.Fatalf("map-existing should run drift by default:\n%s%s", out.String(), errw.String())
	}
	out.Reset()
	errw.Reset()
	cmdCheckModel(dir, []string{"--no-drift"}, &out, &errw)
	if strings.Contains(out.String()+errw.String(), "drift") {
		t.Fatalf("--no-drift ignored:\n%s%s", out.String(), errw.String())
	}
}

func TestExportSkillRefusesToOverwriteForeignFile(t *testing.T) {
	dir := initDir(t)
	target := filepath.Join(dir, "mine.md")
	if err := os.WriteFile(target, []byte("hand written"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errw bytes.Buffer
	if code := cmdMethod(dir, []string{"export-skill", "--out", target}, &out, &errw); code != 1 || !strings.Contains(errw.String(), "--force") {
		t.Fatalf("%d %s", code, errw.String())
	}
	if b, _ := os.ReadFile(target); string(b) != "hand written" {
		t.Fatal("file was overwritten")
	}
	if code := cmdMethod(dir, []string{"export-skill", "--out", target, "--force"}, &out, &errw); code != 0 {
		t.Fatal(errw.String())
	}
	// A generated file is overwritten without --force.
	if code := cmdMethod(dir, []string{"export-skill", "--out", target}, &out, &errw); code != 0 {
		t.Fatal(errw.String())
	}
}

func TestParseInitArgsMCPFlags(t *testing.T) {
	_, _, m, err := parseInitArgs([]string{"--npx"})
	if err != nil || m != mcpNpx {
		t.Fatal(m, err)
	}
	_, _, m, err = parseInitArgs([]string{"x", "--no-mcp"})
	if err != nil || m != mcpSkip {
		t.Fatal(m, err)
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("%s: %v", data, err)
	}
	return doc
}

func TestInitWritesMCPConfig(t *testing.T) {
	dir := initDir(t)
	servers := readJSON(t, filepath.Join(dir, ".mcp.json"))["mcpServers"].(map[string]any)
	ambit := servers["ambit"].(map[string]any)
	if ambit["command"] != "ambit" || len(ambit["args"].([]any)) != 1 || ambit["args"].([]any)[0] != "mcp" {
		t.Fatalf("%v", ambit)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cursor", "mcp.json")); err == nil {
		t.Fatal("cursor config written without .cursor/")
	}
}

func TestInitMCPNpxAndSkip(t *testing.T) {
	dir := initDir(t, "--npx")
	ambit := readJSON(t, filepath.Join(dir, ".mcp.json"))["mcpServers"].(map[string]any)["ambit"].(map[string]any)
	if ambit["command"] != "npx" || len(ambit["args"].([]any)) != 3 {
		t.Fatalf("%v", ambit)
	}
	dir = initDir(t, "--no-mcp")
	if _, err := os.Stat(filepath.Join(dir, ".mcp.json")); err == nil {
		t.Fatal(".mcp.json written despite --no-mcp")
	}
}

func TestInitMCPMergesAndNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, ".cursor"), 0o755)
	other := `{"mcpServers":{"other":{"command":"x"}},"extra":1}`
	_ = os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(other), 0o644)
	custom := `{"mcpServers":{"ambit":{"command":"custom"}}}`
	_ = os.WriteFile(filepath.Join(dir, ".cursor", "mcp.json"), []byte(custom), 0o644)
	var out, errw bytes.Buffer
	if code := cmdInit([]string{dir}, strings.NewReader(""), false, &out, &errw); code != 0 {
		t.Fatal(errw.String())
	}
	doc := readJSON(t, filepath.Join(dir, ".mcp.json"))
	servers := doc["mcpServers"].(map[string]any)
	if servers["other"] == nil || servers["ambit"] == nil || doc["extra"] == nil {
		t.Fatalf("%v", doc)
	}
	cur := readJSON(t, filepath.Join(dir, ".cursor", "mcp.json"))["mcpServers"].(map[string]any)["ambit"].(map[string]any)
	if cur["command"] != "custom" {
		t.Fatalf("existing ambit entry overwritten: %v", cur)
	}
	// Invalid JSON is left alone.
	bad := t.TempDir()
	_ = os.WriteFile(filepath.Join(bad, ".mcp.json"), []byte("{nope"), 0o644)
	if code := cmdInit([]string{bad}, strings.NewReader(""), false, &out, &errw); code != 0 {
		t.Fatal(errw.String())
	}
	if data, _ := os.ReadFile(filepath.Join(bad, ".mcp.json")); string(data) != "{nope" {
		t.Fatalf("%s", data)
	}
}

func TestConfigDriftIgnoreAdd(t *testing.T) {
	dir := initDir(t, "--no-mcp")
	var out, errw bytes.Buffer
	if code := cmdConfig(dir, []string{"drift-ignore", "add", "dist/**"}, &out, &errw); code != 0 {
		t.Fatal(errw.String())
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".arch", "config.json"))
	if !strings.Contains(string(data), "dist/**") {
		t.Fatalf("%s", data)
	}
	if code := cmdConfig(dir, []string{"drift-ignore", "add"}, &out, &errw); code != 1 {
		t.Fatal("usage error expected")
	}
}
