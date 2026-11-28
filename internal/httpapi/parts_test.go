package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/KoJaco/ambit/internal/core"
)

func TestPartsCRUD(t *testing.T) {
	idx, _ := newModel(t)
	var got core.ScopeInfo
	rec := doJSON(t, idx, http.MethodGet, "/parts", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.Scope != "whole" || got.Parts == nil || len(got.Parts) != 0 {
		t.Fatalf("%+v %v %s", got, err, rec.Body.String())
	}
	rec = doJSON(t, idx, http.MethodPut, "/parts", map[string]any{
		"scope": "partial",
		"part":  map[string]any{"id": "billing", "name": "Billing", "include": []string{"internal/billing/**"}, "root_node_ids": []string{}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Scope != "partial" || len(got.Parts) != 1 || got.Parts[0].ID != "billing" {
		t.Fatalf("%+v", got)
	}
	if idx.Scope != core.ScopePartial {
		t.Fatal("index not updated")
	}
	for _, bad := range []map[string]any{
		{},
		{"scope": "sideways"},
		{"part": map[string]any{"id": "Bad Id"}},
		{"part": map[string]any{"id": "x", "include": []string{"[bad"}}},
	} {
		if rec := doJSON(t, idx, http.MethodPut, "/parts", bad); rec.Code != http.StatusBadRequest {
			t.Fatalf("%v: %d %s", bad, rec.Code, rec.Body.String())
		}
	}
	if rec := doJSON(t, idx, http.MethodDelete, "/parts/nope", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("delete missing %d", rec.Code)
	}
	rec = doJSON(t, idx, http.MethodDelete, "/parts/billing", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"parts":[]`) {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestPolicyEndpointIsHumanSurfaceAndGatedByStage(t *testing.T) {
	idx, _ := newModel(t)
	rec := doJSON(t, idx, http.MethodGet, "/policy/proposals", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"auto_accept":false`) {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, idx, http.MethodPut, "/policy/proposals", map[string]any{}); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing field %d", rec.Code)
	}
	rec = doJSON(t, idx, http.MethodPut, "/policy/proposals", map[string]any{"auto_accept": true})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"auto_accept":true`) || !strings.Contains(rec.Body.String(), `"effective":false`) {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if !idx.ProposalPolicy.AutoAccept {
		t.Fatal("index not updated")
	}
	if err := idx.SetConfigKeys(map[string]any{"method": "design-first", "stage": "build"}); err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, idx, http.MethodGet, "/policy/proposals", nil)
	if !strings.Contains(rec.Body.String(), `"effective":true`) {
		t.Fatalf("%s", rec.Body.String())
	}
}

func TestAuditEndpointsAndHumanAcceptIsLogged(t *testing.T) {
	idx, _ := newModel(t)
	id, err := idx.Create(core.CreateInput{Name: "Payments", Type: "service", Spec: "a"})
	if err != nil {
		t.Fatal(err)
	}
	spec := "b"
	pid, err := idx.Stage(core.StageInput{Source: core.OpUpdateNode, Origin: "mcp-x", Ops: []core.StageOp{{Op: core.OpUpdateNode, NodeID: id, Update: core.UpdateInput{Spec: &spec}}}})
	if err != nil {
		t.Fatal(err)
	}
	if rec := doJSON(t, idx, http.MethodPost, "/proposals/"+pid+"/accept", nil); rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, idx, http.MethodGet, "/proposals/audit", nil)
	var list struct {
		Entries []core.AuditEntry `json:"entries"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &list) != nil || len(list.Entries) != 1 || list.Entries[0].Actor != core.ActorHuman {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "mcp-x") {
		t.Fatal("raw origin leaked")
	}
	// The digest defaults to the architect agent, so a human accept is absent from it.
	rec = doJSON(t, idx, http.MethodGet, "/proposals/audit/digest", nil)
	var d core.AuditDigest
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &d) != nil || d.Proposals != 0 || d.ByRule == nil || d.Entries == nil {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, idx, http.MethodGet, "/proposals/audit/digest?actor=human", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil || d.Proposals != 1 || d.ByRule[0].Rule != "human" {
		t.Fatalf("%s", rec.Body.String())
	}
	if rec := doJSON(t, idx, http.MethodGet, "/proposals/audit?since=nonsense", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("since %d", rec.Code)
	}
	// A real proposal id still routes to the proposal handler.
	if rec := doJSON(t, idx, http.MethodGet, "/proposals/p-20260928-0000-abcd", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("%d", rec.Code)
	}
}
