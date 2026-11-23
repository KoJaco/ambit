package mcp

import (
	"context"

	"github.com/KoJaco/ambit/internal/core"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type methodRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type stageRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func methodRefOf(m *core.Method) methodRef { return methodRef{ID: m.ID, Name: m.Name} }

// stageRefOf returns nil for a stage-less method (JSON null).
func stageRefOf(m *core.Method, id string) *stageRef {
	if id == "" {
		return nil
	}
	if i := m.StageIndex(id); i >= 0 {
		return &stageRef{ID: id, Name: m.Stages[i].Name}
	}
	return &stageRef{ID: id, Name: id}
}

type getStartedIn struct{}

type getStartedOut struct {
	Method    methodRef  `json:"method"`
	Stage     *stageRef  `json:"stage"`
	Guidance  string     `json:"guidance"`
	Stages    []stageRef `json:"stages"`
	NextSteps []string   `json:"next_steps"`
}

func (e *toolEnv) handleGetStarted(_ context.Context, _ *sdkmcp.CallToolRequest, _ getStartedIn) (*sdkmcp.CallToolResult, getStartedOut, error) {
	m := e.idx.ActiveMethod()
	stage := e.idx.EffectiveStage()
	out := getStartedOut{
		Method:    methodRefOf(m),
		Stage:     stageRefOf(m, stage),
		Guidance:  m.Guidance,
		Stages:    []stageRef{},
		NextSteps: nextSteps(m, stage),
	}
	for _, s := range m.Stages {
		out.Stages = append(out.Stages, stageRef{ID: s.ID, Name: s.Name})
	}
	return nil, out, nil
}

// nextSteps composes short pointers from the method and stage. It is deliberately small:
// the method guidance carries the real workflow text.
func nextSteps(m *core.Method, stage string) []string {
	steps := []string{"Call get_graph to read the current model before authoring."}
	switch m.ID {
	case "design-first":
		if stage == "design" {
			steps = append(steps,
				"Stage is design: model the system with seed_model, create_node and set_relationship; the architect accepts proposals in the UI.",
				"Fill specs, implementation globs and contracts on leaf nodes, then call check_model and fix enforced findings.",
				"update_node_status refuses assigned, in_progress and done until a human advances the stage to build.")
		} else {
			steps = append(steps,
				"Stage is build: call get_build_order, pick a node whose blocked_by is empty, then get_context and check_scope before editing code.",
				"Use update_node_status to move nodes through assigned, in_progress and done.")
		}
	case "map-existing":
		steps = append(steps,
			"Read the repository layout first, then seed_model with shallow nodes whose implementation globs match real paths.",
			"Call check_drift and check_model to find unowned files and overlapping globs. Do not change application code.")
	default:
		steps = append(steps,
			"Use get_build_order and get_context before building a node.",
			"check_model is available on demand; this method enforces nothing.")
	}
	if len(m.Stages) > 0 {
		steps = append(steps, "The stage is set by a human only; no tool changes it.")
	}
	return steps
}

type checkModelIn struct {
	IncludeDrift *bool `json:"include_drift,omitempty"`
	AllRules     bool  `json:"all_rules,omitempty"`
}

func (e *toolEnv) handleCheckModel(_ context.Context, _ *sdkmcp.CallToolRequest, in checkModelIn) (*sdkmcp.CallToolResult, core.LintResult, error) {
	opts := core.CheckModelOptions{AllRules: in.AllRules}
	if in.IncludeDrift != nil {
		opts.IncludeDrift = *in.IncludeDrift
	} else {
		opts.IncludeDrift = e.idx.ActiveMethod().Check.IncludeDrift
	}
	res, err := core.CheckModel(e.idx, opts)
	if err != nil {
		return nil, core.LintResult{}, err
	}
	return nil, res, nil
}

const getStartedDescription = "Start here. Returns the active workflow method ({id, name}), the project stage ({id, name} or null), the method guidance, the method's stages and short next_steps for this method and stage. Read-only: the method and stage are chosen by a human and no tool changes them."

const checkModelDescription = "Lint the model against the active method's rules. Returns {ok, method, stage, findings[], counts, drift?}. Findings carry rule, severity (advisory|enforced), node_id or relationship_id and a message; ok is false only when an enforced finding exists. Optional include_drift (default: the method's setting) adds check_drift output; all_rules runs every rule as advisory. Read-only."
