package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/KoJaco/ambit/internal/core"
)

type hub struct {
	idx  *core.Index
	mu   sync.Mutex
	subs map[chan core.WatchNotice]struct{}
}

func newHub(idx *core.Index) *hub {
	return &hub{idx: idx, subs: map[chan core.WatchNotice]struct{}{}}
}

func (h *hub) start(ctx context.Context) error {
	return h.idx.WatchNotices(ctx, func(n core.WatchNotice) {
		h.mu.Lock()
		defer h.mu.Unlock()
		for ch := range h.subs {
			select {
			case ch <- n:
			default:
			}
		}
	})
}

func (h *hub) subscribe() chan core.WatchNotice {
	ch := make(chan core.WatchNotice, 8)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *hub) unsubscribe(ch chan core.WatchNotice) {
	h.mu.Lock()
	delete(h.subs, ch)
	h.mu.Unlock()
}

func getEvents(h *hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "change stream is not running"})
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming is not available"})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher.Flush()
		ch := h.subscribe()
		defer h.unsubscribe(ch)
		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case n := <-ch:
				if n.Err != nil || !n.ModelChanged && !n.IntegrityChanged {
					continue
				}
				if n.ModelChanged {
					if err := writeSSE(w, "model-changed", map[string]any{"node_ids": nodeIDStrings(n.NodeIDs)}); err != nil {
						return
					}
				}
				if n.IntegrityChanged {
					if err := writeSSE(w, "integrity-changed", map[string]any{}); err != nil {
						return
					}
				}
				flusher.Flush()
			}
		}
	}
}

func nodeIDStrings(ids []core.NodeID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}

func writeSSE(w http.ResponseWriter, event string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, raw)
	return err
}
