package httpapi

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"time"

	"github.com/KoJaco/ambit/internal/core"
	"github.com/KoJaco/ambit/internal/webui"
)

// Handler is the localhost API plus, when opts.UI is set, the embedded UI.
func Handler(idx *core.Index, opts ...Options) http.Handler {
	return routes(idx, nil, opts...)
}

type route struct {
	pattern string
	handler http.HandlerFunc
}

// apiRoutes is the single table of registered API routes.
func apiRoutes(idx *core.Index, h *hub) []route {
	return []route{
		{"GET /levels", getRootLevel(idx)},
		{"GET /levels/relationships/{id}", getRelationshipLevel(idx)},
		{"GET /levels/{nodeId}", getChildLevel(idx)},
		{"GET /nodes/{id}", getNode(idx)},
		{"POST /nodes", postNode(idx)},
		{"PATCH /nodes/{id}", patchNode(idx)},
		{"DELETE /nodes/{id}", deleteNode(idx)},
		{"PUT /relationships", putRelationship(idx)},
		{"DELETE /relationships/{id}", deleteRelationship(idx)},
		{"GET /assignment", getAssignment(idx)},
		{"PUT /assignment", putAssignment(idx)},
		{"DELETE /assignment", deleteAssignment(idx)},
		{"GET /layout/{key}", getLayout(idx)},
		{"PUT /layout/{key}", putLayout(idx)},
		{"GET /constraints", getConstraints(idx)},
		{"PUT /constraints", putConstraints(idx)},
		{"GET /build-order", getBuildOrder(idx)},
		{"GET /integrity", getIntegrity(idx)},
		{"GET /icons", getIcons},
		{"GET /proposals", getProposals(idx)},
		{"GET /proposals/{id}", getProposal(idx)},
		{"POST /proposals/group", postGroupProposal(idx)},
		{"POST /proposals/ungroup", postUngroupProposal(idx)},
		{"POST /proposals/reparent", postReparentProposal(idx)},
		{"POST /proposals/{id}/operations/{index}/accept", postAcceptOperation(idx)},
		{"POST /proposals/{id}/operations/{index}/reject", postRejectOperation(idx)},
		{"POST /proposals/{id}/accept", postAcceptAll(idx)},
		{"POST /proposals/{id}/reject", postRejectAll(idx)},
		{"DELETE /proposals/{id}", deleteProposal(idx)},
		{"GET /method", getMethod(idx)},
		{"GET /methods", getMethods(idx)},
		{"PUT /method", putMethod(idx)},
		{"GET /stage", getStage(idx)},
		{"PUT /stage", putStage(idx)},
		{"GET /check-model", getCheckModel(idx)},
		{"GET /events", getEvents(h)},
	}
}

func routes(idx *core.Index, h *hub, opts ...Options) http.Handler {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	mux := http.NewServeMux()
	for _, r := range apiRoutes(idx, h) {
		mux.HandleFunc(r.pattern, r.handler)
	}
	// Fallback for everything else: UI files, SPA shell, or JSON 404 under API prefixes.
	mux.HandleFunc("GET /", uiFallback(o.UI))
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
	var ui fs.FS
	if webui.Available() {
		ui = webui.FS()
		fmt.Println("ui: embedded")
	} else {
		fmt.Println("ui: not built (run make build)")
	}
	srv := &http.Server{
		Handler:           routes(idx, h, Options{UI: ui}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.Serve(ln)
}
