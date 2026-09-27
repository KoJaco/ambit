package httpapi

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/KoJaco/ambit/internal/core"
)

// Handler is the localhost API. Read and write routes are registered as they land.
func Handler(idx *core.Index) http.Handler {
	return routes(idx)
}

func routes(idx *core.Index) http.Handler {
	mux := http.NewServeMux()
	_ = idx
	return mux
}

// ListenAndServe binds addr after ValidateAddr and serves Handler.
// It prints the bound address to stdout. There is no write timeout: the later
// SSE stream is a long-lived response.
func ListenAndServe(addr string, idx *core.Index) error {
	if err := ValidateAddr(addr); err != nil {
		return err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Printf("listening on %s\n", ln.Addr().String())
	srv := &http.Server{
		Handler:           Handler(idx),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.Serve(ln)
}
