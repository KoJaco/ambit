package httpapi

import (
	"net/http"

	"github.com/KoJaco/ambit/internal/core"
)

func scopeBody(idx *core.Index) core.ScopeInfo {
	info := idx.ScopeInfo()
	if info.Parts == nil {
		info.Parts = []core.Part{}
	}
	return info
}

// getParts returns the model scope and its parts.
func getParts(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, scopeBody(idx))
	}
}

// putParts is the human surface for scope and parts: it sets the scope and/or adds or
// replaces one part. Agents go through the set_part MCP tool, which stages a proposal.
func putParts(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Scope string     `json:"scope"`
			Part  *core.Part `json:"part"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeBadRequest(w, err)
			return
		}
		if body.Scope == "" && body.Part == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "scope or part is required"})
			return
		}
		if body.Part != nil {
			if _, err := idx.SetPart(*body.Part); err != nil {
				writeAPIError(w, err)
				return
			}
		}
		if body.Scope != "" {
			if err := idx.SetScope(body.Scope); err != nil {
				writeAPIError(w, err)
				return
			}
		}
		writeJSON(w, http.StatusOK, scopeBody(idx))
	}
}

func deletePart(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := idx.DeletePart(r.PathValue("id")); err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, scopeBody(idx))
	}
}

// getProposalPolicy reports the auto-accept policy and whether it is in effect.
func getProposalPolicy(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, idx.ProposalPolicyInfo())
	}
}

// putProposalPolicy turns auto-accept on or off. It is the only way to change the policy:
// no MCP tool can.
func putProposalPolicy(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AutoAccept *bool `json:"auto_accept"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeBadRequest(w, err)
			return
		}
		if body.AutoAccept == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "auto_accept is required"})
			return
		}
		info, err := idx.SetProposalPolicy(core.ProposalPolicy{AutoAccept: *body.AutoAccept})
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, info)
	}
}

func auditQuery(r *http.Request, defaultActor string) core.AuditQuery {
	q := core.AuditQuery{Since: r.URL.Query().Get("since"), Actor: r.URL.Query().Get("actor")}
	if q.Actor == "" {
		q.Actor = defaultActor
	}
	return q
}

// getProposalAudit lists accepted-proposal audit entries (since= RFC 3339, actor=).
func getProposalAudit(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := idx.ReadAudit(auditQuery(r, ""))
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
	}
}

// getProposalAuditDigest groups what the architect agent accepted by rule. The actor
// defaults to architect-agent.
func getProposalAuditDigest(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		d, err := idx.AuditDigestFor(auditQuery(r, core.ActorArchitectAgent))
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, d)
	}
}
