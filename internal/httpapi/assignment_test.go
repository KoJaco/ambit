package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/KoJaco/ambit/internal/core"
)

func getBody(t *testing.T, idx *core.Index, path string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	Handler(idx).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: %d %s", path, rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestGetAssignmentNone(t *testing.T) {
	idx, _ := newModel(t)
	body := getBody(t, idx, "/assignment")
	if v, ok := body["node_id"]; !ok || v != nil {
		t.Fatalf("body %v", body)
	}
	if _, ok := body["dangling"]; ok {
		t.Fatalf("body %v", body)
	}
}

func TestAssignmentSetReadClear(t *testing.T) {
	idx, dir := newModel(t)
	a, err := idx.Create(core.CreateInput{Name: "Alpha", Type: "service"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := idx.Create(core.CreateInput{Name: "Beta", Type: "service"})
	if err != nil {
		t.Fatal(err)
	}
	before, err := idx.Node(a)
	if err != nil {
		t.Fatal(err)
	}
	nodeFile := filepath.Join(dir, ".arch", "nodes", string(a)+".json")
	rawBefore, err := os.ReadFile(nodeFile)
	if err != nil {
		t.Fatal(err)
	}

	if rec := doJSON(t, idx, http.MethodPut, "/assignment", map[string]string{"node_id": string(a)}); rec.Code != http.StatusNoContent {
		t.Fatalf("put %d %s", rec.Code, rec.Body.String())
	}
	body := getBody(t, idx, "/assignment")
	if body["node_id"] != string(a) || body["assigned_at"] == "" || body["assigned_at"] == nil {
		t.Fatalf("body %v", body)
	}
	if _, ok := body["dangling"]; ok {
		t.Fatalf("body %v", body)
	}

	// The flag is on the assigned node only, in both levels and node responses.
	lvl := getBody(t, idx, "/levels")
	flags := map[string]bool{}
	for _, c := range lvl["children"].([]any) {
		m := c.(map[string]any)
		flag, _ := m["assigned"].(bool)
		flags[m["id"].(string)] = flag
		if _, present := m["assigned"]; present != flag {
			t.Fatalf("assigned must be omitted when false: %v", m)
		}
	}
	if !flags[string(a)] || flags[string(b)] {
		t.Fatalf("flags %v", flags)
	}
	if got := getBody(t, idx, "/nodes/"+string(a)); got["assigned"] != true {
		t.Fatalf("node a %v", got)
	}
	if got := getBody(t, idx, "/nodes/"+string(b)); got["assigned"] != nil {
		t.Fatalf("node b %v", got)
	}

	// Assigning changes neither the node status nor the node file.
	after, err := idx.Node(a)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != before.Status {
		t.Fatalf("status %q -> %q", before.Status, after.Status)
	}
	rawAfter, err := os.ReadFile(nodeFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(rawAfter) != string(rawBefore) {
		t.Fatal("node file changed")
	}

	if rec := doJSON(t, idx, http.MethodDelete, "/assignment", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete %d", rec.Code)
	}
	if body := getBody(t, idx, "/assignment"); body["node_id"] != nil {
		t.Fatalf("body %v", body)
	}
	if got := getBody(t, idx, "/nodes/"+string(a)); got["assigned"] != nil {
		t.Fatalf("node a %v", got)
	}
}

func TestGetAssignmentDangling(t *testing.T) {
	idx, dir := newModel(t)
	if _, err := idx.Create(core.CreateInput{Name: "Alpha", Type: "service"}); err != nil {
		t.Fatal(err)
	}
	local := `{"assignment":{"node_id":"ghost","assigned_at":"2026-01-02T03:04:05Z"}}`
	if err := os.WriteFile(filepath.Join(dir, ".arch", "local.json"), []byte(local), 0o644); err != nil {
		t.Fatal(err)
	}
	reopened, err := core.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	body := getBody(t, reopened, "/assignment")
	if body["node_id"] != "ghost" || body["dangling"] != true || body["assigned_at"] != "2026-01-02T03:04:05Z" {
		t.Fatalf("body %v", body)
	}
}
