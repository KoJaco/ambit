package httpapi

import (
	"net/http"
	"testing"
)

func TestLayoutGetMissingIsEmpty(t *testing.T) {
	idx, _ := newModel(t)
	body := getJSON(t, idx, "/layout/_root")
	if body != "{\"positions\":[]}\n" && body != "{\"positions\":[]}" {
		t.Fatalf("body %q", body)
	}
	rec := doJSON(t, idx, http.MethodPut, "/layout/_root", map[string]any{
		"positions": []map[string]any{{"id": "payments-service", "x": 1.5, "y": 2}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("put %d %s", rec.Code, rec.Body.String())
	}
}
