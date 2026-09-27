package httpapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/KoJaco/ambit/internal/core"
)

// Handler is the localhost API. Read and write routes are registered as they land.
func Handler(idx *core.Index) http.Handler {
	return routes(idx, nil)
}

func routes(idx *core.Index, h *hub) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /levels", getRootLevel(idx))
	mux.HandleFunc("GET /levels/{nodeId}", getChildLevel(idx))
	mux.HandleFunc("GET /nodes/{id}", getNode(idx))
	mux.HandleFunc("POST /nodes", postNode(idx))
	mux.HandleFunc("PATCH /nodes/{id}", patchNode(idx))
	mux.HandleFunc("DELETE /nodes/{id}", deleteNode(idx))
	mux.HandleFunc("PUT /relationships", putRelationship(idx))
	mux.HandleFunc("PUT /assignment", putAssignment(idx))
	mux.HandleFunc("DELETE /assignment", deleteAssignment(idx))
	mux.HandleFunc("GET /layout/{key}", getLayout(idx))
	mux.HandleFunc("PUT /layout/{key}", putLayout(idx))
	mux.HandleFunc("GET /integrity", getIntegrity(idx))
	mux.HandleFunc("GET /events", getEvents(h))
	return mux
}

// ListenAndServe binds addr after ValidateAddr and serves Handler.
// It prints the bound address to stdout. There is no write timeout: the later
// SSE stream is a long-lived response.
func ListenAndServe(addr string, idx *core.Index) error {
	if err := ValidateAddr(addr); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := newHub(idx)
	if err := h.start(ctx); err != nil {
		return err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Printf("listening on %s\n", ln.Addr().String())
	srv := &http.Server{
		Handler:           routes(idx, h),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.Serve(ln)
}
