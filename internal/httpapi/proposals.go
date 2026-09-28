package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/KoJaco/ambit/internal/core"
)

func getProposals(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := idx.ListProposals()
		if err != nil {
			writeAPIError(w, err)
			return
		}
		if list == nil {
			list = []core.ProposalSummary{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"proposals": list})
	}
}

func getProposal(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		diff, err := idx.ProposalDiff(r.PathValue("id"))
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, diff)
	}
}

func postAcceptOperation(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		index, err := operationIndex(r)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		var body struct {
			ConfirmStale bool `json:"confirm_stale"`
		}
		if err := decodeOptionalJSON(r, &body); err != nil {
			writeBadRequest(w, err)
			return
		}
		if err := idx.AcceptOperation(r.PathValue("id"), index, body.ConfirmStale); err != nil {
			writeAPIError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postRejectOperation(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		index, err := operationIndex(r)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		if err := idx.RejectOperation(r.PathValue("id"), index); err != nil {
			writeAPIError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postAcceptAll(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := idx.AcceptAll(r.PathValue("id"))
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func postRejectAll(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := idx.RejectAll(r.PathValue("id")); err != nil {
			writeAPIError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteProposal(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := idx.DeleteProposal(r.PathValue("id")); err != nil {
			writeAPIError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func operationIndex(r *http.Request) (int, error) {
	raw := r.PathValue("index")
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%w: operation index %q", core.ErrBadOperation, raw)
	}
	return n, nil
}

func decodeOptionalJSON(r *http.Request, dest any) error {
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(dest)
	if err == io.EOF {
		return nil
	}
	return err
}
