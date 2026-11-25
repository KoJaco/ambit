package core

import (
	"fmt"
)

// SetMethodResult reports what SetMethod wrote.
type SetMethodResult struct {
	Method string `json:"method"`
	// Stage is the stage in force after the switch ("" for a stage-less method).
	Stage string `json:"stage"`
	// PreviousStage is the configured stage before the switch.
	PreviousStage string `json:"previous_stage,omitempty"`
	// StageReset is true when a previously set stage was not valid in the new method and
	// was replaced by its initial stage (or cleared for a stage-less method).
	StageReset bool `json:"stage_reset,omitempty"`
}

// SetMethod selects the active method in config.json. It changes policy only, never model
// data. The stage is kept when the new method has it, otherwise it is reset to the new
// method's initial stage and the result says so. Selecting freeform removes the key.
// This is a human-surface API; no agent tool calls it.
func (idx *Index) SetMethod(id string) (SetMethodResult, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	m := idx.lookupMethod(id)
	if m == nil {
		return SetMethodResult{}, fmt.Errorf("%w: %q", ErrUnknownMethod, id)
	}
	res := SetMethodResult{Method: id, PreviousStage: idx.StageID}
	kv := map[string]any{"method": id, "stage": nil}
	if id == FreeformMethod {
		kv["method"] = nil
	}
	switch {
	case len(m.Stages) == 0:
		res.StageReset = idx.StageID != ""
	case m.StageIndex(idx.StageID) >= 0:
		res.Stage = idx.StageID
	default:
		res.Stage = m.InitialStage
		res.StageReset = idx.StageID != ""
	}
	if res.Stage != "" {
		kv["stage"] = res.Stage
	}
	if err := idx.setConfigKeysLocked(kv); err != nil {
		return SetMethodResult{}, err
	}
	return res, nil
}

// SetStage sets the project stage in config.json. The stage must be one of the active
// method's stages. Only a human surface (UI, CLI, HTTP) calls this; no MCP tool does.
func (idx *Index) SetStage(id string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	m := idx.activeMethodLocked()
	if len(m.Stages) == 0 {
		return fmt.Errorf("%w: method %q has no stages", ErrStageUnsupported, m.ID)
	}
	if m.StageIndex(id) < 0 {
		ids := make([]string, len(m.Stages))
		for i, s := range m.Stages {
			ids[i] = s.ID
		}
		return fmt.Errorf("%w: %q is not a stage of method %q (stages: %v)", ErrUnknownStage, id, m.ID, ids)
	}
	return idx.setConfigKeysLocked(map[string]any{"stage": id})
}

// StageGate returns ErrStageGate when the active method refuses status in the current
// stage. It is meant for agent surfaces (MCP update_node_status); the human UI does not
// call it.
func (idx *Index) StageGate(status Status) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.stageGateLocked(status)
}

// UpdateStatusGated is the agent path for setting a node status. Under one lock it
// re-reads the model and config.json (method and stage) from disk, so a stage change made
// by another process or Index is honoured immediately, checks the gate, and writes the
// status. There is no window between the check and the write within this process. The gate
// binds callers of this function only; it is not a security boundary against a process
// that edits files or runs the CLI.
func (idx *Index) UpdateStatusGated(id NodeID, status Status) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if err := idx.reload(); err != nil {
		return err
	}
	if err := idx.stageGateLocked(status); err != nil {
		return err
	}
	if err := idx.completionGateLocked(id, status); err != nil {
		return err
	}
	return idx.updateLocked(id, UpdateInput{Status: &status})
}

func (idx *Index) stageGateLocked(status Status) error {
	m := idx.activeMethodLocked()
	stage := idx.effectiveStageLocked()
	if stage == "" {
		return nil
	}
	for _, s := range m.Gates.Status[stage] {
		if s == string(status) {
			msg := fmt.Sprintf("status %q is not allowed while the project stage is %q (method %q); ", status, stage, m.ID)
			if i := m.StageIndex(stage); i >= 0 && i+1 < len(m.Stages) {
				msg += fmt.Sprintf("a human must advance the stage to %q first", m.Stages[i+1].ID)
			} else {
				msg += "a human must advance the stage first"
			}
			return fmt.Errorf("%w: %s", ErrStageGate, msg)
		}
	}
	return nil
}

// completionGateLocked applies the method's completion policy to an agent setting
// status. Humans use the UI or HTTP and are not gated.
func (idx *Index) completionGateLocked(id NodeID, status Status) error {
	if status != StatusDone && status != StatusNeedsReview {
		return nil
	}
	m := idx.activeMethodLocked()
	n := idx.Nodes[id]
	if n == nil {
		return nil
	}
	if !m.Completion.AgentMayComplete {
		return fmt.Errorf("%w: method %q does not let agents mark a node %s; report your result to the architect, who sets the status", ErrCompletionGate, m.ID, status)
	}
	if status == StatusDone && reviewRequiredFor(m, n) {
		return fmt.Errorf("%w: node %q needs review before it is done; set needs_review instead and the architect will mark it done", ErrCompletionGate, id)
	}
	return nil
}
