package httpapi

import (
	"context"
	"fmt"
	"net/http/httptest"
	"time"

	"encoding/json"
	"github.com/KoJaco/ambit/internal/core"
	"net/http"
	"testing"
)

func decodeMap(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("%v: %s", err, s)
	}
	return m
}

func TestMethodDefaultsToFreeform(t *testing.T) {
	idx, _ := newModel(t)
	m := decodeMap(t, getJSON(t, idx, "/method"))
	if m["id"] != "freeform" || m["source"] != "builtin" || m["stage"] != nil {
		t.Fatalf("%v", m)
	}
	if rules := m["rules"].([]any); len(rules) != 11 {
		t.Fatalf("rules %v", rules)
	}
	if _, ok := m["gates"].(map[string]any); !ok {
		t.Fatalf("gates %v", m["gates"])
	}
	list := decodeMap(t, getJSON(t, idx, "/methods"))["methods"].([]any)
	if len(list) != 3 || list[0].(map[string]any)["active"] != true {
		t.Fatalf("%v", list)
	}
	st := decodeMap(t, getJSON(t, idx, "/stage"))
	if st["stage"] != nil || len(st["stages"].([]any)) != 0 {
		t.Fatalf("%v", st)
	}
}

func TestPutMethodAndStage(t *testing.T) {
	idx, _ := newModel(t)
	rec := doJSON(t, idx, http.MethodPut, "/method", map[string]string{"id": "design-first"})
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	res := decodeMap(t, rec.Body.String())
	if res["method"] != "design-first" || res["stage"] != "design" {
		t.Fatalf("%v", res)
	}
	m := decodeMap(t, getJSON(t, idx, "/method"))
	if m["id"] != "design-first" || m["stage"].(map[string]any)["id"] != "design" {
		t.Fatalf("%v", m)
	}
	if g := m["gates"].(map[string]any)["design"].([]any); len(g) != 3 {
		t.Fatalf("gates %v", g)
	}
	rec = doJSON(t, idx, http.MethodPut, "/stage", map[string]string{"id": "build"})
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	st := decodeMap(t, getJSON(t, idx, "/stage"))
	if st["stage"].(map[string]any)["id"] != "build" || st["method"].(map[string]any)["id"] != "design-first" {
		t.Fatalf("%v", st)
	}
	// Switching to a stage-less method resets the stage.
	rec = doJSON(t, idx, http.MethodPut, "/method", map[string]string{"id": "map-existing"})
	res = decodeMap(t, rec.Body.String())
	if res["stage_reset"] != true || res["previous_stage"] != "build" {
		t.Fatalf("%v", res)
	}
}

func TestMethodErrorCodes(t *testing.T) {
	idx, _ := newModel(t)
	cases := []struct {
		method, path string
		body         any
		status       int
		code         string
	}{
		{http.MethodPut, "/method", map[string]string{"id": "nope"}, 400, "unknown_method"},
		{http.MethodPut, "/stage", map[string]string{"id": "build"}, 409, "stage_unsupported"},
	}
	for _, c := range cases {
		rec := doJSON(t, idx, c.method, c.path, c.body)
		m := decodeMap(t, rec.Body.String())
		if rec.Code != c.status || m["code"] != c.code || m["error"] == "" {
			t.Fatalf("%s %s: %d %v", c.method, c.path, rec.Code, m)
		}
	}
	doJSON(t, idx, http.MethodPut, "/method", map[string]string{"id": "design-first"})
	rec := doJSON(t, idx, http.MethodPut, "/stage", map[string]string{"id": "ship"})
	if m := decodeMap(t, rec.Body.String()); rec.Code != 400 || m["code"] != "unknown_stage" {
		t.Fatalf("%d %v", rec.Code, m)
	}
	// Existing errors carry no code.
	rec = doJSON(t, idx, http.MethodPatch, "/nodes/missing", map[string]string{"name": "x"})
	if m := decodeMap(t, rec.Body.String()); rec.Code != 404 || m["code"] != nil {
		t.Fatalf("%d %v", rec.Code, m)
	}
}

func TestMethodErrorCodeMapping(t *testing.T) {
	for _, c := range []struct {
		err    error
		code   string
		status int
	}{
		{wrapErr(coreErr("stage")), "stage_gate", 409},
		{wrapErr(coreErr("bad")), "bad_method", 409},
	} {
		code, status := methodErrorCode(c.err)
		if code != c.code || status != c.status {
			t.Fatalf("%v: %s %d", c.err, code, status)
		}
	}
}

func TestCheckModelEndpoint(t *testing.T) {
	idx, _ := newModel(t)
	doJSON(t, idx, http.MethodPut, "/method", map[string]string{"id": "design-first"})
	res := decodeMap(t, getJSON(t, idx, "/check-model"))
	if res["method"] != "design-first" || res["ok"] != true || res["findings"] == nil || res["counts"] == nil {
		t.Fatalf("%v", res)
	}
	if _, ok := res["drift"]; ok {
		t.Fatalf("drift present: %v", res)
	}
	res = decodeMap(t, getJSON(t, idx, "/check-model?include_drift=1&all=1"))
	if res["drift"] == nil && res["drift_error"] == nil {
		t.Fatalf("expected drift or drift_error: %v", res)
	}
}

func coreErr(kind string) error {
	switch kind {
	case "stage":
		return core.ErrStageGate
	case "bad":
		return core.ErrBadMethod
	}
	return core.ErrBadLintRule
}

func wrapErr(err error) error { return fmt.Errorf("ctx: %w", err) }

// N5: a CLI-style change from a second Index reaches a running server (watcher + SSE).
func TestCrossProcessMethodChangeReachesServer(t *testing.T) {
	idx, dir := newModel(t)
	if _, err := idx.SetMethod("design-first"); err != nil {
		t.Fatal(err)
	}
	if err := idx.SetStage("build"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := newHub(idx)
	if err := h.start(ctx); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(routes(idx, h))
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	other, err := core.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.SetMethod("freeform"); err != nil {
		t.Fatal(err)
	}
	events := collectEvents(t, resp, 4*time.Second, func(name, data string) bool { return name == "model-changed" })
	if len(events) == 0 {
		t.Fatal("no model-changed event after cross-process method change")
	}
	if m := decodeMap(t, getJSON(t, idx, "/method")); m["id"] != "freeform" || m["stage"] != nil {
		t.Fatalf("server still reports %v", m)
	}
	if res := decodeMap(t, getJSON(t, idx, "/check-model")); res["method"] != "freeform" {
		t.Fatalf("%v", res)
	}
}

func TestCheckModelExplicitDriftFalseHonoured(t *testing.T) {
	idx, _ := newModel(t)
	doJSON(t, idx, http.MethodPut, "/method", map[string]string{"id": "map-existing"})
	if res := decodeMap(t, getJSON(t, idx, "/check-model")); res["drift"] == nil && res["drift_error"] == nil {
		t.Fatalf("method default should run drift: %v", res)
	}
	for _, q := range []string{"0", "false"} {
		res := decodeMap(t, getJSON(t, idx, "/check-model?include_drift="+q))
		if res["drift"] != nil || res["drift_error"] != nil {
			t.Fatalf("include_drift=%s ignored: %v", q, res)
		}
	}
}

func TestHumanStatusPatchIsUngatedInDesignStage(t *testing.T) {
	idx, _ := newModel(t)
	id, err := idx.Create(core.CreateInput{Name: "Orders", Type: "service", Spec: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if rec := doJSON(t, idx, http.MethodPut, "/method", map[string]string{"id": "design-first"}); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	rec := doJSON(t, idx, http.MethodPatch, "/nodes/"+string(id), map[string]string{"status": "done"})
	if rec.Code != http.StatusOK {
		t.Fatalf("human path gated: %d %s", rec.Code, rec.Body.String())
	}
}
