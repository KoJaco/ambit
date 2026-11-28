package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/KoJaco/ambit/internal/core"
)

func TestBuildOrderPlanFieldsCompactReadyOnlyAndGates(t *testing.T) {
	idx, _ := newModel(t)
	doJSON(t, idx, http.MethodPost, "/nodes", map[string]any{"id": "ext", "name": "Ext", "type": "external", "external": true,
		"human_gates": []map[string]string{{"kind": "credential", "note": "key"}}})
	doJSON(t, idx, http.MethodPost, "/nodes", map[string]any{"id": "a", "name": "A", "type": "service", "implementation": []string{"a/**"},
		"human_gates": []map[string]string{{"kind": "decision"}}, "deliverables": []string{"docs/a.md"}})
	doJSON(t, idx, http.MethodPost, "/nodes", map[string]any{"id": "b", "name": "B", "type": "service", "depends_on": []string{"a"}})

	var res core.BuildOrderResult
	raw := getJSON(t, idx, "/build-order")
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Externals) != 1 || res.Externals[0].ID != "ext" || len(res.Externals[0].HumanGates) != 1 {
		t.Fatalf("externals %s", raw)
	}
	if res.Order[0].Class != core.ClassNeedsHuman || res.Order[1].Class != core.ClassBlocked || len(res.ReadyNow) != 1 || res.MaxParallel != 0 {
		t.Fatalf("classes %s", raw)
	}

	rec := doJSON(t, idx, http.MethodPut, "/gates", map[string]any{"node_id": "a", "index": 0, "cleared": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("put gate %d %s", rec.Code, rec.Body.String())
	}
	raw = getJSON(t, idx, "/build-order")
	if !strings.Contains(raw, `"cleared":true`) || !strings.Contains(raw, `"class":"agent-parallel"`) {
		t.Fatalf("after clear %s", raw)
	}
	for body, want := range map[string]int{
		`{"node_id":"a","index":9,"cleared":true}`:  http.StatusBadRequest,
		`{"node_id":"zz","index":0,"cleared":true}`: http.StatusNotFound,
		`{"node_id":"a"}`: http.StatusBadRequest,
	} {
		var m map[string]any
		_ = json.Unmarshal([]byte(body), &m)
		if rec := doJSON(t, idx, http.MethodPut, "/gates", m); rec.Code != want {
			t.Fatalf("%s: %d %s", body, rec.Code, rec.Body.String())
		}
	}

	compact := getJSON(t, idx, "/build-order?compact=true")
	if strings.Contains(compact, `"name"`) || strings.Contains(compact, `"blocked_by"`) || !strings.Contains(compact, `"ready_now":["a"]`) || !strings.Contains(compact, `"externals":["ext"]`) {
		t.Fatalf("compact %s", compact)
	}
	ready := getJSON(t, idx, "/build-order?ready_only=1")
	var rr core.BuildOrderResult
	if err := json.Unmarshal([]byte(ready), &rr); err != nil || len(rr.Order) != 1 || rr.Order[0].ID != "a" {
		t.Fatalf("ready_only %s %v", ready, err)
	}
}

func TestNodeExtrasOverHTTP(t *testing.T) {
	idx, _ := newModel(t)
	rec := doJSON(t, idx, http.MethodPost, "/nodes", map[string]any{"id": "n", "name": "N", "type": "service",
		"deliverables": []string{"docs/x.md"}, "milestones": []map[string]string{{"id": "m1", "label": "one"}}, "shared": true})
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"deliverables":["docs/x.md"]`) || !strings.Contains(rec.Body.String(), `"shared":true`) {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, idx, http.MethodPatch, "/nodes/n", map[string]any{"deliverables": []string{}, "references": []string{"docs/r.md"}})
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"deliverables"`) || !strings.Contains(rec.Body.String(), `"references":["docs/r.md"]`) {
		t.Fatalf("patch %d %s", rec.Code, rec.Body.String())
	}
	if rec = doJSON(t, idx, http.MethodPatch, "/nodes/n", map[string]any{"deliverables": []string{"/abs"}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("abs path %d", rec.Code)
	}
	if rec = doJSON(t, idx, http.MethodPatch, "/nodes/n", map[string]any{"gates": []string{"ghost"}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("ghost gate %d", rec.Code)
	}
}
