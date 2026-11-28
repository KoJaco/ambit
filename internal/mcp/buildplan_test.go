package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KoJaco/ambit/internal/core"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func planSession(t *testing.T) (*sdkmcp.ClientSession, *core.Index, string) {
	t.Helper()
	idx, dir := openFixture(t)
	gitInit(t, dir)
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "ambit", Version: "v1"}, nil)
	registerTools(srv, &toolEnv{idx: idx, dir: dir})
	t1, t2 := sdkmcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, t1, nil); err != nil {
		t.Fatal(err)
	}
	session, err := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test", Version: "v1"}, nil).Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session, idx, dir
}

func callTool(t *testing.T, s *sdkmcp.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()
	res, err := s.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s: tool error %+v", name, res.Content)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestReadOnlyToolsAreAnnotated(t *testing.T) {
	s, _, _ := planSession(t)
	res, err := s.ListTools(context.Background(), &sdkmcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	readOnlyTools := map[string]bool{
		"get_context": true, "check_scope": true, "get_graph": true, "list_proposals": true, "check_done": true,
		"get_proposal": true, "wait_for_proposal": true, "list_icons": true, "check_drift": true, "get_build_order": true,
		"get_started": true, "check_model": true, "check_deliverables": true,
		"check_contract_sync": true,
	}
	seen := 0
	for _, tool := range res.Tools {
		got := tool.Annotations != nil && tool.Annotations.ReadOnlyHint
		if got != readOnlyTools[tool.Name] {
			t.Errorf("%s: ReadOnlyHint=%v, want %v", tool.Name, got, readOnlyTools[tool.Name])
		}
		if readOnlyTools[tool.Name] {
			seen++
		}
		if tool.Name == "run_verify" || tool.Name == "check_dependents" {
			// They run shell commands: never read-only, flagged destructive and open-world.
			a := tool.Annotations
			if a == nil || a.ReadOnlyHint || a.DestructiveHint == nil || !*a.DestructiveHint || a.OpenWorldHint == nil || !*a.OpenWorldHint {
				t.Errorf("%s: want non-read-only exec annotations, got %+v", tool.Name, a)
			}
		}
		switch tool.Name {
		case "create_node", "update_node", "seed_model":
			raw, _ := json.Marshal(tool.InputSchema)
			for _, f := range []string{`"human_gates"`, `"deliverables"`, `"milestones"`, `"gates"`, `"external"`, `"shared"`, `"references"`} {
				if !strings.Contains(string(raw), f) {
					t.Errorf("%s schema lacks %s", tool.Name, f)
				}
			}
		}
	}
	if seen != len(readOnlyTools) {
		t.Fatalf("saw %d of %d read-only tools", seen, len(readOnlyTools))
	}
}

func TestBuildOrderToolCompactReadyOnlyAndShape(t *testing.T) {
	s, idx, _ := planSession(t)
	for _, in := range []core.CreateInput{
		{ID: "ext", Name: "Ext", Type: "external", NodeExtras: core.NodeExtras{External: true}},
		{ID: "a", Name: "A", Type: "service", Implementation: []string{"a/**"}},
		{ID: "b", Name: "B", Type: "service", Implementation: []string{"b/**"}, DependsOn: []core.NodeID{"a"}},
		{ID: "c", Name: "C", Type: "service", Implementation: []string{"c/**"}, NodeExtras: core.NodeExtras{HumanGates: []core.HumanGate{{Kind: "decision"}}}},
	} {
		if _, err := idx.Create(in); err != nil {
			t.Fatal(err)
		}
	}
	full := callTool(t, s, "get_build_order", map[string]any{})
	if full["max_parallel"].(float64) != 2 || len(full["externals"].([]any)) != 1 || len(full["ready_now"].([]any)) != 2 {
		t.Fatalf("full %v", full)
	}
	item := full["order"].([]any)[2].(map[string]any)
	if item["class"] == nil || item["name"] == nil {
		t.Fatalf("full item %v", item)
	}
	compact := callTool(t, s, "get_build_order", map[string]any{"compact": true})
	ci := compact["order"].([]any)[0].(map[string]any)
	if _, has := ci["name"]; has || ci["class"] == nil || ci["wave"] == nil {
		t.Fatalf("compact item %v", ci)
	}
	if ids := compact["externals"].([]any); len(ids) != 1 || ids[0] != "ext" {
		t.Fatalf("compact externals %v", compact["externals"])
	}
	ready := callTool(t, s, "get_build_order", map[string]any{"ready_only": true})
	if got := ready["order"].([]any); len(got) != 2 {
		t.Fatalf("ready_only %v", got)
	}
}

func TestPlanToolsEndToEnd(t *testing.T) {
	s, idx, dir := planSession(t)
	if _, err := idx.Create(core.CreateInput{ID: "n", Name: "N", Type: "service", Implementation: []string{"n/**"},
		Verify: []string{"echo verified"}, NodeExtras: core.NodeExtras{Deliverables: []string{"docs/n.md"}, References: []string{"docs/ref.md"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Create(core.CreateInput{ID: "dep", Name: "Dep", Type: "service", DependsOn: []core.NodeID{"n"}, Verify: []string{"echo dep-ok"}}); err != nil {
		t.Fatal(err)
	}
	d := callTool(t, s, "check_deliverables", map[string]any{"node_id": "n"})
	if d["pass"] != false || d["declared"].(float64) != 1 {
		t.Fatalf("deliverables %v", d)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "n.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if d = callTool(t, s, "check_deliverables", map[string]any{"node_id": "n"}); d["pass"] != true {
		t.Fatalf("deliverables after write %v", d)
	}
	v := callTool(t, s, "run_verify", map[string]any{"node_id": "n", "timeout_s": 5})
	if v["pass"] != true || !strings.Contains(v["results"].([]any)[0].(map[string]any)["output"].(string), "verified") {
		t.Fatalf("run_verify %v", v)
	}
	dep := callTool(t, s, "check_dependents", map[string]any{"node_id": "n"})
	if dep["pass"] != true || len(dep["dependents"].([]any)) != 1 {
		t.Fatalf("check_dependents %v", dep)
	}
	cs := callTool(t, s, "check_contract_sync", map[string]any{"node_id": "n"})
	if cs["pass"] != false || !strings.Contains(strings.Join(anyStrings(cs["problems"]), "|"), "docs/ref.md does not exist") {
		t.Fatalf("check_contract_sync %v", cs)
	}
	done := callTool(t, s, "check_done", map[string]any{"node_id": "n"})
	if done["deliverables"] == nil {
		t.Fatalf("check_done lacks deliverables: %v", done)
	}
	res, err := s.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "run_verify", Arguments: map[string]any{"node_id": "ghost"}})
	if err != nil || !res.IsError {
		t.Fatalf("unknown node should be a tool error: %v %+v", err, res)
	}
}

func anyStrings(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

func TestSeedModelAcceptsBuildPlanFields(t *testing.T) {
	idx, dir := openFixture(t)
	env := &toolEnv{idx: idx, dir: dir}
	_, out, err := env.handleSeedModel(context.Background(), nil, seedModelIn{Transcript: "plan", Nodes: []seedNodeIn{
		{ID: "spike", Name: "Spike", Type: "service", NodeExtras: core.NodeExtras{Gates: []core.NodeID{"app"}, Milestones: []core.Milestone{{ID: "m"}}}},
		{ID: "app", Name: "App", Type: "service", NodeExtras: core.NodeExtras{
			External: false, HumanGates: []core.HumanGate{{Kind: "phone", Note: "call"}}, Deliverables: []string{"docs/app.md"}, Shared: true}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idx.AcceptAll(out.ProposalID); err != nil {
		t.Fatal(err)
	}
	app := idx.Nodes["app"]
	if app == nil || len(app.HumanGates) != 1 || !app.Shared || len(idx.Nodes["spike"].Gates) != 1 {
		t.Fatalf("nodes %+v %+v", app, idx.Nodes["spike"])
	}
	// update_node patches and clears.
	empty := []string{}
	_, out, err = env.handleUpdateNode(context.Background(), nil, updateNodeIn{NodeID: "app", NodeExtrasPatch: core.NodeExtrasPatch{Deliverables: &empty}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idx.AcceptAll(out.ProposalID); err != nil {
		t.Fatal(err)
	}
	if len(idx.Nodes["app"].Deliverables) != 0 || len(idx.Nodes["app"].HumanGates) != 1 {
		t.Fatalf("after patch %+v", idx.Nodes["app"])
	}
}
