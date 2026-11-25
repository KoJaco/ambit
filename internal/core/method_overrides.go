package core

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

// methodOverrides is the config.json "method_overrides" object: settings applied on top of
// whichever method is active, so a project can change one parameter (for example
// max-depth) without writing a method file (decision 0015).
type methodOverrides struct {
	Lint       map[string]LintSetting `json:"lint"`
	Completion *struct {
		AgentMayComplete *bool `json:"agent_may_complete"`
		ReviewRequired   *bool `json:"review_required"`
	} `json:"completion"`
	Check *struct {
		IncludeDrift *bool `json:"include_drift"`
	} `json:"check"`
}

// applyMethodOverrides returns m with config.json method_overrides applied. m itself is
// never modified (built-in methods are shared). Invalid input warns and is ignored.
func (idx *Index) applyMethodOverrides(m *Method) *Method {
	raw, ok := idx.configUnknown["method_overrides"]
	if !ok || isNull(raw) {
		return m
	}
	path := filepath.Join(idx.arch(), "config.json")
	var ov methodOverrides
	if err := json.Unmarshal(raw, &ov); err != nil {
		idx.warn(path, fmt.Sprintf("method_overrides ignored: %v", err))
		return m
	}
	c := *m
	c.Lint = make(map[string]LintSetting, len(m.Lint)+len(ov.Lint))
	for k, v := range m.Lint {
		c.Lint[k] = v
	}
	for rule, set := range ov.Lint {
		// An override changes what it names: a bare severity keeps the base params, and an
		// object without a severity keeps the base severity.
		base := c.Lint[rule]
		if set.Severity == "" {
			set.Severity = base.Severity
			if set.Severity == "" {
				set.Severity = LintOff
			}
		}
		if set.Params == nil {
			set.Params = base.Params
		} else if base.Params != nil {
			merged := make(map[string]any, len(base.Params)+len(set.Params))
			for k, v := range base.Params {
				merged[k] = v
			}
			for k, v := range set.Params {
				merged[k] = v
			}
			set.Params = merged
		}
		if err := set.validate(rule); err != nil {
			idx.warn(path, fmt.Sprintf("method_overrides.lint.%s ignored: %v", rule, err))
			continue
		}
		if !isLintRule(rule) {
			idx.warn(path, fmt.Sprintf("method_overrides.lint: rule %q is unknown and is ignored", rule))
			continue
		}
		c.Lint[rule] = set
	}
	if ov.Completion != nil {
		if ov.Completion.AgentMayComplete != nil {
			c.Completion.AgentMayComplete = *ov.Completion.AgentMayComplete
		}
		if ov.Completion.ReviewRequired != nil {
			c.Completion.ReviewRequired = *ov.Completion.ReviewRequired
		}
	}
	if ov.Check != nil && ov.Check.IncludeDrift != nil {
		c.Check.IncludeDrift = *ov.Check.IncludeDrift
	}
	return &c
}
