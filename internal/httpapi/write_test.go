package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KoJaco/ambit/internal/core"
)

func TestCycleConflictComesFromCore(t *testing.T) {
	idx, _ := newModel(t)
	platform, err := idx.Create(core.CreateInput{Name: "Platform", Type: "boundary"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := idx.Create(core.CreateInput{Name: "Child", Type: "service", ParentID: platform})
	if err != nil {
		t.Fatal(err)
	}
	parent := child
	coreErr := idx.Update(platform, core.UpdateInput{Parent: &parent})
	if !errors.Is(coreErr, core.ErrCycle) {
		t.Fatalf("core error %v", coreErr)
	}

	rec := doJSON(t, idx, http.MethodPatch, "/nodes/"+string(platform), map[string]string{
		"parent_id": string(child),
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error != coreErr.Error() {
		t.Fatalf("http %q core %q", body.Error, coreErr.Error())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(string(platform))) || !bytes.Contains(rec.Body.Bytes(), []byte(string(child))) {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestMutationsWriteThroughCore(t *testing.T) {
	idx, dir := newModel(t)
	rec := doJSON(t, idx, http.MethodPost, "/nodes", map[string]any{
		"name":     "Payments Service",
		"type":     "service",
		"markdown": "# Payments\n",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created nodeJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID != "payments-service" || created.Status != "draft" || created.Markdown != "# Payments\n" {
		t.Fatalf("created %#v", created)
	}

	status := "assigned"
	rec = doJSON(t, idx, http.MethodPatch, "/nodes/"+created.ID, map[string]any{"status": status})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch %d %s", rec.Code, rec.Body.String())
	}
	reopened, err := core.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Nodes[core.NodeID(created.ID)].Status != core.StatusAssigned {
		t.Fatalf("status %s", reopened.Nodes[core.NodeID(created.ID)].Status)
	}

	other := doJSON(t, idx, http.MethodPost, "/nodes", map[string]any{"name": "Orders", "type": "service"})
	if other.Code != http.StatusCreated {
		t.Fatalf("orders %d %s", other.Code, other.Body.String())
	}
	var orders nodeJSON
	if err := json.Unmarshal(other.Body.Bytes(), &orders); err != nil {
		t.Fatal(err)
	}
	rel := doJSON(t, idx, http.MethodPut, "/relationships", map[string]string{
		"from": created.ID, "to": orders.ID, "label": "calls", "kind": "sync",
	})
	if rel.Code != http.StatusOK {
		t.Fatalf("rel %d %s", rel.Code, rel.Body.String())
	}
	reopened, err = core.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.Relationships) != 1 || reopened.Relationships[0].Label != "calls" {
		t.Fatalf("relationships %#v", reopened.Relationships)
	}

	del := httptest.NewRecorder()
	Handler(idx).ServeHTTP(del, httptest.NewRequest(http.MethodDelete, "/nodes/"+orders.ID, nil))
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete %d %s", del.Code, del.Body.String())
	}
	reopened, err = core.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reopened.Nodes[core.NodeID(orders.ID)]; ok {
		t.Fatal("deleted node still present")
	}
}

func TestMalformedJSONIs400(t *testing.T) {
	idx, _ := newModel(t)
	req := httptest.NewRequest(http.MethodPost, "/nodes", bytes.NewBufferString("{"))
	rec := httptest.NewRecorder()
	Handler(idx).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
}

func doJSON(t *testing.T, idx *core.Index, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	Handler(idx).ServeHTTP(rec, req)
	return rec
}
