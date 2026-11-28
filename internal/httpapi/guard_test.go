package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func guardReq(method, path, body string, hdr map[string]string) *http.Request {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	r.Host = "127.0.0.1:8080"
	for k, v := range hdr {
		if k == "Host" {
			r.Host = v
			continue
		}
		r.Header.Set(k, v)
	}
	return r
}

func TestGuardWrites(t *testing.T) {
	idx, _ := newModel(t)
	h := Handler(idx)
	js := map[string]string{"Content-Type": "application/json"}
	with := func(extra map[string]string) map[string]string {
		m := map[string]string{"Content-Type": "application/json"}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	cases := []struct {
		name   string
		method string
		path   string
		body   string
		hdr    map[string]string
		want   int // 0 = anything but the guard codes
	}{
		{"text/plain post", "POST", "/nodes", `{"name":"x"}`, map[string]string{"Content-Type": "text/plain"}, 415},
		{"no content type post", "POST", "/nodes", `{"name":"x"}`, nil, 415},
		{"form post", "POST", "/nodes", `a=b`, map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, 415},
		{"put text/plain", "PUT", "/constraints", `{}`, map[string]string{"Content-Type": "text/plain"}, 415},
		{"patch text/plain", "PATCH", "/nodes/x", `{}`, map[string]string{"Content-Type": "text/plain"}, 415},
		{"bad host", "POST", "/nodes", `{}`, with(map[string]string{"Host": "evil.example.com"}), 403},
		{"rebind host with port", "PUT", "/stage", `{}`, with(map[string]string{"Host": "attacker.test:8080"}), 403},
		{"delete bad host", "DELETE", "/nodes/x", "", map[string]string{"Host": "evil.example.com"}, 403},
		{"cross origin", "POST", "/nodes", `{}`, with(map[string]string{"Origin": "https://evil.example.com"}), 403},
		{"null origin", "PUT", "/stage", `{}`, with(map[string]string{"Origin": "null"}), 403},
		{"delete cross origin", "DELETE", "/nodes/x", "", map[string]string{"Origin": "https://evil.example.com"}, 403},
		{"patch cross-site fetch", "PATCH", "/nodes/x", `{}`, with(map[string]string{"Sec-Fetch-Site": "cross-site"}), 403},
		{"loopback post", "POST", "/nodes", `{"name":"Zed","type":"service","spec":"s"}`, js, 0},
		{"localhost host", "POST", "/nodes", `{"name":"Zed2","type":"service","spec":"s"}`, with(map[string]string{"Host": "localhost:8080"}), 0},
		{"ipv6 host", "PUT", "/stage", `{}`, with(map[string]string{"Host": "[::1]:8080"}), 0},
		{"vite proxy origin", "POST", "/nodes", `{"name":"Zed3","type":"service","spec":"s"}`, with(map[string]string{"Origin": "http://localhost:5173", "Sec-Fetch-Site": "same-origin"}), 0},
		{"delete loopback no body", "DELETE", "/nodes/nope", "", nil, 0},
		{"json charset", "POST", "/nodes", `{"name":"Zed4","type":"service","spec":"s"}`, map[string]string{"Content-Type": "application/json; charset=utf-8"}, 0},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, guardReq(c.method, c.path, c.body, c.hdr))
		if c.want == 0 {
			if rec.Code == 403 || rec.Code == 415 {
				t.Errorf("%s: guard rejected valid request: %d %s", c.name, rec.Code, rec.Body.String())
			}
		} else if rec.Code != c.want {
			t.Errorf("%s: got %d want %d (%s)", c.name, rec.Code, c.want, rec.Body.String())
		}
	}
}

func TestGuardLeavesReadsAlone(t *testing.T) {
	idx, _ := newModel(t)
	r := httptest.NewRequest("GET", "/levels", nil) // Host example.com
	r.Header.Set("Origin", "https://other.example")
	rec := httptest.NewRecorder()
	Handler(idx).ServeHTTP(rec, r)
	if rec.Code != 200 {
		t.Fatalf("GET %d", rec.Code)
	}
}
