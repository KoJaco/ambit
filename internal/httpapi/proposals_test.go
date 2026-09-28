package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/KoJaco/ambit/internal/core"
)

func TestProposalHTTPStaleConfirmAndUnreadableDelete(t *testing.T) {
	idx, _ := newModel(t)
	id, err := idx.Create(core.CreateInput{Name: "Payments", Type: "service", Spec: "before"})
	if err != nil {
		t.Fatal(err)
	}
	spec := "staged"
	pid, err := idx.Stage(core.StageInput{
		Source: core.OpUpdateNode,
		Ops: []core.StageOp{{
			Op:     core.OpUpdateNode,
			NodeID: id,
			Update: core.UpdateInput{Spec: &spec},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	list := doJSON(t, idx, http.MethodGet, "/proposals", nil)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), pid) || strings.Contains(list.Body.String(), `"stale":true`) {
		t.Fatalf("list %d %s", list.Code, list.Body.String())
	}
	diff := doJSON(t, idx, http.MethodGet, "/proposals/"+pid, nil)
	if diff.Code != http.StatusOK || !strings.Contains(diff.Body.String(), `"fields":["spec"]`) {
		t.Fatalf("diff %d %s", diff.Code, diff.Body.String())
	}

	edited := "architect"
	if err := idx.Update(id, core.UpdateInput{Spec: &edited}); err != nil {
		t.Fatal(err)
	}
	stale := doJSON(t, idx, http.MethodPost, "/proposals/"+pid+"/operations/0/accept", map[string]any{})
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), "confirm_stale") || !strings.Contains(stale.Body.String(), string(id)) {
		t.Fatalf("stale %d %s", stale.Code, stale.Body.String())
	}
	if got := idx.Nodes[id].Spec; got != edited {
		t.Fatalf("wrote over architect edit: %q", got)
	}
	ok := doJSON(t, idx, http.MethodPost, "/proposals/"+pid+"/operations/0/accept", map[string]any{"confirm_stale": true})
	if ok.Code != http.StatusNoContent {
		t.Fatalf("confirm %d %s", ok.Code, ok.Body.String())
	}
	if got := idx.Nodes[id].Spec; got != spec {
		t.Fatalf("spec %q", got)
	}

	bad := doJSON(t, idx, http.MethodPost, "/proposals/"+pid+"/operations/nope/accept", map[string]any{})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("index %d %s", bad.Code, bad.Body.String())
	}
	missing := doJSON(t, idx, http.MethodGet, "/proposals/p-20260928-0000-abcd", nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing %d %s", missing.Code, missing.Body.String())
	}
}

func TestUnreadableProposalCanBeDeleted(t *testing.T) {
	idx, dir := newModel(t)
	pid, err := idx.Stage(core.StageInput{
		Source: core.OpCreateNode,
		Ops:    []core.StageOp{{Op: core.OpCreateNode, Create: core.CreateInput{Name: "Orders", Type: "service", Spec: "o"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	path := dir + "/.arch/.proposals/" + pid + "/manifest.json"
	obj := readObj(t, path)
	obj["manifest_version"] = 2
	writeObj(t, path, obj)

	list := doJSON(t, idx, http.MethodGet, "/proposals", nil)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"unreadable":true`) {
		t.Fatalf("list %s", list.Body.String())
	}
	diff := doJSON(t, idx, http.MethodGet, "/proposals/"+pid, nil)
	if diff.Code != http.StatusConflict {
		t.Fatalf("diff %d %s", diff.Code, diff.Body.String())
	}
	accept := doJSON(t, idx, http.MethodPost, "/proposals/"+pid+"/accept", nil)
	if accept.Code != http.StatusConflict {
		t.Fatalf("accept-all %d %s", accept.Code, accept.Body.String())
	}
	del := doJSON(t, idx, http.MethodDelete, "/proposals/"+pid, nil)
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete %d %s", del.Code, del.Body.String())
	}
	var body struct {
		Proposals []json.RawMessage `json:"proposals"`
	}
	list = doJSON(t, idx, http.MethodGet, "/proposals", nil)
	if err := json.Unmarshal(list.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Proposals) != 0 {
		t.Fatalf("still listed %s", list.Body.String())
	}
}

func TestAcceptAllReportsSkips(t *testing.T) {
	idx, _ := newModel(t)
	payments, err := idx.Create(core.CreateInput{Name: "Payments", Type: "service", Spec: "pay"})
	if err != nil {
		t.Fatal(err)
	}
	spec := "staged"
	pid, err := idx.Stage(core.StageInput{
		Source:  core.SourceSeedModel,
		Summary: "orders and a payments edit",
		Ops: []core.StageOp{
			{Op: core.OpCreateNode, Create: core.CreateInput{Name: "Orders", Type: "service", Spec: "o"}},
			{Op: core.OpUpdateNode, NodeID: payments, Update: core.UpdateInput{Spec: &spec}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	edited := "newer"
	if err := idx.Update(payments, core.UpdateInput{Spec: &edited}); err != nil {
		t.Fatal(err)
	}
	rec := doJSON(t, idx, http.MethodPost, "/proposals/"+pid+"/accept", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("accept-all %d %s", rec.Code, rec.Body.String())
	}
	var result core.AcceptAllResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Applied) != 1 || len(result.Skipped) != 1 || result.Skipped[0].NodeID != string(payments) {
		t.Fatalf("result %+v", result)
	}
	if idx.Nodes[payments].Spec != edited {
		t.Fatalf("spec %q", idx.Nodes[payments].Spec)
	}
	if _, ok := idx.Nodes["orders"]; !ok {
		t.Fatal("orders was not created")
	}
}
