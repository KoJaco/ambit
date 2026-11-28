package mcp

import (
	"context"
	"fmt"
	"strings"

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

// nextSteps composes short pointers from the method data (gates, lint severities,
// stages, check settings) and the current stage, never from built-in method ids. It is
// deliberately small: the method guidance carries the real workflow text.
func nextSteps(m *core.Method, stage string) []string {
	steps := []string{"Call get_graph to read the current model before authoring."}
	refused := m.Gates.Status[stage]
	if stage != "" {
		line := fmt.Sprintf("Stage is %s.", stage)
		if len(refused) > 0 {
			next := "a human advances the stage"
			if i := m.StageIndex(stage); i >= 0 && i+1 < len(m.Stages) {
				next = fmt.Sprintf("a human advances the stage to %s", m.Stages[i+1].ID)
			}
			line += fmt.Sprintf(" update_node_status refuses %s until %s.", strings.Join(refused, ", "), next)
		}
		steps = append(steps, line)
	}
	if len(refused) > 0 {
		steps = append(steps, "Model the system with seed_model, create_node and set_relationship; the architect accepts proposals in the UI.")
	} else {
		steps = append(steps,
			"Call get_build_order, pick a node whose blocked_by is empty, then get_context and check_scope before editing code.",
			"Use update_node_status to move nodes through assigned and in_progress. "+completionStep(m)+" Call check_done with your node_id (and worktree, if you work in one) before you finish.")
	}
	var enforced, advisory []string
	for _, id := range core.LintRuleIDs() {
		switch m.Lint[id].Severity {
		case core.LintEnforced:
			enforced = append(enforced, id)
		case core.LintAdvisory:
			advisory = append(advisory, id)
		}
	}
	switch {
	case len(enforced) > 0:
		steps = append(steps, fmt.Sprintf("Call check_model and fix enforced findings (rules: %s).", strings.Join(enforced, ", ")))
	case len(advisory) > 0:
		steps = append(steps, fmt.Sprintf("Call check_model; findings are advisory (rules: %s).", strings.Join(advisory, ", ")))
	default:
		steps = append(steps, "check_model is available on demand; this method enforces nothing.")
	}
	if m.Check.IncludeDrift {
		steps = append(steps, "check_model includes check_drift output by default for this method; use it to find unowned files and overlapping globs. Drift reads files git can see (tracked, plus untracked and not gitignored); pass node_id to limit it to one node and worktree to read a git worktree. Files no node should own (generated, lockfiles, tooling) are excluded with globs in the drift_ignore array of .arch/config.json (or `ambit config drift-ignore add <glob>`).")
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
	opts := core.CheckModelOptions{AllRules: in.AllRules, IncludeDrift: in.IncludeDrift}
	res, err := core.CheckModel(e.idx, opts)
	if err != nil {
		return nil, core.LintResult{}, err
	}
	return nil, res, nil
}

const getStartedDescription = "Start here. Returns the active workflow method ({id, name}), the project stage ({id, name} or null), the method guidance, the method's stages and short next_steps for this method and stage. Read-only: the method and stage are chosen by a human and no tool changes them."

const checkModelDescription = "Lint the model against the active method's rules. Returns {ok, method, stage, findings[], counts, drift?}. Findings carry rule, severity (advisory|enforced), node_id or relationship_id and a message; ok is false only when an enforced finding exists. Optional include_drift (default: the method's setting) adds check_drift output (with a hint to use drift_ignore when many files are unowned); all_rules runs every rule as advisory. Read-only."

// completionStep states, from the method's completion policy, how an agent finishes.
func completionStep(m *core.Method) string {
	switch {
	case !m.Completion.AgentMayComplete:
		return "Agents do not mark nodes finished under this method: report your result to the architect, who sets the status."
	case m.Completion.ReviewRequired:
		return "When finished, set needs_review (not done); the architect marks it done after review."
	default:
		return "When finished, set done."
	}
}
