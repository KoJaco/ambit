package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testUI() fstest.MapFS {
	return fstest.MapFS{
		"index.html":        {Data: []byte("<!doctype html><title>ambit</title>")},
		"assets/app-abc.js": {Data: []byte("console.log(1)")},
		"favicon.ico":       {Data: []byte("ico")},
	}
}

func do(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestUIShellForClientRoutes(t *testing.T) {
	idx, _ := newModel(t)
	h := Handler(idx, Options{UI: testUI()})
	for _, p := range []string{"/", "/node/x", "/relationship/y", "/index.html"} {
		rec := do(t, h, http.MethodGet, p)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<title>ambit") {
			t.Fatalf("%s: %d %q", p, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "no-cache" {
			t.Fatalf("%s cache %q", p, rec.Header().Get("Cache-Control"))
		}
	}
}

func TestUIAssets(t *testing.T) {
	idx, _ := newModel(t)
	h := Handler(idx, Options{UI: testUI()})
	rec := do(t, h, http.MethodGet, "/assets/app-abc.js")
	if rec.Code != 200 || rec.Body.String() != "console.log(1)" {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("cache %q", rec.Header().Get("Cache-Control"))
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("content-type %q", ct)
	}
	if rec := do(t, h, http.MethodGet, "/favicon.ico"); rec.Code != 200 || rec.Body.String() != "ico" {
		t.Fatalf("favicon %d", rec.Code)
	}
}

func TestUIFallbackNeverShadowsAPI(t *testing.T) {
	idx, _ := newModel(t)
	h := Handler(idx, Options{UI: testUI()})
	for _, p := range []string{"/nodes/zzz", "/nodes", "/relationships", "/assignment", "/levels/zzz", "/proposals/zzz/x/y", "/nodes/"} {
		rec := do(t, h, http.MethodGet, p)
		if rec.Code != 404 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") ||
			!strings.Contains(rec.Body.String(), `"error"`) || strings.Contains(rec.Body.String(), "<!doctype") {
			t.Fatalf("%s: %d %s %q", p, rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
		}
	}
}

func TestUIFallbackMethods(t *testing.T) {
	idx, _ := newModel(t)
	h := Handler(idx, Options{UI: testUI()})
	rec := do(t, h, http.MethodPost, "/node/x")
	if rec.Code == 200 || strings.Contains(rec.Body.String(), "<!doctype") {
		t.Fatalf("POST: %d %q", rec.Code, rec.Body.String())
	}
	rec = do(t, h, http.MethodHead, "/node/x")
	if rec.Code != 200 || rec.Body.Len() != 0 {
		t.Fatalf("HEAD: %d %q", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodHead, "/assets/app-abc.js"); rec.Code != 200 {
		t.Fatalf("HEAD asset %d", rec.Code)
	}
}

func TestUIMissingMessage(t *testing.T) {
	idx, _ := newModel(t)
	for _, h := range []http.Handler{Handler(idx), Handler(idx, Options{UI: fstest.MapFS{}})} {
		rec := do(t, h, http.MethodGet, "/")
		if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "make build") {
			t.Fatalf("%d %q", rec.Code, rec.Body.String())
		}
		if rec := do(t, h, http.MethodGet, "/levels"); rec.Code != 200 {
			t.Fatalf("API broken without UI: %d", rec.Code)
		}
	}
}

func TestAPIPrefixesCoverRoutes(t *testing.T) {
	known := map[string]bool{}
	for _, p := range APIPrefixes {
		known[p] = true
	}
	for _, r := range apiRoutes(nil, nil) {
		_, path, ok := strings.Cut(r.pattern, " ")
		if !ok {
			t.Fatalf("pattern %q has no method", r.pattern)
		}
		seg := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)[0]
		if !known[seg] {
			t.Errorf("route %q: first segment %q missing from APIPrefixes", r.pattern, seg)
		}
	}
}
