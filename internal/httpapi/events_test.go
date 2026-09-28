package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KoJaco/ambit/internal/core"
)

func TestEventsAreOneWayAndNameNodes(t *testing.T) {
	idx, dir := newModel(t)
	id, err := idx.Create(core.CreateInput{Name: "Orders", Type: "service", Spec: "s"})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	Handler(idx).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(`{"node_ids":["orders"]}`)))
	if rec.Code == http.StatusOK {
		t.Fatalf("client write was accepted: %d %s", rec.Code, rec.Body.String())
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
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("content-type %s", resp.Header.Get("Content-Type"))
	}

	path := filepath.Join(dir, ".arch", "nodes", string(id)+".json")
	obj := readObj(t, path)
	obj["name"] = "Orders Two"
	writeObj(t, path, obj)

	events := collectEvents(t, resp, 3*time.Second, func(name, data string) bool {
		if name == "proposals-changed" {
			t.Fatalf("emitted proposal event: %s %s", name, data)
		}
		return name == "model-changed" && strings.Contains(data, string(id))
	})
	if len(events) == 0 {
		t.Fatal("no model-changed event")
	}
}

func TestIntegrityChangedWhenWarningsChange(t *testing.T) {
	idx, dir := newModel(t)
	if _, err := idx.Create(core.CreateInput{Name: "Platform", Type: "boundary"}); err != nil {
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

	orphan := filepath.Join(dir, ".arch", "nodes", "rogue.json")
	if err := os.WriteFile(orphan, []byte("{\n    \"id\": \"rogue\",\n    \"name\": \"Rogue\",\n    \"type\": \"service\",\n    \"status\": \"draft\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	found := false
	collectEvents(t, resp, 3*time.Second, func(name, data string) bool {
		if name == "proposals-changed" {
			t.Fatalf("emitted proposal event")
		}
		if name == "integrity-changed" {
			found = true
			return true
		}
		return false
	})
	if !found {
		t.Fatal("integrity-changed did not fire")
	}
}

func TestProposalsChangedOnStageAndAccept(t *testing.T) {
	idx, _ := newModel(t)
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

	pid, err := idx.Stage(core.StageInput{
		Source: core.OpCreateNode,
		Ops:    []core.StageOp{{Op: core.OpCreateNode, Create: core.CreateInput{Name: "Orders", Type: "service", Spec: "o"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	saw := 0
	events := collectEvents(t, resp, 4*time.Second, func(name, data string) bool {
		if name != "proposals-changed" {
			return false
		}
		if data != "{}" {
			t.Fatalf("payload %s", data)
		}
		saw++
		if saw == 1 {
			if err := idx.AcceptOperation(pid, 0, false); err != nil {
				t.Errorf("accept %v", err)
			}
			return false
		}
		return true
	})
	if saw < 2 {
		t.Fatalf("proposals-changed count %d, events %v", saw, events)
	}
}

func collectEvents(t *testing.T, resp *http.Response, wait time.Duration, done func(name, data string) bool) []string {
	t.Helper()
	type pair struct {
		name string
		data string
	}
	out := make(chan pair, 8)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		var name, data string
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				data = strings.TrimPrefix(line, "data: ")
			case line == "" && name != "":
				out <- pair{name, data}
				name, data = "", ""
			}
		}
	}()
	deadline := time.After(wait)
	var got []string
	for {
		select {
		case ev := <-out:
			got = append(got, ev.name)
			if done(ev.name, ev.data) {
				return got
			}
		case <-deadline:
			return got
		}
	}
}

func readObj(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	return obj
}

func writeObj(t *testing.T, path string, obj map[string]any) {
	t.Helper()
	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
