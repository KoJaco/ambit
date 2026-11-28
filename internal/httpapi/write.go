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
			ID:             core.NodeID(body.ID),
			Name:           body.Name,
			Type:           body.Type,
			Spec:           body.Markdown,
			ParentID:       core.NodeID(body.ParentID),
			RelationshipID: core.RelID(body.RelationshipID),
			Implementation: body.Implementation,
			Scope:          body.Scope,
			Protected:      body.Protected,
			Risk:           body.Risk,
			Review:         body.Review,
			Verify:         body.Verify,
			Status:         core.Status(body.Status),
			Icon:           body.Icon,
			Color:          body.Color,
			Renderer:       body.Renderer,
			Schema:         body.Schema,
			DependsOn:      toNodeIDs(body.DependsOn),
			Contract:       body.Contract,
			NodeExtras:     body.NodeExtras,
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
		writeJSON(w, http.StatusCreated, nodeBody(n, assignedID(idx)))
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
			Risk:           body.Risk,
			Review:         body.Review,
			Verify:         body.Verify,

			NodeExtrasPatch: body.NodeExtrasPatch,
		}
		if body.Status != nil {
			status := core.Status(*body.Status)
			in.Status = &status
		}
		if body.ParentID != nil {
			parent := core.NodeID(*body.ParentID)
			in.Parent = &parent
		}
		in.Icon = body.Icon
		in.Color = body.Color
		in.Renderer = body.Renderer
		in.Schema = body.Schema
		in.Contract = body.Contract
		if body.DependsOn != nil {
			deps := toNodeIDs(*body.DependsOn)
			if deps == nil {
				deps = []core.NodeID{}
			}
			in.DependsOn = &deps
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
		writeJSON(w, http.StatusOK, nodeBody(n, assignedID(idx)))
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
			ID:       core.RelID(body.ID),
			From:     core.NodeID(body.From),
			To:       core.NodeID(body.To),
			Label:    body.Label,
			Kind:     body.Kind,
			Contract: body.Contract,
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
		Contract: contractOrNil(rel.Contract),
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

func nodeBody(n core.NodeView, assigned core.NodeID) nodeJSON {
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
		Risk:           n.Risk,
		Review:         n.Review,
		Verify:         n.Verify,
		Markdown:       n.Spec,
		Icon:           n.Icon,
		Color:          n.Color,
		Renderer:       n.Renderer,
		Schema:         n.Schema,
		DependsOn:      dependsOn(n.DependsOn),
		Contract:       nodeContractOrNil(n.Contract),
		Assigned:       assigned != "" && n.ID == assigned,
		NodeExtras:     n.NodeExtras,
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
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	Type           string              `json:"type"`
	Status         string              `json:"status"`
	ParentID       string              `json:"parent_id"`
	RelationshipID string              `json:"relationship_id"`
	Implementation []string            `json:"implementation"`
	Scope          []string            `json:"scope"`
	Protected      bool                `json:"protected"`
	Risk           string              `json:"risk"`
	Review         string              `json:"review"`
	Verify         []string            `json:"verify"`
	Markdown       string              `json:"markdown"`
	Icon           string              `json:"icon"`
	Color          string              `json:"color"`
	Renderer       string              `json:"renderer"`
	Schema         []core.SchemaColumn `json:"schema"`
	DependsOn      []string            `json:"depends_on"`
	// Contract is optional; omitted means none.
	Contract *core.NodeContract `json:"contract"`
	core.NodeExtras
}

type patchJSON struct {
	Name           *string              `json:"name"`
	Type           *string              `json:"type"`
	Status         *string              `json:"status"`
	ParentID       *string              `json:"parent_id"`
	Implementation *[]string            `json:"implementation"`
	Scope          *[]string            `json:"scope"`
	Protected      *bool                `json:"protected"`
	Risk           *string              `json:"risk"`
	Review         *string              `json:"review"`
	Verify         *[]string            `json:"verify"`
	Markdown       *string              `json:"markdown"`
	Icon           *string              `json:"icon"`
	Color          *string              `json:"color"`
	Renderer       *string              `json:"renderer"`
	Schema         *[]core.SchemaColumn `json:"schema"`
	// DependsOn replaces the list when present ([] clears). Contract replaces when present ({} clears).
	DependsOn *[]string          `json:"depends_on"`
	Contract  *core.NodeContract `json:"contract"`
	core.NodeExtrasPatch
}

func postGroupProposal(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			NodeIDs []string `json:"node_ids"`
			Name    string   `json:"name"`
			Type    string   `json:"type"`
			Icon    string   `json:"icon"`
			Color   string   `json:"color"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeBadRequest(w, err)
			return
		}
		ids := make([]core.NodeID, len(body.NodeIDs))
		for i, id := range body.NodeIDs {
			ids[i] = core.NodeID(id)
		}
		pid, err := idx.StageGroup(core.GroupInput{
			NodeIDs: ids, Name: body.Name, Type: body.Type, Icon: body.Icon, Color: body.Color,
		})
		writeStagedProposal(w, idx, pid, err)
	}
}

func postUngroupProposal(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			NodeID          string `json:"node_id"`
			DeleteContainer bool   `json:"delete_container"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeBadRequest(w, err)
			return
		}
		pid, err := idx.StageUngroup(core.UngroupInput{
			NodeID: core.NodeID(body.NodeID), DeleteContainer: body.DeleteContainer,
		})
		writeStagedProposal(w, idx, pid, err)
	}
}

func postReparentProposal(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			NodeID      string `json:"node_id"`
			NewParentID string `json:"new_parent_id"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeBadRequest(w, err)
			return
		}
		pid, err := idx.StageReparent(core.NodeID(body.NodeID), core.NodeID(body.NewParentID))
		writeStagedProposal(w, idx, pid, err)
	}
}

// writeStagedProposal answers a staging call with 201 and the proposal diff, the same
// body GET /proposals/{id} returns.
func writeStagedProposal(w http.ResponseWriter, idx *core.Index, pid string, err error) {
	if err != nil {
		writeAPIError(w, err)
		return
	}
	diff, err := idx.ProposalDiff(pid)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, diff)
}

func toNodeIDs(in []string) []core.NodeID {
	if len(in) == 0 {
		return nil
	}
	out := make([]core.NodeID, len(in))
	for i, s := range in {
		out[i] = core.NodeID(s)
	}
	return out
}

func dependsOn(in []core.NodeID) []string {
	out := make([]string, len(in))
	for i, id := range in {
		out[i] = string(id)
	}
	return out
}

// nodeContractOrNil drops empty contracts so reads omit the key.
func nodeContractOrNil(c *core.NodeContract) *core.NodeContract {
	if c.IsEmpty() {
		return nil
	}
	return c
}

func contractOrNil(c *core.EdgeContract) *core.EdgeContract {
	if c.IsEmpty() {
		return nil
	}
	return c
}

func getConstraints(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"constraints": constraintBodies(idx.ConstraintList())})
	}
}

// putConstraints replaces the project constraints directly (the architect's own surface).
func putConstraints(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Constraints []core.Constraint `json:"constraints"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeBadRequest(w, err)
			return
		}
		if err := idx.SetConstraints(body.Constraints); err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"constraints": constraintBodies(idx.ConstraintList())})
	}
}

func constraintBodies(in []core.Constraint) []core.Constraint {
	if in == nil {
		return []core.Constraint{}
	}
	return in
}

func getBuildOrder(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		res, err := core.BuildOrderView(idx, core.BuildOrderOptions{
			Scope:     core.NodeID(q.Get("scope")),
			Compact:   queryBool(q.Get("compact")),
			ReadyOnly: queryBool(q.Get("ready_only")),
		})
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func queryBool(v string) bool {
	switch v {
	case "1", "true", "yes":
		return true
	}
	return false
}

// putGate sets whether the human gate at index of a node is cleared. The state lives in the
// gitignored local.json, so it is per user; there is deliberately no MCP equivalent.
func putGate(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			NodeID  string `json:"node_id"`
			Index   *int   `json:"index"`
			Cleared *bool  `json:"cleared"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeBadRequest(w, err)
			return
		}
		if body.NodeID == "" || body.Index == nil || body.Cleared == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node_id, index and cleared are required"})
			return
		}
		if err := idx.SetGateCleared(core.NodeID(body.NodeID), *body.Index, *body.Cleared); err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"node_id": body.NodeID, "index": *body.Index, "cleared": *body.Cleared})
	}
}
