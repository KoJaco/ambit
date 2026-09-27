package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KoJaco/ambit/internal/core"
)

func TestLevelResponseOmitsTheRestOfTheGraph(t *testing.T) {
	idx, _ := newModel(t)
	platform, err := idx.Create(core.CreateInput{Name: "Platform", Type: "boundary", Spec: "platform spec"})
	if err != nil {
		t.Fatal(err)
	}
	orders, err := idx.Create(core.CreateInput{Name: "Orders Service", Type: "service", ParentID: platform})
	if err != nil {
		t.Fatal(err)
	}
	payments, err := idx.Create(core.CreateInput{Name: "Payments Service", Type: "service", ParentID: platform, Status: core.StatusDone})
	if err != nil {
		t.Fatal(err)
	}
	_, err = idx.Create(core.CreateInput{
		Name:           "Capture Worker",
		Type:           "worker",
		ParentID:       payments,
		Implementation: []string{"secret/capture-only/**"},
		Spec:           "OUTSIDE-LEVEL-capture",
	})
	if err != nil {
		t.Fatal(err)
	}
	billing, err := idx.Create(core.CreateInput{Name: "Billing Ledger", Type: "service", Spec: "OUTSIDE-LEVEL-billing"})
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.SetRelationship(orders, payments, "requests authorisation", core.KindSync); err != nil {
		t.Fatal(err)
	}
	if err := idx.SetRelationship(orders, billing, "settles", core.KindAsync); err != nil {
		t.Fatal(err)
	}

	body := getJSON(t, idx, "/levels/"+string(platform))
	if strings.Contains(body, "OUTSIDE-LEVEL") || strings.Contains(body, "secret/capture-only") || strings.Contains(body, "Billing Ledger") {
		t.Fatalf("response serialised nodes outside the level:\n%s", body)
	}
	var lvl levelJSON
	if err := json.Unmarshal([]byte(body), &lvl); err != nil {
		t.Fatal(err)
	}
	if lvl.Node == nil || lvl.Node.ID != string(platform) {
		t.Fatalf("node %#v", lvl.Node)
	}
	if len(lvl.Children) != 2 {
		t.Fatalf("children %#v", lvl.Children)
	}
	if len(lvl.Relationships) != 1 || lvl.Relationships[0].From != string(orders) || lvl.Relationships[0].To != string(payments) {
		t.Fatalf("relationships %#v", lvl.Relationships)
	}
	if len(lvl.Crossings) != 1 || lvl.Crossings[0].NodeID != string(orders) || lvl.Crossings[0].OtherID != string(billing) || lvl.Crossings[0].Direction != "out" {
		t.Fatalf("crossings %#v", lvl.Crossings)
	}

	rootBody := getJSON(t, idx, "/levels")
	for _, leaked := range []string{"Orders Service", "Payments Service", "Capture Worker", "OUTSIDE-LEVEL-capture", "secret/capture-only", "payments-service"} {
		if strings.Contains(rootBody, leaked) {
			t.Fatalf("root serialised %q from outside the level:\n%s", leaked, rootBody)
		}
	}
	var root levelJSON
	if err := json.Unmarshal([]byte(rootBody), &root); err != nil {
		t.Fatal(err)
	}
	if root.Node != nil {
		t.Fatalf("root node %#v", root.Node)
	}
	ids := map[string]bool{}
	for _, c := range root.Children {
		ids[c.ID] = true
	}
	if !ids[string(platform)] || !ids[string(billing)] || len(root.Children) != 2 {
		t.Fatalf("root children %#v", root.Children)
	}
	if len(root.Relationships) != 0 {
		t.Fatalf("root edges %#v", root.Relationships)
	}
}

func TestUnknownLevelIs404(t *testing.T) {
	idx, _ := newModel(t)
	rec := httptest.NewRecorder()
	Handler(idx).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/levels/missing-node", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "missing-node") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func getJSON(t *testing.T, idx *core.Index, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	Handler(idx).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("%s status %d body %s", path, rec.Code, rec.Body.String())
	}
	b, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func newModel(t *testing.T) (*core.Index, string) {
	t.Helper()
	dir := t.TempDir()
	if err := core.Init(dir); err != nil {
		t.Fatal(err)
	}
	idx, err := core.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return idx, dir
}
