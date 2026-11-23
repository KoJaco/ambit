package core

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func nmMake(t *testing.T, idx *Index, in CreateInput) NodeID {
	t.Helper()
	id, err := idx.Create(in)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func nmStage(t *testing.T, idx *Index, ops ...StageOp) string {
	t.Helper()
	id, err := idx.Stage(StageInput{Source: SourceSeedModel, Ops: ops})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestStageContractAndDependsOnAcceptAndSnapshots(t *testing.T) {
	idx, dir := newModel(t)
	nmMake(t, idx, CreateInput{ID: "db", Name: "DB", Type: "database"})
	contract := &NodeContract{Payloads: "POST /orders {id}", Errors: []string{"409 duplicate"}}
	pid := nmStage(t, idx,
		StageOp{Op: OpCreateNode, Create: CreateInput{ID: "api", Name: "API", Type: "service", DependsOn: []NodeID{"db"}, Contract: contract}},
		StageOp{Op: OpUpdateNode, NodeID: "db", Update: UpdateInput{Contract: &NodeContract{OwnsData: []string{"orders"}}}},
	)
	raw := string(mustRead(t, filepath.Join(dir, ".arch", ".proposals", pid, "nodes", "api.json")))
	if !strings.Contains(raw, `"depends_on"`) || !strings.Contains(raw, "POST /orders") {
		t.Fatalf("proposal file lacks new fields:\n%s", raw)
	}
	diff, err := idx.ProposalDiff(pid)
	if err != nil {
		t.Fatal(err)
	}
	p := diff.Operations[0].Proposed
	if len(p.DependsOn) != 1 || p.DependsOn[0] != "db" || p.Contract == nil || p.Contract.Payloads != "POST /orders {id}" {
		t.Fatalf("proposed snapshot %+v", p)
	}
	up := diff.Operations[1]
	if up.Current.Contract != nil || up.Proposed.Contract == nil || len(up.Fields) != 1 || up.Fields[0] != "contract" {
		t.Fatalf("update diff %+v / %+v / %v", up.Current, up.Proposed, up.Fields)
	}
	res, err := idx.AcceptAll(pid)
	if err != nil || len(res.Applied) != 2 || len(res.Failed) != 0 {
		t.Fatalf("accept %+v %v", res, err)
	}
	api := idx.Nodes["api"]
	if len(api.DependsOn) != 1 || api.DependsOn[0] != "db" || api.Contract == nil || api.Contract.Errors[0] != "409 duplicate" {
		t.Fatalf("api %+v", api)
	}
	if idx.Nodes["db"].Contract == nil || idx.Nodes["db"].Contract.OwnsData[0] != "orders" {
		t.Fatalf("db contract %+v", idx.Nodes["db"].Contract)
	}
	// Clearing depends_on and contract via update.
	empty := []NodeID{}
	pid = nmStage(t, idx, StageOp{Op: OpUpdateNode, NodeID: "api", Update: UpdateInput{DependsOn: &empty, Contract: &NodeContract{}}})
	if _, err := idx.AcceptAll(pid); err != nil {
		t.Fatal(err)
	}
	if n := idx.Nodes["api"]; len(n.DependsOn) != 0 || n.Contract != nil {
		t.Fatalf("not cleared %+v", n)
	}
}

func TestStageRejectsBadContractAndDependency(t *testing.T) {
	idx, _ := newModel(t)
	nmMake(t, idx, CreateInput{ID: "a", Name: "A", Type: "service"})
	_, err := idx.Stage(StageInput{Source: OpUpdateNode, Ops: []StageOp{{Op: OpUpdateNode, NodeID: "a",
		Update: UpdateInput{Contract: &NodeContract{Payloads: strings.Repeat("x", MaxContractPayloads+1)}}}}})
	if !errors.Is(err, ErrBadContract) {
		t.Fatalf("err %v", err)
	}
	for name, deps := range map[string][]NodeID{"self": {"a"}, "missing": {"nope"}} {
		d := deps
		_, err = idx.Stage(StageInput{Source: OpUpdateNode, Ops: []StageOp{{Op: OpUpdateNode, NodeID: "a", Update: UpdateInput{DependsOn: &d}}}})
		if !errors.Is(err, ErrBadDependency) {
			t.Fatalf("%s: err %v", name, err)
		}
	}
	_, err = idx.Stage(StageInput{Source: OpCreateNode, Ops: []StageOp{{Op: OpCreateNode, Create: CreateInput{Name: "B", Type: "service", DependsOn: []NodeID{"ghost"}}}}})
	if !errors.Is(err, ErrBadDependency) {
		t.Fatalf("create err %v", err)
	}
}

func TestStageDependencyCycleAcrossOpsIsCaught(t *testing.T) {
	idx, _ := newModel(t)
	nmMake(t, idx, CreateInput{ID: "a", Name: "A", Type: "service"})
	nmMake(t, idx, CreateInput{ID: "b", Name: "B", Type: "service"})
	ab, ba := []NodeID{"b"}, []NodeID{"a"}
	_, err := idx.Stage(StageInput{Source: SourceSeedModel, Ops: []StageOp{
		{Op: OpUpdateNode, NodeID: "a", Update: UpdateInput{DependsOn: &ab}},
		{Op: OpUpdateNode, NodeID: "b", Update: UpdateInput{DependsOn: &ba}},
	}})
	if !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("err %v", err)
	}
}

func TestAcceptAllCatchesCycleFormedByTwoProposalsAndRollsBack(t *testing.T) {
	idx, _ := newModel(t)
	nmMake(t, idx, CreateInput{ID: "a", Name: "A", Type: "service"})
	nmMake(t, idx, CreateInput{ID: "b", Name: "B", Type: "service"})
	nmMake(t, idx, CreateInput{ID: "c", Name: "C", Type: "service"})
	ab, ba, cc := []NodeID{"b"}, []NodeID{"a"}, "changed"
	first := nmStage(t, idx, StageOp{Op: OpUpdateNode, NodeID: "a", Update: UpdateInput{DependsOn: &ab}})
	second := nmStage(t, idx,
		StageOp{Op: OpUpdateNode, NodeID: "c", Update: UpdateInput{Spec: &cc}},
		StageOp{Op: OpUpdateNode, NodeID: "b", Update: UpdateInput{DependsOn: &ba}},
	)
	if res, err := idx.AcceptAll(first); err != nil || len(res.Applied) != 1 {
		t.Fatalf("first %+v %v", res, err)
	}
	res, err := idx.AcceptAll(second)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failed) != 1 || !strings.Contains(res.Failed[0].Error, "dependency cycle") || len(res.Applied) != 0 {
		t.Fatalf("result %+v", res)
	}
	if idx.Nodes["c"].Spec == cc {
		t.Fatal("earlier op was not rolled back")
	}
	if len(idx.Nodes["b"].DependsOn) != 0 {
		t.Fatalf("b %+v", idx.Nodes["b"])
	}
}

func TestStageDeleteRejectedWhileDependedOn(t *testing.T) {
	idx, _ := newModel(t)
	nmMake(t, idx, CreateInput{ID: "db", Name: "Database", Type: "database"})
	nmMake(t, idx, CreateInput{ID: "api", Name: "API", Type: "service", DependsOn: []NodeID{"db"}})
	_, err := idx.Stage(StageInput{Source: OpDeleteNode, Ops: []StageOp{{Op: OpDeleteNode, NodeID: "db"}}})
	if !errors.Is(err, ErrHasDependents) || !strings.Contains(err.Error(), "API (api)") {
		t.Fatalf("err %v", err)
	}
	// Deleting both in one proposal (dependent first) is allowed.
	if _, err := idx.Stage(StageInput{Source: SourceSeedModel, Ops: []StageOp{{Op: OpDeleteNode, NodeID: "api"}, {Op: OpDeleteNode, NodeID: "db"}}}); err != nil {
		t.Fatal(err)
	}
	// A dependent created after staging blocks accept.
	pid := nmStage(t, idx, StageOp{Op: OpDeleteNode, NodeID: "api"})
	nmMake(t, idx, CreateInput{ID: "web", Name: "Web", Type: "service", DependsOn: []NodeID{"api"}})
	// The op is now stale (reported, not applied) rather than failing a whole accept.
	if err := idx.AcceptOperation(pid, 0, true); !errors.Is(err, ErrStale) || !strings.Contains(err.Error(), "dependents") {
		t.Fatalf("accept err %v", err)
	}
	res, err := idx.AcceptAll(pid)
	if err != nil || len(res.Applied) != 0 || len(res.Failed) != 0 || len(res.Skipped) != 1 || res.Skipped[0].Reason != staleHasDependents {
		t.Fatalf("accept all %+v %v", res, err)
	}
	if idx.Nodes["api"] == nil {
		t.Fatal("api was deleted")
	}
}

func TestStageDependsOnNodeCreatedLaterInProposalWaitsForCreate(t *testing.T) {
	idx, _ := newModel(t)
	pid := nmStage(t, idx,
		StageOp{Op: OpCreateNode, Create: CreateInput{ID: "db", Name: "DB", Type: "database"}},
		StageOp{Op: OpCreateNode, Create: CreateInput{ID: "api", Name: "API", Type: "service", DependsOn: []NodeID{"db"}}},
	)
	if err := idx.RejectOperation(pid, 0); err != nil {
		t.Fatal(err)
	}
	res, err := idx.AcceptAll(pid)
	if err != nil || len(res.Applied) != 0 || len(res.Skipped) != 1 || res.Skipped[0].Reason != staleDependsOnNotAccepted {
		t.Fatalf("res %+v %v", res, err)
	}
}

func TestEdgeContractStageAcceptAndStaleness(t *testing.T) {
	idx, _ := newModel(t)
	nmMake(t, idx, CreateInput{ID: "web", Name: "Web", Type: "frontend"})
	nmMake(t, idx, CreateInput{ID: "api", Name: "API", Type: "service"})
	rel, err := idx.SetRelationship(SetRelationshipInput{From: "web", To: "api", Label: "calls", Kind: KindSync})
	if err != nil {
		t.Fatal(err)
	}
	ec := &EdgeContract{Format: ContractFormatOpenAPI, Ref: "docs/openapi.yaml", Text: "GET /orders"}
	pid := nmStage(t, idx, StageOp{Op: OpSetRelationship, From: "web", To: "api", RelID: rel.ID, Label: "calls", Kind: KindSync, Contract: ec})
	m, _ := readManifest(idx.mustProposalPath(pid))
	if !m.Operations[0].RelCovered || m.Operations[0].Contract == nil {
		t.Fatalf("op %+v", m.Operations[0])
	}
	diff, err := idx.ProposalDiff(pid)
	if err != nil {
		t.Fatal(err)
	}
	d := diff.Operations[0]
	if d.Relationship.Contract == nil || d.Relationship.Contract.Ref != "docs/openapi.yaml" || d.CurrentRelationship.Contract != nil || d.Stale {
		t.Fatalf("diff %+v / %+v", d.Relationship, d.CurrentRelationship)
	}
	if err := idx.AcceptOperation(pid, 0, false); err != nil {
		t.Fatal(err)
	}
	if r, _ := idx.relByID(rel.ID); r.Contract == nil || r.Contract.Text != "GET /orders" {
		t.Fatalf("rel %+v", r)
	}

	// Two proposals editing one edge: the second goes stale once the first is applied.
	one := nmStage(t, idx, StageOp{Op: OpSetRelationship, From: "web", To: "api", RelID: rel.ID, Label: "calls", Kind: KindSync, Contract: &EdgeContract{Text: "one"}})
	two := nmStage(t, idx, StageOp{Op: OpSetRelationship, From: "web", To: "api", RelID: rel.ID, Label: "calls", Kind: KindSync, Contract: &EdgeContract{Text: "two"}})
	if err := idx.AcceptOperation(one, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := idx.AcceptOperation(two, 0, false); !errors.Is(err, ErrStale) {
		t.Fatalf("err %v", err)
	}
	// A nil contract leaves the edge contract alone; an empty one clears it.
	keep := nmStage(t, idx, StageOp{Op: OpSetRelationship, From: "web", To: "api", RelID: rel.ID, Label: "renamed", Kind: KindSync})
	if err := idx.AcceptOperation(keep, 0, false); err != nil {
		t.Fatal(err)
	}
	if r, _ := idx.relByID(rel.ID); r.Contract == nil || r.Contract.Text != "one" || r.Label != "renamed" {
		t.Fatalf("rel %+v", r)
	}
	clr := nmStage(t, idx, StageOp{Op: OpSetRelationship, From: "web", To: "api", RelID: rel.ID, Label: "renamed", Kind: KindSync, Contract: &EdgeContract{}})
	if err := idx.AcceptOperation(clr, 0, false); err != nil {
		t.Fatal(err)
	}
	if r, _ := idx.relByID(rel.ID); r.Contract != nil {
		t.Fatalf("rel %+v", r)
	}
}

func TestEdgeContractValidationAndDeleteRelationshipStaleness(t *testing.T) {
	idx, _ := newModel(t)
	nmMake(t, idx, CreateInput{ID: "web", Name: "Web", Type: "frontend"})
	nmMake(t, idx, CreateInput{ID: "api", Name: "API", Type: "service"})
	_, err := idx.Stage(StageInput{Source: OpSetRelationship, Ops: []StageOp{{Op: OpSetRelationship, From: "web", To: "api", Contract: &EdgeContract{Ref: "../secret"}}}})
	if !errors.Is(err, ErrBadContract) {
		t.Fatalf("err %v", err)
	}
	rel, err := idx.SetRelationship(SetRelationshipInput{From: "web", To: "api", Label: "calls", Kind: KindSync})
	if err != nil {
		t.Fatal(err)
	}
	pid := nmStage(t, idx, StageOp{Op: OpDeleteRelationship, RelID: rel.ID})
	if _, err := idx.SetRelationship(SetRelationshipInput{ID: rel.ID, From: "web", To: "api", Label: "calls", Kind: KindSync, Contract: &EdgeContract{Text: "changed"}}); err != nil {
		t.Fatal(err)
	}
	sum, _ := idx.ListProposals()
	if !sum[0].Operations[0].Stale {
		t.Fatalf("delete should be stale after the edge contract changed: %+v", sum[0].Operations[0])
	}
	if err := idx.AcceptOperation(pid, 0, false); !errors.Is(err, ErrStale) {
		t.Fatalf("err %v", err)
	}
}

func TestSetRelationshipOpsInOneProposalDoNotGoStale(t *testing.T) {
	idx, _ := newModel(t)
	nmMake(t, idx, CreateInput{ID: "web", Name: "Web", Type: "frontend"})
	nmMake(t, idx, CreateInput{ID: "api", Name: "API", Type: "service"})
	rel, _ := idx.SetRelationship(SetRelationshipInput{From: "web", To: "api", Label: "calls", Kind: KindSync})
	pid := nmStage(t, idx,
		StageOp{Op: OpSetRelationship, From: "web", To: "api", RelID: rel.ID, Label: "calls", Kind: KindSync, Contract: &EdgeContract{Text: "v1"}},
		StageOp{Op: OpSetRelationship, From: "web", To: "api", RelID: rel.ID, Label: "calls", Kind: KindSync, Contract: &EdgeContract{Text: "v2"}},
	)
	res, err := idx.AcceptAll(pid)
	if err != nil || len(res.Applied) != 2 || len(res.Skipped) != 0 {
		t.Fatalf("res %+v %v", res, err)
	}
	if r, _ := idx.relByID(rel.ID); r.Contract.Text != "v2" {
		t.Fatalf("rel %+v", r)
	}
}

func TestSetConstraintsStageAcceptDiffAndStale(t *testing.T) {
	idx, dir := newModel(t)
	if err := idx.SetConstraints([]Constraint{{ID: "old", Text: "old rule"}}); err != nil {
		t.Fatal(err)
	}
	pid := nmStage(t, idx, StageOp{Op: OpSetConstraints, Constraints: []Constraint{
		{ID: "scope-user", Text: "every query scopes by user_id"},
		{ID: "no-log-secrets", Text: "no secrets in logs", Severity: ConstraintShould},
	}})
	diff, err := idx.ProposalDiff(pid)
	if err != nil {
		t.Fatal(err)
	}
	d := diff.Operations[0]
	if d.Op != OpSetConstraints || len(d.CurrentConstraints) != 1 || d.CurrentConstraints[0].ID != "old" ||
		len(d.ProposedConstraints) != 2 || d.ProposedConstraints[0].Severity != ConstraintMust || d.Stale {
		t.Fatalf("diff %+v", d)
	}
	if err := idx.AcceptOperation(pid, 0, false); err != nil {
		t.Fatal(err)
	}
	if len(idx.Constraints) != 2 || idx.Constraints[0].ID != "scope-user" {
		t.Fatalf("constraints %+v", idx.Constraints)
	}
	_ = dir

	// Concurrent config edit makes a staged replacement stale.
	pid = nmStage(t, idx, StageOp{Op: OpSetConstraints, Constraints: []Constraint{{ID: "only", Text: "only rule"}}})
	if err := idx.SetConstraints([]Constraint{{ID: "other", Text: "architect edit"}}); err != nil {
		t.Fatal(err)
	}
	sum, _ := idx.ListProposals()
	if !sum[0].Operations[0].Stale {
		t.Fatal("expected stale")
	}
	if err := idx.AcceptOperation(pid, 0, false); !errors.Is(err, ErrStale) {
		t.Fatalf("err %v", err)
	}
	if err := idx.AcceptOperation(pid, 0, true); err != nil {
		t.Fatal(err)
	}
	if len(idx.Constraints) != 1 || idx.Constraints[0].ID != "only" {
		t.Fatalf("constraints %+v", idx.Constraints)
	}
	// Empty list clears.
	pid = nmStage(t, idx, StageOp{Op: OpSetConstraints})
	if err := idx.AcceptOperation(pid, 0, false); err != nil || len(idx.Constraints) != 0 {
		t.Fatalf("clear %v %+v", err, idx.Constraints)
	}
}

func TestSetConstraintsValidationAndRollback(t *testing.T) {
	idx, _ := newModel(t)
	_, err := idx.Stage(StageInput{Source: OpSetConstraints, Ops: []StageOp{{Op: OpSetConstraints, Constraints: []Constraint{{ID: "Bad Id", Text: "x"}}}}})
	if !errors.Is(err, ErrBadConstraint) {
		t.Fatalf("err %v", err)
	}
	_, err = idx.Stage(StageInput{Source: SourceSeedModel, Ops: []StageOp{{Op: OpSetConstraints}, {Op: OpSetConstraints}}})
	if !errors.Is(err, ErrBadOperation) {
		t.Fatalf("err %v", err)
	}
	// A later failing op rolls the config back too.
	nmMake(t, idx, CreateInput{ID: "a", Name: "A", Type: "service"})
	nmMake(t, idx, CreateInput{ID: "b", Name: "B", Type: "service"})
	aDeps := []NodeID{"b"}
	pid := nmStage(t, idx,
		StageOp{Op: OpSetConstraints, Constraints: []Constraint{{ID: "r", Text: "rule"}}},
		StageOp{Op: OpUpdateNode, NodeID: "a", Update: UpdateInput{DependsOn: &aDeps}},
	)
	bDeps := []NodeID{"a"}
	if err := idx.Update("b", UpdateInput{DependsOn: &bDeps}); err != nil {
		t.Fatal(err)
	}
	res, err := idx.AcceptAll(pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failed) != 1 || len(idx.Constraints) != 0 {
		t.Fatalf("res %+v constraints %+v", res, idx.Constraints)
	}
}

func TestBriefProjectRulesInterfacesAndContract(t *testing.T) {
	idx, _ := newModel(t)
	nmMake(t, idx, CreateInput{ID: "web", Name: "Web", Type: "frontend", Implementation: []string{"web/**"}})
	nmMake(t, idx, CreateInput{ID: "api", Name: "API", Type: "service", Implementation: []string{"api/**"},
		Contract: &NodeContract{Payloads: strings.Repeat("P", 1500), Invariants: []string{"idempotent"}, Errors: []string{"409 dup"}, OwnsData: []string{"orders"}}})
	if _, err := idx.SetRelationship(SetRelationshipInput{From: "web", To: "api", Label: "orders", Kind: KindSync,
		Contract: &EdgeContract{Format: ContractFormatOpenAPI, Ref: "docs/api.yaml", Text: "POST /orders"}}); err != nil {
		t.Fatal(err)
	}
	if err := idx.SetConstraints([]Constraint{
		{ID: "soft", Text: "prefer small functions", Severity: ConstraintShould},
		{ID: "hard", Text: "scope by user_id"},
	}); err != nil {
		t.Fatal(err)
	}
	api, err := idx.Brief("api")
	if err != nil {
		t.Fatal(err)
	}
	web, _ := idx.Brief("web")

	rules := strings.Index(api, "Project rules")
	if rules < 0 || strings.Index(api, "[must] hard") > strings.Index(api, "[should] soft") || strings.Index(api, "[must] hard") < rules {
		t.Fatalf("rules missing or misordered:\n%s", api)
	}
	if !strings.Contains(api, "you PROVIDE this interface") || !strings.Contains(web, "you CONSUME this interface") {
		t.Fatalf("direction wrong:\napi:\n%s\nweb:\n%s", api, web)
	}
	for _, want := range []string{"format: openapi", "docs/api.yaml", "POST /orders"} {
		if !strings.Contains(api, want) || !strings.Contains(web, want) {
			t.Fatalf("missing %q", want)
		}
	}
	// Own contract is never truncated; sibling view is capped with a pointer.
	if !strings.Contains(api, strings.Repeat("P", 1500)) || !strings.Contains(api, "idempotent") || !strings.Contains(api, "Owns data") {
		t.Fatalf("own contract incomplete:\n%s", api)
	}
	if strings.Contains(web, strings.Repeat("P", 900)) || !strings.Contains(web, "get_graph") {
		t.Fatalf("sibling contract not capped:\n%s", web)
	}
	if !strings.Contains(web, "Contract errors: 409 dup") && !strings.Contains(web, "Contract payloads") {
		t.Fatalf("sibling contract missing:\n%s", web)
	}
	if strings.Contains(web, "api/**") {
		t.Fatalf("sibling implementation leaked:\n%s", web)
	}
	if !strings.HasSuffix(strings.TrimSpace(api), BriefClosing) || !strings.Contains(api, "check_drift") || !strings.Contains(api, "update_node_status") {
		t.Fatalf("closing line missing:\n%s", api)
	}
	// Section order.
	order := []string{"Project rules", "Interfaces you must honour", "Your node's contract", "Call the check_scope"}
	last := -1
	for _, s := range order {
		i := strings.Index(api, s)
		if i < last {
			t.Fatalf("%q out of order:\n%s", s, api)
		}
		last = i
	}
}

func TestBriefBuildOrderUnmetDependencyAndDependents(t *testing.T) {
	idx, _ := newModel(t)
	nmMake(t, idx, CreateInput{ID: "db", Name: "DB", Type: "database", Implementation: []string{"db/**"}})
	nmMake(t, idx, CreateInput{ID: "api", Name: "API", Type: "service", Implementation: []string{"api/**"}, DependsOn: []NodeID{"db"}})
	nmMake(t, idx, CreateInput{ID: "web", Name: "Web", Type: "frontend", Implementation: []string{"web/**"}, DependsOn: []NodeID{"api"}})

	api, err := idx.Brief("api")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Build order", `"DB" (db) (draft)`, "Not done yet", "stop and report back", `"Web" (web)`} {
		if !strings.Contains(api, want) {
			t.Fatalf("missing %q:\n%s", want, api)
		}
	}
	st := StatusDone
	if err := idx.Update("db", UpdateInput{Status: &st}); err != nil {
		t.Fatal(err)
	}
	api, _ = idx.Brief("api")
	if strings.Contains(api, "Not done yet") || strings.Contains(api, "stop and report back rather than guessing its interface") {
		t.Fatalf("unmet line should be gone:\n%s", api)
	}
	if !strings.Contains(api, `"DB" (db) (done)`) {
		t.Fatalf("dependency status missing:\n%s", api)
	}
	db, _ := idx.Brief("db")
	if strings.Contains(db, "Not done yet") || !strings.Contains(db, `"API" (api)`) {
		t.Fatalf("db brief:\n%s", db)
	}
}

func TestBriefOmitsNewSectionsWithoutNewData(t *testing.T) {
	idx, _ := newModel(t)
	nmMake(t, idx, CreateInput{ID: "a", Name: "A", Type: "service", Implementation: []string{"a/**"}})
	text, _ := idx.Brief("a")
	for _, s := range []string{"Project rules", "Interfaces you must honour", "contract", "Build order"} {
		if strings.Contains(text, s) {
			t.Fatalf("unexpected %q:\n%s", s, text)
		}
	}
}
