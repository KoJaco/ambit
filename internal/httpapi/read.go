package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

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
		writeLevel(w, idx, "", expandParam(r))
	}
}

func getChildLevel(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeLevel(w, idx, core.NodeID(r.PathValue("nodeId")), expandParam(r))
	}
}

func getRelationshipLevel(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lvl, err := idx.RelationshipLevel(core.RelID(r.PathValue("id")))
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, relationshipLevelBody(lvl))
	}
}

func relationshipLevelBody(lvl core.RelationshipLevel) relationshipLevelJSON {
	body := relationshipLevelJSON{
		Relationship:  relViewJSON(lvl.Relationship),
		Members:       make([]summaryJSON, 0, len(lvl.Members)),
		Context:       make([]summaryJSON, 0, 2),
		Relationships: make([]relJSON, 0, len(lvl.Relationships)),
		Crossings:     make([]crossJSON, 0, len(lvl.Crossings)),
		Warnings:      warningBodies(lvl.Warnings),
	}
	for _, m := range lvl.Members {
		body.Members = append(body.Members, summaryOf(m))
	}
	body.Context = append(body.Context, summaryOf(lvl.Context[0]), summaryOf(lvl.Context[1]))
	for _, rel := range lvl.Relationships {
		body.Relationships = append(body.Relationships, relJSON{
			ID: string(rel.ID), From: string(rel.From), To: string(rel.To), Label: rel.Label, Kind: rel.Kind,
			Drillable: rel.Drillable,
		})
	}
	for _, c := range lvl.Crossings {
		body.Crossings = append(body.Crossings, crossJSON{
			NodeID: string(c.NodeID), Direction: c.Direction, Label: c.Label, Kind: c.Kind, OtherID: string(c.OtherID),
			RelID: string(c.RelID), Drillable: c.Drillable,
		})
	}
	return body
}

func relViewJSON(v core.RelView) relViewBody {
	return relViewBody{ID: string(v.ID), From: string(v.From), To: string(v.To), Label: v.Label, Kind: v.Kind}
}

type relationshipLevelJSON struct {
	Relationship  relViewBody   `json:"relationship"`
	Members       []summaryJSON `json:"children"`
	Context       []summaryJSON `json:"context"`
	Relationships []relJSON     `json:"relationships"`
	Crossings     []crossJSON   `json:"crossings"`
	Warnings      []warnJSON    `json:"warnings"`
}

type relViewBody struct {
	ID    string `json:"id"`
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
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

// expandParam parses ?expand=a,b into node ids. Blanks are dropped.
func expandParam(r *http.Request) []core.NodeID {
	var out []core.NodeID
	for _, raw := range r.URL.Query()["expand"] {
		for _, part := range strings.Split(raw, ",") {
			if id := strings.TrimSpace(part); id != "" {
				out = append(out, core.NodeID(id))
			}
		}
	}
	return out
}

func writeLevel(w http.ResponseWriter, idx *core.Index, parent core.NodeID, expanded []core.NodeID) {
	lvl, err := idx.Level(parent, expanded)
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
		ViewDefaults:  viewDefaultsJSON{GroupMax: lvl.ViewDefaults.GroupMax, LevelBudget: lvl.ViewDefaults.LevelBudget},
	}
	if lvl.Node != nil {
		s := summaryOf(*lvl.Node)
		body.Node = &s
	}
	for _, child := range lvl.Children {
		body.Children = append(body.Children, summaryOf(child))
	}
	for _, rel := range lvl.Relationships {
		body.Relationships = append(body.Relationships, edgeBody(rel))
	}
	for _, c := range lvl.Crossings {
		body.Crossings = append(body.Crossings, crossJSON{
			NodeID: string(c.NodeID), Direction: c.Direction, Label: c.Label, Kind: c.Kind, OtherID: string(c.OtherID),
			RelID: string(c.RelID), Drillable: c.Drillable,
		})
	}
	return body
}

func edgeBody(rel core.EdgeView) relJSON {
	out := relJSON{
		ID: string(rel.ID), From: string(rel.From), To: string(rel.To), Label: rel.Label, Kind: rel.Kind,
		Drillable: rel.Drillable, Aggregated: rel.Aggregated, Kinds: rel.Kinds,
	}
	for _, m := range rel.Members {
		out.Members = append(out.Members, string(m))
	}
	return out
}

func summaryOf(n core.NodeView) summaryJSON {
	out := summaryJSON{
		ID: string(n.ID), Name: n.Name, Type: n.Type, Status: string(n.Status), Protected: n.Protected,
		Icon: n.Icon, Color: n.Color, Renderer: n.Renderer, Schema: n.Schema, ChildCount: n.ChildCount,
		GroupID: string(n.GroupID), Expanded: n.Expanded, InternalRelCount: n.InternalRelCount,
	}
	for _, t := range n.TypeMix {
		out.TypeMix = append(out.TypeMix, typeCountJSON{Type: t.Type, Count: t.Count})
	}
	return out
}

func warningBodies(in []core.Diagnostic) []warnJSON {
	out := make([]warnJSON, 0, len(in))
	for _, d := range in {
		out = append(out, warnJSON{Severity: string(d.Severity), Path: d.Path, Message: d.Message})
	}
	return out
}

type levelJSON struct {
	Node          *summaryJSON     `json:"node"`
	Children      []summaryJSON    `json:"children"`
	Relationships []relJSON        `json:"relationships"`
	Crossings     []crossJSON      `json:"crossings"`
	Warnings      []warnJSON       `json:"warnings"`
	ViewDefaults  viewDefaultsJSON `json:"view_defaults"`
}

type viewDefaultsJSON struct {
	GroupMax    int `json:"group_max"`
	LevelBudget int `json:"level_budget"`
}

type typeCountJSON struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

type nodeJSON struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	Type           string              `json:"type"`
	Status         string              `json:"status"`
	ParentID       string              `json:"parent_id,omitempty"`
	RelationshipID string              `json:"relationship_id,omitempty"`
	Implementation []string            `json:"implementation"`
	Scope          []string            `json:"scope"`
	Protected      bool                `json:"protected"`
	Markdown       string              `json:"markdown"`
	Icon           string              `json:"icon,omitempty"`
	Color          string              `json:"color,omitempty"`
	Renderer       string              `json:"renderer,omitempty"`
	Schema         []core.SchemaColumn `json:"schema,omitempty"`
}

type summaryJSON struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Type       string              `json:"type"`
	Status     string              `json:"status"`
	Protected  bool                `json:"protected"`
	Icon       string              `json:"icon,omitempty"`
	Color      string              `json:"color,omitempty"`
	Renderer   string              `json:"renderer,omitempty"`
	Schema     []core.SchemaColumn `json:"schema,omitempty"`
	ChildCount int                 `json:"child_count"`
	GroupID    string              `json:"group_id,omitempty"`
	Expanded   bool                `json:"expanded,omitempty"`
	// InternalRelCount is relationships hidden inside a collapsed node.
	InternalRelCount int             `json:"internal_rel_count,omitempty"`
	TypeMix          []typeCountJSON `json:"type_mix,omitempty"`
}

type relJSON struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Label     string `json:"label"`
	Kind      string `json:"kind"`
	Drillable bool   `json:"drillable,omitempty"`
	// Aggregated edges merge rolled-up relationships; Members lists their ids.
	Aggregated bool     `json:"aggregated,omitempty"`
	Members    []string `json:"members,omitempty"`
	Kinds      []string `json:"kinds,omitempty"`
}

type crossJSON struct {
	NodeID    string `json:"node_id"`
	Direction string `json:"direction"`
	Label     string `json:"label"`
	Kind      string `json:"kind"`
	OtherID   string `json:"other_id"`
	RelID     string `json:"relationship_id,omitempty"`
	Drillable bool   `json:"drillable,omitempty"`
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
	case errors.Is(err, core.ErrNotFound), errors.Is(err, core.ErrProposalNotFound),
		errors.Is(err, core.ErrRelationshipNotFound), errors.Is(err, core.ErrEmptyRelationship):
		code = http.StatusNotFound
	case errors.Is(err, core.ErrCycle),
		errors.Is(err, core.ErrHasChildren),
		errors.Is(err, core.ErrNoSlug),
		errors.Is(err, core.ErrBadStatus),
		errors.Is(err, core.ErrBadKind),
		errors.Is(err, core.ErrMissingEndpoint),
		errors.Is(err, core.ErrMissingParent),
		errors.Is(err, core.ErrEmptyName),
		errors.Is(err, core.ErrEmptyType),
		errors.Is(err, core.ErrStale),
		errors.Is(err, core.ErrUnreadableProposal),
		errors.Is(err, core.ErrExists),
		errors.Is(err, core.ErrContainerConflict):
		code = http.StatusConflict
	case errors.Is(err, core.ErrInvalidLayoutKey), errors.Is(err, core.ErrBadOperation),
		errors.Is(err, core.ErrInvalidPresentation):
		code = http.StatusBadRequest
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
