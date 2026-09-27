package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/KoJaco/ambit/internal/core"
)

func getIntegrity(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"warnings": warningBodies(idx.Warnings()),
		})
	}
}

func getRootLevel(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeLevel(w, idx, "")
	}
}

func getChildLevel(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeLevel(w, idx, core.NodeID(r.PathValue("nodeId")))
	}
}

func getNode(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := core.NodeID(r.PathValue("id"))
		n, err := idx.Node(id)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, nodeBody(n))
	}
}

func writeLevel(w http.ResponseWriter, idx *core.Index, parent core.NodeID) {
	lvl, err := idx.Level(parent)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, levelBody(lvl))
}

func levelBody(lvl core.Level) levelJSON {
	body := levelJSON{
		Children:      make([]summaryJSON, 0, len(lvl.Children)),
		Relationships: make([]relJSON, 0, len(lvl.Relationships)),
		Crossings:     make([]crossJSON, 0, len(lvl.Crossings)),
		Warnings:      warningBodies(lvl.Warnings),
	}
	if lvl.Node != nil {
		s := summaryOf(*lvl.Node)
		body.Node = &s
	}
	for _, child := range lvl.Children {
		body.Children = append(body.Children, summaryOf(child))
	}
	for _, rel := range lvl.Relationships {
		body.Relationships = append(body.Relationships, relJSON{
			From: string(rel.From), To: string(rel.To), Label: rel.Label, Kind: rel.Kind,
		})
	}
	for _, c := range lvl.Crossings {
		body.Crossings = append(body.Crossings, crossJSON{
			NodeID: string(c.NodeID), Direction: c.Direction, Label: c.Label, Kind: c.Kind, OtherID: string(c.OtherID),
		})
	}
	return body
}

func summaryOf(n core.NodeView) summaryJSON {
	return summaryJSON{
		ID: string(n.ID), Name: n.Name, Type: n.Type, Status: string(n.Status), Protected: n.Protected,
	}
}

func warningBodies(in []core.Diagnostic) []warnJSON {
	out := make([]warnJSON, 0, len(in))
	for _, d := range in {
		out = append(out, warnJSON{Severity: string(d.Severity), Path: d.Path, Message: d.Message})
	}
	return out
}

type levelJSON struct {
	Node          *summaryJSON  `json:"node"`
	Children      []summaryJSON `json:"children"`
	Relationships []relJSON     `json:"relationships"`
	Crossings     []crossJSON   `json:"crossings"`
	Warnings      []warnJSON    `json:"warnings"`
}

type nodeJSON struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	Status         string   `json:"status"`
	ParentID       string   `json:"parent_id,omitempty"`
	Implementation []string `json:"implementation"`
	Scope          []string `json:"scope"`
	Protected      bool     `json:"protected"`
	Markdown       string   `json:"markdown"`
}

type summaryJSON struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Protected bool   `json:"protected"`
}

type relJSON struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}

type crossJSON struct {
	NodeID    string `json:"node_id"`
	Direction string `json:"direction"`
	Label     string `json:"label"`
	Kind      string `json:"kind"`
	OtherID   string `json:"other_id"`
}

type warnJSON struct {
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Message  string `json:"message"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeAPIError(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, core.ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, core.ErrCycle),
		errors.Is(err, core.ErrHasChildren),
		errors.Is(err, core.ErrNoSlug),
		errors.Is(err, core.ErrBadStatus),
		errors.Is(err, core.ErrBadKind),
		errors.Is(err, core.ErrMissingEndpoint),
		errors.Is(err, core.ErrMissingParent),
		errors.Is(err, core.ErrEmptyName),
		errors.Is(err, core.ErrEmptyType):
		code = http.StatusConflict
	case errors.Is(err, core.ErrInvalidLayoutKey):
		code = http.StatusBadRequest
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
