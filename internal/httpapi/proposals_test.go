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

func TestGroupUngroupReparentHTTPRoundTrip(t *testing.T) {
	idx, _ := newModel(t)
	for _, c := range []core.CreateInput{
		{Name: "Platform", Type: "boundary"},
		{Name: "API", Type: "service", ParentID: "platform"},
		{Name: "Worker", Type: "service", ParentID: "platform"},
	} {
		if _, err := idx.Create(c); err != nil {
			t.Fatal(err)
		}
	}
	rec := doJSON(t, idx, http.MethodPost, "/proposals/group", map[string]any{
		"node_ids": []string{"api", "worker"}, "name": "Backend",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("group %d %s", rec.Code, rec.Body.String())
	}
	var diff core.ProposalDiff
	if err := json.Unmarshal(rec.Body.Bytes(), &diff); err != nil || len(diff.Operations) != 3 || diff.Source != "group_nodes" {
		t.Fatalf("diff %v %+v", err, diff)
	}
	if _, ok := idx.Nodes["backend"]; ok {
		t.Fatal("silent write")
	}
	if got := doJSON(t, idx, http.MethodGet, "/proposals", nil); !strings.Contains(got.Body.String(), diff.ID) {
		t.Fatalf("not listed: %s", got.Body.String())
	}
	if acc := doJSON(t, idx, http.MethodPost, "/proposals/"+diff.ID+"/accept", map[string]any{}); acc.Code != http.StatusOK {
		t.Fatalf("accept %d %s", acc.Code, acc.Body.String())
	}
	if idx.Nodes["api"].ParentID != "backend" {
		t.Fatalf("parent %q", idx.Nodes["api"].ParentID)
	}

	bad := doJSON(t, idx, http.MethodPost, "/proposals/group", map[string]any{"node_ids": []string{"api"}, "name": "X"})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("one node %d", bad.Code)
	}
	missing := doJSON(t, idx, http.MethodPost, "/proposals/group", map[string]any{"node_ids": []string{"api", "zz"}, "name": "X"})
	if missing.Code != http.StatusNotFound {
		t.Fatalf("unknown %d", missing.Code)
	}

	rec = doJSON(t, idx, http.MethodPost, "/proposals/reparent", map[string]any{"node_id": "api", "new_parent_id": "platform"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("reparent %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &diff); err != nil || len(diff.Operations) != 1 {
		t.Fatalf("%v %+v", err, diff)
	}
	doJSON(t, idx, http.MethodPost, "/proposals/"+diff.ID+"/reject", nil)

	rec = doJSON(t, idx, http.MethodPost, "/proposals/ungroup", map[string]any{"node_id": "backend", "delete_container": true})
	if rec.Code != http.StatusCreated {
		t.Fatalf("ungroup %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &diff); err != nil || len(diff.Operations) != 3 {
		t.Fatalf("%v %+v", err, diff)
	}
	if acc := doJSON(t, idx, http.MethodPost, "/proposals/"+diff.ID+"/accept", map[string]any{}); acc.Code != http.StatusOK {
		t.Fatalf("accept %d %s", acc.Code, acc.Body.String())
	}
	if _, ok := idx.Nodes["backend"]; ok || idx.Nodes["api"].ParentID != "platform" {
		t.Fatal("ungroup not applied")
	}
}

func TestProposalHTTPStateAndRestage(t *testing.T) {
	idx, _ := newModel(t)
	pid, err := idx.Stage(core.StageInput{Source: core.SourceSeedModel, Ops: []core.StageOp{
		{Op: core.OpCreateNode, Create: core.CreateInput{ID: "svc", Name: "Svc", Type: "service", Spec: "# s\n"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if st := doJSON(t, idx, http.MethodGet, "/proposals/"+pid+"/state", nil); st.Code != http.StatusOK || !strings.Contains(st.Body.String(), `"status":"pending"`) {
		t.Fatalf("state %d %s", st.Code, st.Body.String())
	}
	if res := doJSON(t, idx, http.MethodPost, "/proposals/"+pid+"/accept", nil); res.Code != http.StatusOK {
		t.Fatalf("accept %d %s", res.Code, res.Body.String())
	}
	if st := doJSON(t, idx, http.MethodGet, "/proposals/"+pid+"/state", nil); !strings.Contains(st.Body.String(), `"status":"accepted"`) {
		t.Fatalf("state %s", st.Body.String())
	}
	if rs := doJSON(t, idx, http.MethodPost, "/proposals/restage", map[string]any{"proposal_id": "p-20200101-0000-abcd"}); rs.Code != http.StatusNotFound {
		t.Fatalf("restage unknown %d %s", rs.Code, rs.Body.String())
	}
	if err := idx.Delete("svc"); err != nil {
		t.Fatal(err)
	}
	rs := doJSON(t, idx, http.MethodPost, "/proposals/restage", map[string]any{"proposal_id": pid, "operations": []int{0}})
	if rs.Code != http.StatusCreated || !strings.Contains(rs.Body.String(), `"proposal_id":"p-`) {
		t.Fatalf("restage %d %s", rs.Code, rs.Body.String())
	}
	var out map[string]string
	_ = json.Unmarshal(rs.Body.Bytes(), &out)
	if res := doJSON(t, idx, http.MethodPost, "/proposals/"+out["proposal_id"]+"/accept", nil); res.Code != http.StatusOK || idx.Nodes["svc"] == nil {
		t.Fatalf("accept restaged %d %s", res.Code, res.Body.String())
	}
}
