package httpapi

import (
	"net/http"

	"github.com/KoJaco/ambit/internal/core"
)

type methodRefJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type stageRefJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type stageDefJSON struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Preamble string `json:"preamble,omitempty"`
}

type ruleJSON struct {
	ID       string         `json:"id"`
	Severity string         `json:"severity"`
	Params   map[string]any `json:"params,omitempty"`
}

type methodJSON struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	Description   string              `json:"description"`
	Source        string              `json:"source"`
	Shadows       bool                `json:"shadows_builtin,omitempty"`
	Extends       string              `json:"extends,omitempty"`
	Guidance      string              `json:"guidance"`
	BriefPreamble string              `json:"brief_preamble"`
	Stages        []stageDefJSON      `json:"stages"`
	InitialStage  string              `json:"initial_stage"`
	Stage         *stageRefJSON       `json:"stage"`
	Gates         map[string][]string `json:"gates"`
	Rules         []ruleJSON          `json:"rules"`
	CheckDrift    bool                `json:"include_drift"`
}

type stageStateJSON struct {
	Method methodRefJSON  `json:"method"`
	Stage  *stageRefJSON  `json:"stage"`
	Stages []stageDefJSON `json:"stages"`
}

func stageDefs(m *core.Method) []stageDefJSON {
	out := make([]stageDefJSON, 0, len(m.Stages))
	for _, s := range m.Stages {
		out = append(out, stageDefJSON{ID: s.ID, Name: s.Name, Preamble: s.Preamble})
	}
	return out
}

func stageRef(m *core.Method, id string) *stageRefJSON {
	if id == "" {
		return nil
	}
	if i := m.StageIndex(id); i >= 0 {
		return &stageRefJSON{ID: id, Name: m.Stages[i].Name}
	}
	return &stageRefJSON{ID: id, Name: id}
}

func methodBody(idx *core.Index) methodJSON {
	m := idx.ActiveMethod()
	out := methodJSON{
		ID: m.ID, Name: m.Name, Description: m.Description, Source: m.Source, Shadows: m.Shadows,
		Extends: m.Extends, Guidance: m.Guidance, BriefPreamble: m.BriefPreamble,
		Stages: stageDefs(m), InitialStage: m.InitialStage, Stage: stageRef(m, idx.EffectiveStage()),
		Gates: map[string][]string{}, Rules: []ruleJSON{}, CheckDrift: m.Check.IncludeDrift,
	}
	for k, v := range m.Gates.Status {
		out.Gates[k] = append([]string{}, v...)
	}
	for _, id := range core.LintRuleIDs() {
		set := m.Lint[id]
		if set.Severity == "" {
			set.Severity = core.LintOff
		}
		out.Rules = append(out.Rules, ruleJSON{ID: id, Severity: set.Severity, Params: set.Params})
	}
	return out
}

func stageBody(idx *core.Index) stageStateJSON {
	m := idx.ActiveMethod()
	return stageStateJSON{
		Method: methodRefJSON{ID: m.ID, Name: m.Name},
		Stage:  stageRef(m, idx.EffectiveStage()),
		Stages: stageDefs(m),
	}
}

func getMethod(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, methodBody(idx))
	}
}

func getMethods(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list := idx.ListMethods()
		if list == nil {
			list = []core.MethodSummary{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"methods": list})
	}
}

type idBody struct {
	ID string `json:"id"`
}

func putMethod(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body idBody
		if err := decodeJSON(r, &body); err != nil {
			writeBadRequest(w, err)
			return
		}
		res, err := idx.SetMethod(body.ID)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func getStage(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, stageBody(idx))
	}
}

func putStage(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body idBody
		if err := decodeJSON(r, &body); err != nil {
			writeBadRequest(w, err)
			return
		}
		if err := idx.SetStage(body.ID); err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, stageBody(idx))
	}
}

func queryFlag(r *http.Request, key string) bool {
	switch r.URL.Query().Get(key) {
	case "1", "true", "yes":
		return true
	}
	return false
}

func getCheckModel(idx *core.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := core.CheckModel(idx, core.CheckModelOptions{
			IncludeDrift: queryFlag(r, "include_drift"),
			AllRules:     queryFlag(r, "all"),
		})
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}
