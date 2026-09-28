package httpapi

import (
	"net/http"

	"github.com/KoJaco/ambit/internal/core"
)

func getLayout(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		layout, err := idx.Layout(r.PathValue("key"))
		if err != nil {
			writeLayoutError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, layout)
	}
}

func putLayout(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var layout core.Layout
		if err := decodeJSON(r, &layout); err != nil {
			writeBadRequest(w, err)
			return
		}
		if err := idx.PutLayout(r.PathValue("key"), layout); err != nil {
			writeLayoutError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, layout)
	}
}

func writeLayoutError(w http.ResponseWriter, err error) {
	writeAPIError(w, err)
}
