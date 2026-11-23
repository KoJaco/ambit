package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/KoJaco/ambit/internal/core"
)

func TestNodeContractAndDependsOnCreatePatchRead(t *testing.T) {
	idx, _ := newModel(t)
	rec := doJSON(t, idx, http.MethodPost, "/nodes", map[string]any{"id": "db", "name": "DB", "type": "database"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create db %d %s", rec.Code, rec.Body.String())
	}
	var plain map[string]json.RawMessage
	_ = json.Unmarshal(rec.Body.Bytes(), &plain)
	if string(plain["depends_on"]) != "[]" {
		t.Fatalf("depends_on should be [] on a node without any: %s", rec.Body.String())
	}
	if _, ok := plain["contract"]; ok {
		t.Fatalf("contract should be omitted when empty: %s", rec.Body.String())
	}

	rec = doJSON(t, idx, http.MethodPost, "/nodes", map[string]any{
		"id": "api", "name": "API", "type": "service", "depends_on": []string{"db"},
		"contract": map[string]any{"payloads": "POST /x", "invariants": []string{"scoped"}, "errors": []string{"409"}, "owns_data": []string{"orders"}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create api %d %s", rec.Code, rec.Body.String())
	}
	var n nodeJSON
	_ = json.Unmarshal(rec.Body.Bytes(), &n)
	if len(n.DependsOn) != 1 || n.DependsOn[0] != "db" || n.Contract == nil || n.Contract.Payloads != "POST /x" || n.Contract.OwnsData[0] != "orders" {
		t.Fatalf("created %+v", n)
	}

	// patch with neither field leaves them alone
	rec = doJSON(t, idx, http.MethodPatch, "/nodes/api", map[string]any{"name": "API 2"})
	_ = json.Unmarshal(rec.Body.Bytes(), &n)
	if rec.Code != http.StatusOK || len(n.DependsOn) != 1 || n.Contract == nil {
		t.Fatalf("patch %d %s", rec.Code, rec.Body.String())
	}
	// get node and level reads carry both
	var got nodeJSON
	if err := json.Unmarshal([]byte(getJSON(t, idx, "/nodes/api")), &got); err != nil || got.Contract == nil || len(got.DependsOn) != 1 {
		t.Fatalf("get node %+v err %v", got, err)
	}
	var lvl levelJSON
	if err := json.Unmarshal([]byte(getJSON(t, idx, "/levels")), &lvl); err != nil {
		t.Fatal(err)
	}
	var seen bool
	for _, c := range lvl.Children {
		if c.ID == "api" && c.Contract != nil && len(c.DependsOn) == 1 {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("level lacks contract/depends_on: %+v", lvl.Children)
	}

	// empty values clear
	rec = doJSON(t, idx, http.MethodPatch, "/nodes/api", map[string]any{"depends_on": []string{}, "contract": map[string]any{}})
	n = nodeJSON{}
	_ = json.Unmarshal(rec.Body.Bytes(), &n)
	if rec.Code != http.StatusOK || len(n.DependsOn) != 0 || n.Contract != nil {
		t.Fatalf("clear %d %s", rec.Code, rec.Body.String())
	}
}

func TestDependsOnErrorMappings(t *testing.T) {
	idx, _ := newModel(t)
	doJSON(t, idx, http.MethodPost, "/nodes", map[string]any{"id": "a", "name": "A", "type": "service"})
	doJSON(t, idx, http.MethodPost, "/nodes", map[string]any{"id": "b", "name": "B", "type": "service", "depends_on": []string{"a"}})

	// cycle -> 409
	rec := doJSON(t, idx, http.MethodPatch, "/nodes/a", map[string]any{"depends_on": []string{"b"}})
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "cycle") {
		t.Fatalf("cycle %d %s", rec.Code, rec.Body.String())
	}
	// has dependents -> 409
	rec = doJSON(t, idx, http.MethodDelete, "/nodes/a", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("delete with dependents %d %s", rec.Code, rec.Body.String())
	}
	// bad dependency (unknown id) -> 400
	rec = doJSON(t, idx, http.MethodPatch, "/nodes/a", map[string]any{"depends_on": []string{"missing"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad dependency %d %s", rec.Code, rec.Body.String())
	}
	// bad contract -> 400
	rec = doJSON(t, idx, http.MethodPatch, "/nodes/a", map[string]any{"contract": map[string]any{"payloads": strings.Repeat("x", core.MaxContractPayloads+1)}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad contract %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, idx, http.MethodPost, "/nodes", map[string]any{"name": "C", "type": "service", "depends_on": []string{"nope"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create bad dependency %d %s", rec.Code, rec.Body.String())
	}
}

func TestRelationshipContractWriteAndRead(t *testing.T) {
	idx, _ := newModel(t)
	doJSON(t, idx, http.MethodPost, "/nodes", map[string]any{"id": "a", "name": "A", "type": "service"})
	doJSON(t, idx, http.MethodPost, "/nodes", map[string]any{"id": "b", "name": "B", "type": "service"})
	rec := doJSON(t, idx, http.MethodPut, "/relationships", map[string]any{
		"from": "a", "to": "b", "label": "calls", "kind": "sync",
		"contract": map[string]any{"format": "openapi", "ref": "api/openapi.yaml", "text": "see ref"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("put %d %s", rec.Code, rec.Body.String())
	}
	var rel relJSON
	_ = json.Unmarshal(rec.Body.Bytes(), &rel)
	if rel.Contract == nil || rel.Contract.Format != "openapi" || rel.ID == "" {
		t.Fatalf("rel %+v", rel)
	}
	var lvl levelJSON
	if err := json.Unmarshal([]byte(getJSON(t, idx, "/levels")), &lvl); err != nil || len(lvl.Relationships) != 1 || lvl.Relationships[0].Contract == nil || lvl.Relationships[0].Contract.Ref != "api/openapi.yaml" {
		t.Fatalf("level rels %+v err %v", lvl.Relationships, err)
	}
	// omitted contract keeps it; {} clears it
	rec = doJSON(t, idx, http.MethodPut, "/relationships", map[string]any{"id": rel.ID, "from": "a", "to": "b", "label": "calls2", "kind": "sync"})
	rel = relJSON{}
	_ = json.Unmarshal(rec.Body.Bytes(), &rel)
	if rec.Code != http.StatusOK || rel.Contract == nil {
		t.Fatalf("omitted %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, idx, http.MethodPut, "/relationships", map[string]any{"id": rel.ID, "from": "a", "to": "b", "label": "calls2", "kind": "sync", "contract": map[string]any{}})
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"contract"`) {
		t.Fatalf("clear %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, idx, http.MethodPut, "/relationships", map[string]any{"id": rel.ID, "from": "a", "to": "b", "contract": map[string]any{"format": "weird"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad format %d %s", rec.Code, rec.Body.String())
	}
}

func TestConstraintsGetPut(t *testing.T) {
	idx, _ := newModel(t)
	if out := getJSON(t, idx, "/constraints"); strings.TrimSpace(out) != `{"constraints":[]}` {
		t.Fatalf("empty %s", out)
	}
	rec := doJSON(t, idx, http.MethodPut, "/constraints", map[string]any{"constraints": []map[string]string{
		{"id": "scope-by-user", "text": "Every query scopes by user_id"},
		{"id": "no-secrets", "text": "No secrets in logs", "severity": "should"},
	}})
	if rec.Code != http.StatusOK {
		t.Fatalf("put %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Constraints []core.Constraint `json:"constraints"`
	}
	if err := json.Unmarshal([]byte(getJSON(t, idx, "/constraints")), &body); err != nil || len(body.Constraints) != 2 ||
		body.Constraints[0].Severity != "must" || body.Constraints[1].Severity != "should" {
		t.Fatalf("get %+v err %v", body, err)
	}
	for name, in := range map[string][]map[string]string{
		"bad slug":   {{"id": "Bad ID", "text": "x"}},
		"empty text": {{"id": "a", "text": ""}},
		"bad sev":    {{"id": "a", "text": "x", "severity": "nope"}},
		"duplicate":  {{"id": "a", "text": "x"}, {"id": "a", "text": "y"}},
	} {
		rec = doJSON(t, idx, http.MethodPut, "/constraints", map[string]any{"constraints": in})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	if len(idx.ConstraintList()) != 2 {
		t.Fatal("invalid put changed constraints")
	}
	rec = doJSON(t, idx, http.MethodPut, "/constraints", map[string]any{"constraints": []any{}})
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"constraints":[]}` {
		t.Fatalf("clear %d %s", rec.Code, rec.Body.String())
	}
	if rec = doJSON(t, idx, http.MethodPut, "/constraints", "nope"); rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed %d", rec.Code)
	}
}

func TestBuildOrderEndpoint(t *testing.T) {
	idx, _ := newModel(t)
	doJSON(t, idx, http.MethodPost, "/nodes", map[string]any{"id": "a", "name": "A", "type": "service"})
	doJSON(t, idx, http.MethodPost, "/nodes", map[string]any{"id": "b", "name": "B", "type": "service", "depends_on": []string{"a"}})
	var res core.BuildOrderResult
	raw := getJSON(t, idx, "/build-order")
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Order) != 2 || res.Order[0].ID != "a" || res.Order[1].Wave != 2 || len(res.Order[1].BlockedBy) != 1 || len(res.Waves) != 2 {
		t.Fatalf("order %s", raw)
	}
	for _, k := range []string{`"cycles":[]`, `"unordered":[]`, `"blocked_by"`, `"depends_on"`, `"done"`} {
		if !strings.Contains(raw, k) {
			t.Fatalf("missing %s in %s", k, raw)
		}
	}
	raw = getJSON(t, idx, "/build-order?scope=b")
	if err := json.Unmarshal([]byte(raw), &res); err != nil || len(res.Order) != 1 || res.Scope != "b" {
		t.Fatalf("scoped %s err %v", raw, err)
	}
	rec := doJSON(t, idx, http.MethodGet, "/build-order?scope=nope", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown scope %d", rec.Code)
	}
}
