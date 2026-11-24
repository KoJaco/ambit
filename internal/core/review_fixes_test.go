package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWatchReloadsConstraintsOnConfigChange(t *testing.T) {
	idx, dir := newModel(t)
	nmMake(t, idx, CreateInput{ID: "api", Name: "API", Type: "service"})
	other := reopen(t, dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got := make(chan WatchNotice, 4)
	if err := other.WatchNotices(ctx, func(n WatchNotice) { got <- n }); err != nil {
		t.Fatal(err)
	}
	if err := idx.SetConstraints([]Constraint{{ID: "scope-user", Text: "scope queries by user_id"}}); err != nil {
		t.Fatal(err)
	}
	select {
	case n := <-got:
		if n.Err != nil || !n.ModelChanged || len(n.NodeIDs) != 0 {
			t.Fatalf("notice %+v", n)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no notice for config.json change")
	}
	brief, err := other.Brief("api")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(brief, "Project rules") || !strings.Contains(brief, "scope-user") {
		t.Fatalf("brief missing rules:\n%s", brief)
	}
}

func cycleModel(t *testing.T) *Index {
	idx, _ := newModel(t)
	nmMake(t, idx, CreateInput{ID: "p", Name: "P", Type: "group"})
	nmMake(t, idx, CreateInput{ID: "a", Name: "A", Type: "service", ParentID: "p"})
	nmMake(t, idx, CreateInput{ID: "x", Name: "X", Type: "service"})
	return idx
}

func TestDependencyCycleThroughPureContainer(t *testing.T) {
	// x depends on pure container p, then p's child a depends on x.
	idx := cycleModel(t)
	deps := []NodeID{"p"}
	if err := idx.Update("x", UpdateInput{DependsOn: &deps}); err != nil {
		t.Fatal(err)
	}
	back := []NodeID{"x"}
	if err := idx.Update("a", UpdateInput{DependsOn: &back}); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("update err %v", err)
	}
	if _, err := idx.Stage(StageInput{Source: SourceSeedModel, Ops: []StageOp{{Op: OpUpdateNode, NodeID: "a", Update: UpdateInput{DependsOn: &back}}}}); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("stage err %v", err)
	}
	if _, err := idx.Create(CreateInput{ID: "b", Name: "B", Type: "service", ParentID: "p", DependsOn: []NodeID{"x"}}); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("create err %v", err)
	}
	if _, err := idx.Stage(StageInput{Source: SourceSeedModel, Ops: []StageOp{{Op: OpCreateNode, Create: CreateInput{ID: "b", Name: "B", Type: "service", ParentID: "p", DependsOn: []NodeID{"x"}}}}}); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("stage create err %v", err)
	}

	// The other order: a depends on x first, then x on p.
	idx = cycleModel(t)
	if err := idx.Update("a", UpdateInput{DependsOn: &back}); err != nil {
		t.Fatal(err)
	}
	if err := idx.Update("x", UpdateInput{DependsOn: &deps}); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("update err %v", err)
	}
	if _, err := idx.Stage(StageInput{Source: SourceSeedModel, Ops: []StageOp{{Op: OpUpdateNode, NodeID: "x", Update: UpdateInput{DependsOn: &deps}}}}); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("stage err %v", err)
	}
}

func TestDependencyCycleSiblingsDependingOnParent(t *testing.T) {
	idx, _ := newModel(t)
	nmMake(t, idx, CreateInput{ID: "p", Name: "P", Type: "group"})
	nmMake(t, idx, CreateInput{ID: "a", Name: "A", Type: "service", ParentID: "p", DependsOn: []NodeID{"p"}})
	if _, err := idx.Create(CreateInput{ID: "b", Name: "B", Type: "service", ParentID: "p", DependsOn: []NodeID{"p"}}); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("create err %v", err)
	}
	if _, err := idx.Stage(StageInput{Source: SourceSeedModel, Ops: []StageOp{{Op: OpCreateNode, Create: CreateInput{ID: "b", Name: "B", Type: "service", ParentID: "p", DependsOn: []NodeID{"p"}}}}}); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("stage err %v", err)
	}
	nmMake(t, idx, CreateInput{ID: "c", Name: "C", Type: "service", ParentID: "p"})
	deps := []NodeID{"p"}
	if err := idx.Update("c", UpdateInput{DependsOn: &deps}); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("update err %v", err)
	}
}

func TestDiffMarksClearedEdgeContract(t *testing.T) {
	idx, _ := newModel(t)
	nmMake(t, idx, CreateInput{ID: "web", Name: "Web", Type: "frontend"})
	nmMake(t, idx, CreateInput{ID: "api", Name: "API", Type: "service"})
	rel, err := idx.SetRelationship(SetRelationshipInput{From: "web", To: "api", Label: "calls", Kind: KindSync, Contract: &EdgeContract{Text: "v1"}})
	if err != nil {
		t.Fatal(err)
	}
	pid := nmStage(t, idx,
		StageOp{Op: OpSetRelationship, From: "web", To: "api", RelID: rel.ID, Label: "calls", Kind: KindSync, Contract: &EdgeContract{}},
		StageOp{Op: OpSetRelationship, From: "web", To: "api", RelID: rel.ID, Label: "calls", Kind: KindSync},
	)
	diff, err := idx.ProposalDiff(pid)
	if err != nil {
		t.Fatal(err)
	}
	if r := diff.Operations[0].Relationship; r.Contract != nil || !r.ContractCleared {
		t.Fatalf("clear row %+v", r)
	}
	if r := diff.Operations[1].Relationship; r.ContractCleared {
		t.Fatalf("unchanged row %+v", r)
	}
	raw, _ := json.Marshal(diff.Operations[1].Relationship)
	if strings.Contains(string(raw), "contract_cleared") {
		t.Fatalf("contract_cleared must be omitted: %s", raw)
	}
}

func TestContractUnknownKeysRoundTrip(t *testing.T) {
	var nc NodeContract
	in := `{"payloads":"p","future":{"a":[1,2]},"errors":["e"]}`
	if err := json.Unmarshal([]byte(in), &nc); err != nil {
		t.Fatal(err)
	}
	if nc.Payloads != "p" || len(nc.Errors) != 1 || len(nc.Unknown) != 1 {
		t.Fatalf("decoded %+v", nc)
	}
	out, _ := json.Marshal(nc)
	if string(out) != `{"payloads":"p","errors":["e"],"future":{"a":[1,2]}}` {
		t.Fatalf("out %s", out)
	}
	var ec EdgeContract
	if err := json.Unmarshal([]byte(`{"text":"t","z":1,"y":"s"}`), &ec); err != nil {
		t.Fatal(err)
	}
	out, _ = json.Marshal(ec)
	if string(out) != `{"text":"t","y":"s","z":1}` {
		t.Fatalf("out %s", out)
	}
	var c Constraint
	if err := json.Unmarshal([]byte(`{"id":"a","text":"t","severity":"must","owner":"sec"}`), &c); err != nil {
		t.Fatal(err)
	}
	out, _ = json.Marshal(c)
	if string(out) != `{"id":"a","text":"t","severity":"must","owner":"sec"}` {
		t.Fatalf("out %s", out)
	}
	// Known-key-only values are unchanged in shape.
	out, _ = json.Marshal(NodeContract{Payloads: "p"})
	if string(out) != `{"payloads":"p"}` {
		t.Fatalf("out %s", out)
	}
	// Unknown keys do not bypass validation of known keys.
	if err := validateEdgeContract(&EdgeContract{Format: "bogus", Unknown: map[string]jsonRaw{"k": jsonRaw("1")}}); !errors.Is(err, ErrBadContract) {
		t.Fatalf("err %v", err)
	}
}

func TestContractUnknownKeysPreservedOnDisk(t *testing.T) {
	idx, dir := newModel(t)
	nmMake(t, idx, CreateInput{ID: "api", Name: "API", Type: "service", Contract: &NodeContract{Payloads: "p"}})
	nmMake(t, idx, CreateInput{ID: "web", Name: "Web", Type: "frontend"})
	rel, err := idx.SetRelationship(SetRelationshipInput{From: "web", To: "api", Label: "calls", Kind: KindSync, Contract: &EdgeContract{Text: "t"}})
	if err != nil {
		t.Fatal(err)
	}

	nodePath := filepath.Join(dir, ".arch", "nodes", "api.json")
	obj := readObj(t, nodePath)
	obj["contract"].(map[string]any)["x_node"] = "keep"
	writeObj(t, nodePath, obj)

	indexPath := filepath.Join(dir, ".arch", "index.json")
	iobj := readObj(t, indexPath)
	iobj["relationships"].([]any)[0].(map[string]any)["contract"].(map[string]any)["x_edge"] = 7
	writeObj(t, indexPath, iobj)

	cfgPath := filepath.Join(dir, ".arch", "config.json")
	cobj := readObj(t, cfgPath)
	cobj["constraints"] = []any{map[string]any{"id": "r1", "text": "t", "severity": "must", "x_c": true}}
	writeObj(t, cfgPath, cobj)

	idx = reopen(t, dir)
	name := "API 2"
	if err := idx.Update("api", UpdateInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	// An edit that re-sends the contract without the unknown key keeps it.
	if err := idx.Update("api", UpdateInput{Contract: &NodeContract{Payloads: "p2"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.SetRelationship(SetRelationshipInput{ID: rel.ID, From: "web", To: "api", Label: "calls2", Kind: KindSync}); err != nil {
		t.Fatal(err)
	}
	if err := idx.SetConstraints([]Constraint{{ID: "r1", Text: "t2"}}); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		nodePath:  `"x_node":"keep"`,
		indexPath: `"x_edge":7`,
		cfgPath:   `"x_c":true`,
	} {
		if data := mustRead(t, path); !strings.Contains(string(data), want) {
			t.Fatalf("%s lost unknown key:\n%s", path, data)
		}
	}
}

func TestSavedJSONIsNotHTMLEscaped(t *testing.T) {
	idx, dir := newModel(t)
	nmMake(t, idx, CreateInput{ID: "api", Name: "API", Type: "service", Contract: &NodeContract{Payloads: "List<Order> & more"}})
	nmMake(t, idx, CreateInput{ID: "web", Name: "Web", Type: "frontend"})
	if _, err := idx.SetRelationship(SetRelationshipInput{From: "web", To: "api", Label: "a<b", Kind: KindSync, Contract: &EdgeContract{Text: "Map<K,V> & co"}}); err != nil {
		t.Fatal(err)
	}
	if err := idx.SetConstraints([]Constraint{{ID: "generics", Text: "return List<Order> & never null"}}); err != nil {
		t.Fatal(err)
	}
	pid := nmStage(t, idx, StageOp{Op: OpSetConstraints, Constraints: []Constraint{{ID: "g2", Text: "A<B>"}}})
	for _, path := range []string{
		filepath.Join(dir, ".arch", "nodes", "api.json"),
		filepath.Join(dir, ".arch", "index.json"),
		filepath.Join(dir, ".arch", "config.json"),
		filepath.Join(dir, ".arch", ".proposals", pid, "manifest.json"),
	} {
		data := string(mustRead(t, path))
		if strings.Contains(data, "\\u003c") || strings.Contains(data, "\\u0026") || strings.Contains(data, "\\u003e") {
			t.Fatalf("%s is HTML-escaped:\n%q", path, data)
		}
	}
	if data := string(mustRead(t, filepath.Join(dir, ".arch", "config.json"))); !strings.Contains(data, "List<Order>") {
		t.Fatalf("config:\n%s", data)
	}
	if _, err := os.Stat(filepath.Join(dir, ".arch", "nodes", "api.json")); err != nil {
		t.Fatal(err)
	}
	re := reopen(t, dir)
	if re.Nodes["api"].Contract.Payloads != "List<Order> & more" {
		t.Fatalf("payloads %q", re.Nodes["api"].Contract.Payloads)
	}
}

func TestSetThenDeleteRelationshipInOneProposalNotStale(t *testing.T) {
	idx, _ := newModel(t)
	nmMake(t, idx, CreateInput{ID: "web", Name: "Web", Type: "frontend"})
	nmMake(t, idx, CreateInput{ID: "api", Name: "API", Type: "service"})
	rel, _ := idx.SetRelationship(SetRelationshipInput{From: "web", To: "api", Label: "calls", Kind: KindSync})
	pid := nmStage(t, idx,
		StageOp{Op: OpSetRelationship, From: "web", To: "api", RelID: rel.ID, Label: "renamed", Kind: KindSync, Contract: &EdgeContract{Text: "v1"}},
		StageOp{Op: OpDeleteRelationship, RelID: rel.ID},
	)
	res, err := idx.AcceptAll(pid)
	if err != nil || len(res.Applied) != 2 || len(res.Skipped) != 0 || len(res.Failed) != 0 {
		t.Fatalf("res %+v %v", res, err)
	}
	if r, _ := idx.relByID(rel.ID); r != nil {
		t.Fatalf("edge still present: %+v", r)
	}
}

func TestStageDeleteRelationshipChecksInteriorDependents(t *testing.T) {
	idx, _ := newModel(t)
	nmMake(t, idx, CreateInput{ID: "web", Name: "Web", Type: "frontend"})
	nmMake(t, idx, CreateInput{ID: "api", Name: "API", Type: "service"})
	rel, _ := idx.SetRelationship(SetRelationshipInput{From: "web", To: "api", Label: "calls", Kind: KindSync})
	nmMake(t, idx, CreateInput{ID: "mid", Name: "Mid", Type: "service", RelationshipID: rel.ID})
	nmMake(t, idx, CreateInput{ID: "user", Name: "User", Type: "service", DependsOn: []NodeID{"mid"}})
	_, err := idx.Stage(StageInput{Source: SourceSeedModel, Ops: []StageOp{{Op: OpDeleteRelationship, RelID: rel.ID}}})
	if !errors.Is(err, ErrHasDependents) {
		t.Fatalf("err %v", err)
	}
}

func TestStaleDeleteRelationshipWhenInteriorGainsDependent(t *testing.T) {
	idx, _ := newModel(t)
	nmMake(t, idx, CreateInput{ID: "web", Name: "Web", Type: "frontend"})
	nmMake(t, idx, CreateInput{ID: "api", Name: "API", Type: "service"})
	rel, _ := idx.SetRelationship(SetRelationshipInput{From: "web", To: "api", Label: "calls", Kind: KindSync})
	nmMake(t, idx, CreateInput{ID: "mid", Name: "Mid", Type: "service", RelationshipID: rel.ID})
	pid := nmStage(t, idx, StageOp{Op: OpDeleteRelationship, RelID: rel.ID})
	// A dependent on the interior node appears (and its own file does not touch mid's).
	nmMake(t, idx, CreateInput{ID: "user", Name: "User", Type: "service", DependsOn: []NodeID{"mid"}})
	res, err := idx.AcceptAll(pid)
	if err != nil || len(res.Applied) != 0 || len(res.Failed) != 0 || len(res.Skipped) != 1 {
		t.Fatalf("res %+v %v", res, err)
	}
	if !strings.Contains(res.Skipped[0].Reason, "dependents") {
		t.Fatalf("reason %q", res.Skipped[0].Reason)
	}
}

func TestBriefBuildOrderIsContainerAwareAndCascades(t *testing.T) {
	idx, _ := newModel(t)
	nmMake(t, idx, CreateInput{ID: "grp", Name: "Group", Type: "group"})
	nmMake(t, idx, CreateInput{ID: "g1", Name: "G1", Type: "service", ParentID: "grp"})
	nmMake(t, idx, CreateInput{ID: "db", Name: "DB", Type: "database"})
	nmMake(t, idx, CreateInput{ID: "q", Name: "Q", Type: "group", DependsOn: []NodeID{"grp", "db"}})
	nmMake(t, idx, CreateInput{ID: "q1", Name: "Q1", Type: "service", ParentID: "q"})

	// Pure container q is not in the build order: fallback uses declared deps and the done rule.
	brief, err := idx.Brief("q")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(brief, "Not done yet") || !strings.Contains(brief, `"Group" (grp)`) || !strings.Contains(brief, `"DB" (db)`) {
		t.Fatalf("q brief:\n%s", brief)
	}
	done := StatusDone
	for _, id := range []NodeID{"g1", "db"} {
		if err := idx.Update(id, UpdateInput{Status: &done}); err != nil {
			t.Fatal(err)
		}
	}
	// grp's own status is draft, but its child is done, so it counts as done.
	brief, _ = idx.Brief("q")
	if strings.Contains(brief, "Not done yet") {
		t.Fatalf("grp should count as done via children:\n%s", brief)
	}

	// Dependents cascade: q's depends_on applies to its child q1, and a dependency on a
	// pure container means its buildable descendants.
	db, _ := idx.Brief("db")
	if !strings.Contains(db, `"Q1" (q1)`) {
		t.Fatalf("db brief should list cascaded dependent q1:\n%s", db)
	}
	g1, _ := idx.Brief("g1")
	if !strings.Contains(g1, `"Q1" (q1)`) {
		t.Fatalf("g1 brief should list q1 via container grp:\n%s", g1)
	}
}

func TestBriefInterfacesSectionIsCapped(t *testing.T) {
	idx, _ := newModel(t)
	nmMake(t, idx, CreateInput{ID: "api", Name: "API", Type: "service"})
	for i := 1; i <= 20; i++ {
		id := fmt.Sprintf("c%d", i)
		nmMake(t, idx, CreateInput{ID: NodeID(id), Name: id, Type: "frontend"})
		if _, err := idx.SetRelationship(SetRelationshipInput{From: NodeID(id), To: "api", Label: "calls " + id, Kind: KindSync, Contract: &EdgeContract{Text: strings.Repeat("x", 3900)}}); err != nil {
			t.Fatal(err)
		}
	}
	brief, err := idx.Brief("api")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(brief, "Interfaces you must honour")
	if start < 0 {
		t.Fatal("no interfaces section")
	}
	end := strings.Index(brief[start:], "\nCall the check_scope")
	section := brief[start : start+end]
	if len([]rune(section)) > interfacesCap+300 {
		t.Fatalf("section too long: %d", len([]rune(section)))
	}
	if !strings.Contains(section, "omitted") || !strings.Contains(section, "get_graph") {
		t.Fatalf("missing pointer:\n%s", section)
	}
}
