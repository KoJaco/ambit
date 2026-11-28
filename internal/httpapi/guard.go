package httpapi

import (
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// guardWrites protects state-changing routes (everything but GET/HEAD/OPTIONS)
// from cross-site requests and DNS rebinding:
//   - Host must be a loopback name (localhost, 127.0.0.1, [::1]), any port.
//   - Origin, when sent, must be a loopback origin (this keeps the Vite dev
//     proxy working while rejecting pages served from other sites).
//   - Sec-Fetch-Site: cross-site is rejected.
//   - A request with a body must be application/json, so a cross-origin form
//     or text/plain "simple" request can never reach a handler.
func guardWrites(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if !isLoopbackHost(r.Host) {
			forbid(w, "host not allowed")
			return
		}
		if o := r.Header.Get("Origin"); o != "" {
			u, err := url.Parse(o)
			if o == "null" || err != nil || !isLoopbackHost(u.Host) {
				forbid(w, "cross-origin request rejected")
				return
			}
		}
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			forbid(w, "cross-site request rejected")
			return
		}
		if r.ContentLength != 0 || len(r.TransferEncoding) > 0 {
			mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || mt != "application/json" {
				writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{
					"error": "Content-Type must be application/json",
				})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func forbid(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusForbidden, map[string]string{"error": msg})
}

// isLoopbackHost reports whether a Host or Origin authority (host[:port]) names
// localhost or a loopback IP.
func isLoopbackHost(hostport string) bool {
	h := hostport
	if hh, _, err := net.SplitHostPort(hostport); err == nil {
		h = hh
	}
	h = strings.TrimSuffix(strings.TrimPrefix(h, "["), "]")
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}
