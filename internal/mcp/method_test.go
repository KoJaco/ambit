package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KoJaco/ambit/internal/core"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGetStartedFreeform(t *testing.T) {
	idx, dir := openFixture(t)
	env := &toolEnv{idx: idx, dir: dir}
	_, out, err := env.handleGetStarted(context.Background(), nil, getStartedIn{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Method.ID != "freeform" || out.Stage != nil || out.Guidance == "" || len(out.Stages) != 0 || len(out.NextSteps) == 0 {
		t.Fatalf("%+v", out)
	}
	raw, _ := json.Marshal(out)
	if !strings.Contains(string(raw), `"stage":null`) || !strings.Contains(string(raw), `"stages":[]`) {
		t.Fatalf("%s", raw)
	}
}

func TestGetStartedDesignFirst(t *testing.T) {
	idx, dir := openFixture(t)
	if _, err := idx.SetMethod("design-first"); err != nil {
		t.Fatal(err)
	}
	env := &toolEnv{idx: idx, dir: dir}
	_, out, _ := env.handleGetStarted(context.Background(), nil, getStartedIn{})
	if out.Method.ID != "design-first" || out.Stage == nil || out.Stage.ID != "design" || len(out.Stages) != 2 {
		t.Fatalf("%+v", out)
	}
	joined := strings.Join(out.NextSteps, " ")
	if !strings.Contains(joined, "design") || !strings.Contains(joined, "refuses") {
		t.Fatalf("%v", out.NextSteps)
	}
	if err := idx.SetStage("build"); err != nil {
		t.Fatal(err)
	}
	_, out, _ = env.handleGetStarted(context.Background(), nil, getStartedIn{})
	if out.Stage.ID != "build" || !strings.Contains(strings.Join(out.NextSteps, " "), "get_build_order") {
		t.Fatalf("%+v", out)
	}
}

func TestUpdateNodeStatusStageGate(t *testing.T) {
	idx, dir := openFixture(t)
	id, err := idx.Create(core.CreateInput{Name: "Orders", Type: "service"})
	if err != nil {
		t.Fatal(err)
	}
	env := &toolEnv{idx: idx, dir: dir}
	set := func(status string) error {
		_, _, err := env.handleUpdateNodeStatus(context.Background(), nil, updateStatusIn{NodeID: string(id), Status: status})
		return err
	}
	// freeform never gates.
	for _, s := range []string{"assigned", "in_progress", "done"} {
		if err := set(s); err != nil {
			t.Fatalf("freeform %s: %v", s, err)
		}
	}
	if _, err := idx.SetMethod("design-first"); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"assigned", "in_progress", "done"} {
		if err := set(s); !errors.Is(err, core.ErrStageGate) {
			t.Fatalf("design %s: %v", s, err)
		}
	}
	for _, s := range []string{"draft", "specified", "blocked"} {
		if err := set(s); err != nil {
			t.Fatalf("design %s: %v", s, err)
		}
	}
	if err := idx.SetStage("build"); err != nil {
		t.Fatal(err)
	}
	if err := set("done"); err != nil {
		t.Fatalf("build done: %v", err)
	}
}

func TestCheckModelTool(t *testing.T) {
	idx, dir := openFixture(t)
	if _, err := idx.SetMethod("design-first"); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Create(core.CreateInput{Name: "Orders", Type: "service"}); err != nil {
		t.Fatal(err)
	}
	env := &toolEnv{idx: idx, dir: dir}
	_, res, err := env.handleCheckModel(context.Background(), nil, checkModelIn{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Method != "design-first" || res.Stage != "design" || res.OK || res.Counts.Enforced == 0 || res.Drift != nil || res.DriftError != "" {
		t.Fatalf("%+v", res)
	}
	no := false
	_, res, _ = env.handleCheckModel(context.Background(), nil, checkModelIn{IncludeDrift: &no, AllRules: true})
	if res.Counts.Advisory == 0 || res.Counts.Enforced != 0 {
		t.Fatalf("all_rules should be advisory: %+v", res.Counts)
	}
}

func TestCheckModelDriftDefaultFromMethod(t *testing.T) {
	idx, dir := openFixture(t)
	env := &toolEnv{idx: idx, dir: dir}
	_, res, _ := env.handleCheckModel(context.Background(), nil, checkModelIn{})
	if res.Drift != nil || res.DriftError != "" {
		t.Fatalf("freeform should not run drift: %+v", res)
	}
	if _, err := idx.SetMethod("map-existing"); err != nil {
		t.Fatal(err)
	}
	_, res, _ = env.handleCheckModel(context.Background(), nil, checkModelIn{})
	if res.Drift == nil && res.DriftError == "" {
		t.Fatalf("map-existing should run drift by default: %+v", res)
	}
	yes := true
	idx2, dir2 := openFixture(t)
	env2 := &toolEnv{idx: idx2, dir: dir2}
	_, res, _ = env2.handleCheckModel(context.Background(), nil, checkModelIn{IncludeDrift: &yes})
	if res.Drift == nil && res.DriftError == "" {
		t.Fatalf("explicit include_drift ignored: %+v", res)
	}
}

func TestGetGraphMethodAndStage(t *testing.T) {
	idx, dir := openFixture(t)
	env := &toolEnv{idx: idx, dir: dir}
	_, out, err := env.handleGetGraph(context.Background(), nil, getGraphIn{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Method.ID != "freeform" || out.Method.Name == "" || out.Stage != nil {
		t.Fatalf("%+v", out)
	}
	if _, err := idx.SetMethod("design-first"); err != nil {
		t.Fatal(err)
	}
	_, out, _ = env.handleGetGraph(context.Background(), nil, getGraphIn{})
	if out.Method.ID != "design-first" || out.Stage == nil || out.Stage.ID != "design" || out.Stage.Name == "" {
		t.Fatalf("%+v", out)
	}
}

func TestNoToolWritesStageOrMethod(t *testing.T) {
	idx, dir := openFixture(t)
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "ambit", Version: "v1"}, nil)
	registerTools(srv, &toolEnv{idx: idx, dir: dir})
	ctx := context.Background()
	t1, t2 := sdkmcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, t1, nil); err != nil {
		t.Fatal(err)
	}
	session, err := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test", Version: "v1"}, nil).Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	res, err := session.ListTools(ctx, &sdkmcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		raw, _ := json.Marshal(tool.InputSchema)
		var schema struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		_ = json.Unmarshal(raw, &schema)
		for prop := range schema.Properties {
			if prop == "stage" || prop == "method" || prop == "stage_id" || prop == "method_id" {
				t.Errorf("tool %s accepts %q", tool.Name, prop)
			}
		}
		if strings.HasPrefix(tool.Name, "set_stage") || strings.HasPrefix(tool.Name, "set_method") {
			t.Errorf("tool %s", tool.Name)
		}
	}
	// A status update must not touch config.json.
	cfg := filepath.Join(dir, ".arch", "config.json")
	before, _ := os.ReadFile(cfg)
	id, _ := idx.Create(core.CreateInput{Name: "X", Type: "service"})
	env := &toolEnv{idx: idx, dir: dir}
	_, _, _ = env.handleUpdateNodeStatus(ctx, nil, updateStatusIn{NodeID: string(id), Status: "done"})
	after, _ := os.ReadFile(cfg)
	if string(before) != string(after) {
		t.Fatalf("config changed:\n%s\n%s", before, after)
	}
}

func TestServerInstructionsFromMethod(t *testing.T) {
	idx, _ := openFixture(t)
	if _, err := idx.SetMethod("design-first"); err != nil {
		t.Fatal(err)
	}
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "ambit", Version: "v1"},
		&sdkmcp.ServerOptions{Instructions: idx.ActiveMethod().Guidance})
	ctx := context.Background()
	t1, t2 := sdkmcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, t1, nil); err != nil {
		t.Fatal(err)
	}
	session, err := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "t", Version: "v1"}, nil).Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if got := session.InitializeResult().Instructions; got != idx.ActiveMethod().Guidance || got == "" {
		t.Fatalf("instructions %q", got)
	}
}

func writeUserMethod(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, ".arch", "methods", name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func startedSteps(t *testing.T, idx *core.Index, dir string) string {
	t.Helper()
	env := &toolEnv{idx: idx, dir: dir}
	_, out, err := env.handleGetStarted(context.Background(), nil, getStartedIn{})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(out.NextSteps, "\n")
}

func TestGetStartedNextStepsFollowMethodData(t *testing.T) {
	idx, dir := openFixture(t)
	// Extends design-first under another id: same gate, so the same advice.
	writeUserMethod(t, dir, "ours.json", `{"schema_version":1,"id":"ours","extends":"design-first"}`)
	if err := idx.SetConfigKeys(map[string]any{"method": "ours"}); err != nil {
		t.Fatal(err)
	}
	steps := startedSteps(t, idx, dir)
	if !strings.Contains(steps, "refuses assigned, in_progress, needs_review, done") || !strings.Contains(steps, "to build") || !strings.Contains(steps, "enforced findings") || strings.Contains(steps, "get_build_order") {
		t.Fatalf("%s", steps)
	}
	if err := idx.SetStage("build"); err != nil {
		t.Fatal(err)
	}
	if steps := startedSteps(t, idx, dir); !strings.Contains(steps, "get_build_order") || strings.Contains(steps, "refuses") {
		t.Fatalf("%s", steps)
	}

	// A shadowed design-first with different stage ids and a different gate.
	writeUserMethod(t, dir, "design-first.json", `{"schema_version":1,"id":"design-first","stages":[{"id":"plan"},{"id":"ship"}],"gates":{"status":{"plan":["done"]}},"lint":{"icon-set":"advisory"}}`)
	idx2, err := core.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idx2.SetMethod("design-first"); err != nil {
		t.Fatal(err)
	}
	steps = startedSteps(t, idx2, dir)
	if !strings.Contains(steps, "Stage is plan.") || !strings.Contains(steps, "refuses done until a human advances the stage to ship") || !strings.Contains(steps, "advisory (rules: icon-set)") || strings.Contains(steps, "enforced findings") {
		t.Fatalf("%s", steps)
	}
	if err := idx2.SetStage("ship"); err != nil {
		t.Fatal(err)
	}
	if steps := startedSteps(t, idx2, dir); !strings.Contains(steps, "Stage is ship.") || !strings.Contains(steps, "get_build_order") {
		t.Fatalf("%s", steps)
	}
}

func TestUpdateNodeStatusSeesStageChangeFromAnotherProcess(t *testing.T) {
	idx, dir := openFixture(t)
	id, _ := idx.Create(core.CreateInput{Name: "Orders", Type: "service"})
	if _, err := idx.SetMethod("design-first"); err != nil {
		t.Fatal(err)
	}
	env := &toolEnv{idx: idx, dir: dir}
	set := func() error {
		_, _, err := env.handleUpdateNodeStatus(context.Background(), nil, updateStatusIn{NodeID: string(id), Status: "done"})
		return err
	}
	if err := set(); !errors.Is(err, core.ErrStageGate) {
		t.Fatal(err)
	}
	other, _ := core.Open(dir) // stands in for `ambit stage set build` in another process
	if err := other.SetStage("build"); err != nil {
		t.Fatal(err)
	}
	if err := set(); err != nil {
		t.Fatalf("stale in-memory stage: %v", err)
	}
}

func TestCheckModelToolMethodWarningAndExplicitDriftFalse(t *testing.T) {
	idx, dir := openFixture(t)
	env := &toolEnv{idx: idx, dir: dir}
	_, res, _ := env.handleCheckModel(context.Background(), nil, checkModelIn{})
	if res.MethodWarning != "" {
		t.Fatal(res.MethodWarning)
	}
	raw, _ := json.Marshal(res)
	if strings.Contains(string(raw), "method_warning") {
		t.Fatalf("must be omitted: %s", raw)
	}
	writeUserMethod(t, dir, "mine.json", `{broken`)
	if err := idx.SetConfigKeys(map[string]any{"method": "mine"}); err != nil {
		t.Fatal(err)
	}
	_, res, _ = env.handleCheckModel(context.Background(), nil, checkModelIn{})
	raw, _ = json.Marshal(res)
	if res.Method != "freeform" || !strings.Contains(string(raw), `"method_warning"`) {
		t.Fatalf("%s", raw)
	}
	if _, err := idx.SetMethod("map-existing"); err != nil {
		t.Fatal(err)
	}
	no := false
	_, res, _ = env.handleCheckModel(context.Background(), nil, checkModelIn{IncludeDrift: &no})
	if res.Drift != nil || res.DriftError != "" {
		t.Fatalf("explicit false ignored: %+v", res)
	}
}

func TestDriftIgnoreDiscoverable(t *testing.T) {
	m, _ := core.BuiltinMethod("map-existing")
	found := false
	for _, s := range nextSteps(m, "") {
		found = found || strings.Contains(s, "drift_ignore")
	}
	if !found {
		t.Fatal("nextSteps does not mention drift_ignore")
	}
	for name, d := range map[string]string{"seed_model": seedModelDescription, "check_model": checkModelDescription} {
		if !strings.Contains(d, "drift_ignore") {
			t.Errorf("%s description omits drift_ignore", name)
		}
	}
}
