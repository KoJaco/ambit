package core

import (
	"strings"
	"testing"
)

func TestContractChangeFlagsDoneNodes(t *testing.T) {
	idx, dir := newModel(t)
	api := mk(t, idx, CreateInput{Name: "Api", Type: "service", Contract: &NodeContract{Payloads: "POST /a"}})
	web := mk(t, idx, CreateInput{Name: "Web", Type: "frontend"})
	if _, err := idx.SetRelationship(SetRelationshipInput{From: web, To: api, Label: "calls", Kind: KindSync, Contract: &EdgeContract{Text: "v1"}}); err != nil {
		t.Fatal(err)
	}
	st := StatusDone
	for _, id := range []NodeID{api, web} {
		if err := idx.Update(id, UpdateInput{Status: &st}); err != nil {
			t.Fatal(err)
		}
	}
	idx = reopen(t, dir)
	if idx.Nodes[api].ContractHash == "" {
		t.Fatal("baseline not persisted")
	}
	if changed(t, idx) != "" {
		t.Fatalf("nothing changed yet: %q", changed(t, idx))
	}

	if err := idx.Update(api, UpdateInput{Contract: &NodeContract{Payloads: "POST /a v2"}}); err != nil {
		t.Fatal(err)
	}
	if got := changed(t, idx); got != string(api) {
		t.Fatalf("own contract change: %q", got)
	}
	brief, err := idx.Brief(api)
	if err != nil || !strings.Contains(brief, "contract this node must honour changed") {
		t.Fatalf("brief lacks warning: %v\n%s", err, brief)
	}
	bo, err := BuildOrder(idx, "")
	if err != nil {
		t.Fatal(err)
	}
	flagged := 0
	for _, it := range bo.Order {
		if it.ContractChanged {
			flagged++
		}
	}
	if flagged != 1 {
		t.Fatalf("build order flagged %d", flagged)
	}

	// Setting the status again accepts the new contract.
	if err := idx.Update(api, UpdateInput{Status: &st}); err != nil {
		t.Fatal(err)
	}
	if changed(t, idx) != "" {
		t.Fatalf("still flagged after re-marking: %q", changed(t, idx))
	}
}

func changed(t *testing.T, idx *Index) string {
	t.Helper()
	if _, err := idx.SetMethod("design-first"); err != nil {
		t.Fatal(err)
	}
	res := findingsFor(t, idx, CheckModelOptions{})
	var ids []string
	for _, f := range onlyRule(res, RuleContractChanged) {
		ids = append(ids, string(f.NodeID))
	}
	return strings.Join(ids, ",")
}

func TestContractChangeFromRelationshipFlagsBothEndpoints(t *testing.T) {
	idx, _ := newModel(t)
	api := mk(t, idx, CreateInput{Name: "Api", Type: "service"})
	web := mk(t, idx, CreateInput{Name: "Web", Type: "frontend"})
	rel, err := idx.SetRelationship(SetRelationshipInput{From: web, To: api, Label: "calls", Kind: KindSync, Contract: &EdgeContract{Text: "v1"}})
	if err != nil {
		t.Fatal(err)
	}
	done, draft := StatusDone, StatusDraft
	for _, id := range []NodeID{api, web} {
		if err := idx.Update(id, UpdateInput{Status: &done}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := idx.SetRelationship(SetRelationshipInput{ID: rel.ID, From: web, To: api, Label: "calls", Kind: KindSync, Contract: &EdgeContract{Text: "v2"}}); err != nil {
		t.Fatal(err)
	}
	if got := changed(t, idx); got != "api,web" && got != "web,api" {
		t.Fatalf("both endpoints should be flagged, got %q", got)
	}
	// Moving a node back to draft stops flagging it; re-marking in_progress rebaselines.
	if err := idx.Update(web, UpdateInput{Status: &draft}); err != nil {
		t.Fatal(err)
	}
	if got := changed(t, idx); got != "api" {
		t.Fatalf("draft node still flagged: %q", got)
	}
	ip := StatusInProgress
	if err := idx.Update(web, UpdateInput{Status: &ip}); err != nil {
		t.Fatal(err)
	}
	if got := changed(t, idx); got != "api" {
		t.Fatalf("after rebaseline: %q", got)
	}
}
