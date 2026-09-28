package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/KoJaco/ambit/internal/core"
)

func postNode(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body createJSON
		if err := decodeJSON(r, &body); err != nil {
			writeBadRequest(w, err)
			return
		}
		id, err := idx.Create(core.CreateInput{
			Name:           body.Name,
			Type:           body.Type,
			Spec:           body.Markdown,
			ParentID:       core.NodeID(body.ParentID),
			RelationshipID: core.RelID(body.RelationshipID),
			Implementation: body.Implementation,
			Scope:          body.Scope,
			Protected:      body.Protected,
			Status:         core.Status(body.Status),
		})
		if err != nil {
			writeAPIError(w, err)
			return
		}
		n, err := idx.Node(id)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, nodeBody(n))
	}
}

func patchNode(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body patchJSON
		if err := decodeJSON(r, &body); err != nil {
			writeBadRequest(w, err)
			return
		}
		id := core.NodeID(r.PathValue("id"))
		in := core.UpdateInput{
			Name:           body.Name,
			Type:           body.Type,
			Spec:           body.Markdown,
			Implementation: body.Implementation,
			Scope:          body.Scope,
			Protected:      body.Protected,
		}
		if body.Status != nil {
			status := core.Status(*body.Status)
			in.Status = &status
		}
		if body.ParentID != nil {
			parent := core.NodeID(*body.ParentID)
			in.Parent = &parent
		}
		if err := idx.Update(id, in); err != nil {
			writeAPIError(w, err)
			return
		}
		n, err := idx.Node(id)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, nodeBody(n))
	}
}

func deleteNode(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := core.NodeID(r.PathValue("id"))
		if err := idx.Delete(id); err != nil {
			writeAPIError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func putRelationship(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body relJSON
		if err := decodeJSON(r, &body); err != nil {
			writeBadRequest(w, err)
			return
		}
		rel, err := idx.SetRelationship(core.SetRelationshipInput{
			ID:    core.RelID(body.ID),
			From:  core.NodeID(body.From),
			To:    core.NodeID(body.To),
			Label: body.Label,
			Kind:  body.Kind,
		})
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, relBody(rel))
	}
}

func deleteRelationship(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := core.RelID(r.PathValue("id"))
		if err := idx.DeleteRelationship(id); err != nil {
			writeAPIError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func relBody(rel core.Relationship) relJSON {
	return relJSON{
		ID: string(rel.ID), From: string(rel.From), To: string(rel.To), Label: rel.Label, Kind: rel.Kind,
	}
}

func putAssignment(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			NodeID     string `json:"node_id"`
			AssignedAt string `json:"assigned_at"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeBadRequest(w, err)
			return
		}
		if body.NodeID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node_id is required"})
			return
		}
		var at time.Time
		if body.AssignedAt != "" {
			parsed, err := time.Parse(time.RFC3339, body.AssignedAt)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "assigned_at must be RFC3339"})
				return
			}
			at = parsed
		}
		if err := idx.SetAssignment(core.NodeID(body.NodeID), at); err != nil {
			writeAPIError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteAssignment(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := idx.ClearAssignment(); err != nil {
			writeAPIError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func nodeBody(n core.NodeView) nodeJSON {
	impl := n.Implementation
	if impl == nil {
		impl = []string{}
	}
	scope := n.Scope
	if scope == nil {
		scope = []string{}
	}
	return nodeJSON{
		ID:             string(n.ID),
		Name:           n.Name,
		Type:           n.Type,
		Status:         string(n.Status),
		ParentID:       string(n.ParentID),
		RelationshipID: string(n.RelationshipID),
		Implementation: impl,
		Scope:          scope,
		Protected:      n.Protected,
		Markdown:       n.Spec,
	}
}

func decodeJSON(r *http.Request, dest any) error {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dest); err != nil {
		if err == io.EOF {
			return err
		}
		return err
	}
	return nil
}

func writeBadRequest(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusBadRequest, map[string]string{
		"error": "malformed JSON: " + err.Error(),
	})
}

type createJSON struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	Status         string   `json:"status"`
	ParentID       string   `json:"parent_id"`
	RelationshipID string   `json:"relationship_id"`
	Implementation []string `json:"implementation"`
	Scope          []string `json:"scope"`
	Protected      bool     `json:"protected"`
	Markdown       string   `json:"markdown"`
}

type patchJSON struct {
	Name           *string   `json:"name"`
	Type           *string   `json:"type"`
	Status         *string   `json:"status"`
	ParentID       *string   `json:"parent_id"`
	Implementation *[]string `json:"implementation"`
	Scope          *[]string `json:"scope"`
	Protected      *bool     `json:"protected"`
	Markdown       *string   `json:"markdown"`
}
