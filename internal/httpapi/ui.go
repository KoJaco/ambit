package httpapi

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// APIPrefixes lists the top-level path segments owned by the API. A request
// under one of these that reaches the UI fallback is a JSON 404, never HTML.
// Every registered route's first segment must appear here (see TestAPIPrefixesCoverRoutes).
var APIPrefixes = []string{"levels", "nodes", "relationships", "assignment", "layout", "integrity", "proposals", "events"}

// Options configures optional server behavior.
type Options struct {
	// UI is the built frontend (rooted so index.html is at "index.html").
	// Nil means the bundle is not built.
	UI fs.FS
}

func isAPIPath(p string) bool {
	seg := strings.TrimPrefix(p, "/")
	if i := strings.IndexByte(seg, '/'); i >= 0 {
		seg = seg[:i]
	}
	for _, pre := range APIPrefixes {
		if seg == pre {
			return true
		}
	}
	return false
}

const uiMissing = "ambit: the UI bundle is not built in this binary. Run `make build` to embed it.\n"

// uiFallback serves embedded files and the SPA shell for GET/HEAD requests that
// match no API route.
func uiFallback(ui fs.FS) http.HandlerFunc {
	available := false
	if ui != nil {
		if info, err := fs.Stat(ui, "index.html"); err == nil && !info.IsDir() {
			available = true
		}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if isAPIPath(r.URL.Path) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		if !available {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			if r.Method != http.MethodHead {
				_, _ = w.Write([]byte(uiMissing))
			}
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if info, err := fs.Stat(ui, name); err == nil && !info.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				serveFile(w, r, ui, name)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-cache")
		serveFile(w, r, ui, "index.html")
	}
}

func serveFile(w http.ResponseWriter, r *http.Request, ui fs.FS, name string) {
	f, err := ui.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if rs, ok := f.(readSeeker); ok {
		http.ServeContent(w, r, name, info.ModTime(), rs)
		return
	}
	// Non-seekable FS file: read it fully.
	data, err := fs.ReadFile(ui, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, name, info.ModTime(), bytesReader(data))
}

type readSeeker interface {
	Read([]byte) (int, error)
	Seek(int64, int) (int64, error)
}

func bytesReader(b []byte) readSeeker { return bytes.NewReader(b) }
