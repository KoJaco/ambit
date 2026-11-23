package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Lint severities a method can assign to a rule.
const (
	LintOff      = "off"
	LintAdvisory = "advisory"
	LintEnforced = "enforced"
)

// Lint rule ids (decision 0013).
const (
	RuleLeafImplementation = "leaf-implementation"
	RuleLeafContract       = "leaf-contract"
	RuleSpecPresent        = "spec-present"
	RuleIconSet            = "icon-set"
	RuleEdgeLabel          = "edge-label"
	RuleEdgeKind           = "edge-kind"
	RuleMaxDepth           = "max-depth"
	RuleGlobOverlap        = "glob-overlap"
	RuleOrphanNode         = "orphan-node"
	RuleDependencyCycle    = "dependency-cycle"

	// DefaultMaxDepth is the max-depth rule default (root nodes are depth 0).
	DefaultMaxDepth = 2
)

// LintSetting is a method's setting for one rule: a severity string, or an object
// {"severity", "params"}.
type LintSetting struct {
	Severity string         `json:"severity"`
	Params   map[string]any `json:"params,omitempty"`
}

func (s *LintSetting) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '"' {
		return json.Unmarshal(data, &s.Severity)
	}
	type plain LintSetting
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("%w: lint value must be a severity string or {severity, params}", ErrBadLintRule)
	}
	*s = LintSetting(p)
	return nil
}

func (s LintSetting) MarshalJSON() ([]byte, error) {
	if len(s.Params) == 0 {
		return marshalNoEscape(s.Severity)
	}
	type plain LintSetting
	return marshalNoEscape(plain(s))
}

func (s LintSetting) validate(rule string) error {
	switch s.Severity {
	case LintOff, LintAdvisory, LintEnforced:
	default:
		return fmt.Errorf("%w: %w: %s: severity %q must be off, advisory or enforced", ErrBadMethod, ErrBadLintRule, rule, s.Severity)
	}
	if v, ok := s.Params["max_depth"]; ok && rule == RuleMaxDepth {
		if f, isNum := v.(float64); !isNum || f != float64(int(f)) || f < 0 {
			return fmt.Errorf("%w: %w: %s: max_depth must be a non-negative integer", ErrBadMethod, ErrBadLintRule, rule)
		}
	}
	return nil
}

func (s LintSetting) intParam(key string, def int) int {
	if f, ok := s.Params[key].(float64); ok {
		return int(f)
	}
	return def
}

// LintFinding is one model lint result.
type LintFinding struct {
	Rule           string `json:"rule"`
	Severity       string `json:"severity"`
	NodeID         NodeID `json:"node_id,omitempty"`
	RelationshipID RelID  `json:"relationship_id,omitempty"`
	Message        string `json:"message"`
}

// LintCounts totals findings by severity.
type LintCounts struct {
	Advisory int `json:"advisory"`
	Enforced int `json:"enforced"`
	Total    int `json:"total"`
}

// CheckModelOptions selects what CheckModel runs.
type CheckModelOptions struct {
	// IncludeDrift also runs Drift against idx.Dir (the method's check.include_drift
	// turns it on as well).
	IncludeDrift bool
	// AllRules runs every rule as advisory, ignoring the method's severities.
	AllRules bool
}

// LintResult is the check_model result. OK is false only when an enforced finding exists.
type LintResult struct {
	Method   string        `json:"method"`
	Stage    string        `json:"stage,omitempty"`
	OK       bool          `json:"ok"`
	Findings []LintFinding `json:"findings"`
	Counts   LintCounts    `json:"counts"`
	Drift    *DriftReport  `json:"drift,omitempty"`
	// DriftError is set when drift was requested but could not run (for example no git).
	DriftError string `json:"drift_error,omitempty"`
}

type lintRule struct {
	id  string
	run func(idx *Index, set LintSetting, emit func(LintFinding))
}

// lintRules is the fixed registry, in report order.
var lintRules = []lintRule{
	{RuleLeafImplementation, lintLeafImplementation},
	{RuleLeafContract, lintLeafContract},
	{RuleSpecPresent, lintSpecPresent},
	{RuleIconSet, lintIconSet},
	{RuleEdgeLabel, lintEdgeLabel},
	{RuleEdgeKind, lintEdgeKind},
	{RuleMaxDepth, lintMaxDepth},
	{RuleGlobOverlap, lintGlobOverlap},
	{RuleOrphanNode, lintOrphanNode},
	{RuleDependencyCycle, lintDependencyCycle},
}

// LintRuleIDs lists the rule ids CheckModel knows, in report order.
func LintRuleIDs() []string {
	out := make([]string, len(lintRules))
	for i, r := range lintRules {
		out[i] = r.id
	}
	return out
}

func isLintRule(id string) bool {
	for _, r := range lintRules {
		if r.id == id {
			return true
		}
	}
	return false
}

// CheckModel lints the model under the active method. Findings are ordered by rule, then
// model order. It never mutates anything. Drift failures are reported in DriftError.
func CheckModel(idx *Index, opts CheckModelOptions) (LintResult, error) {
	if idx == nil {
		return LintResult{}, fmt.Errorf("no index")
	}
	idx.mu.Lock()
	m := idx.activeMethodLocked()
	res := LintResult{Method: m.ID, Stage: idx.effectiveStageLocked(), Findings: []LintFinding{}}
	for _, r := range lintRules {
		set := m.Lint[r.id]
		if opts.AllRules {
			set.Severity = LintAdvisory
		}
		if set.Severity != LintAdvisory && set.Severity != LintEnforced {
			continue
		}
		sev := set.Severity
		r.run(idx, set, func(f LintFinding) {
			f.Rule, f.Severity = r.id, sev
			res.Findings = append(res.Findings, f)
		})
	}
	root := idx.Dir
	wantDrift := opts.IncludeDrift || m.Check.IncludeDrift
	idx.mu.Unlock()

	for _, f := range res.Findings {
		if f.Severity == LintEnforced {
			res.Counts.Enforced++
		} else {
			res.Counts.Advisory++
		}
	}
	res.Counts.Total = len(res.Findings)
	res.OK = res.Counts.Enforced == 0
	if wantDrift {
		d, err := Drift(idx, root)
		if err != nil {
			res.DriftError = err.Error()
		} else {
			res.Drift = &d
		}
	}
	return res, nil
}

// exemptFromLeafRules reports the node types that carry no implementation or contract.
func exemptFromLeafRules(n *Node) bool {
	switch strings.ToLower(n.Type) {
	case "note", "external":
		return true
	}
	return false
}

func lintLeafImplementation(idx *Index, _ LintSetting, emit func(LintFinding)) {
	for _, id := range idx.Order {
		n := idx.Nodes[id]
		if n == nil || exemptFromLeafRules(n) || len(idx.Children[id]) > 0 {
			continue
		}
		if len(n.Implementation) == 0 {
			emit(LintFinding{NodeID: id, Message: fmt.Sprintf("leaf node %s has no implementation globs", id)})
		}
	}
}

func lintLeafContract(idx *Index, _ LintSetting, emit func(LintFinding)) {
	for _, id := range idx.Order {
		n := idx.Nodes[id]
		if n == nil || exemptFromLeafRules(n) || len(idx.Children[id]) > 0 {
			continue
		}
		if n.Contract.IsEmpty() {
			emit(LintFinding{NodeID: id, Message: fmt.Sprintf("leaf node %s has no contract", id)})
		}
	}
}

func lintSpecPresent(idx *Index, _ LintSetting, emit func(LintFinding)) {
	for _, id := range idx.Order {
		if n := idx.Nodes[id]; n != nil && strings.TrimSpace(n.Spec) == "" {
			emit(LintFinding{NodeID: id, Message: fmt.Sprintf("node %s has no spec", id)})
		}
	}
}

func lintIconSet(idx *Index, _ LintSetting, emit func(LintFinding)) {
	for _, id := range idx.Order {
		if n := idx.Nodes[id]; n != nil && n.Icon == "" {
			emit(LintFinding{NodeID: id, Message: fmt.Sprintf("node %s has no icon", id)})
		}
	}
}

func lintEdgeLabel(idx *Index, _ LintSetting, emit func(LintFinding)) {
	for _, r := range idx.Relationships {
		if strings.TrimSpace(r.Label) == "" {
			emit(LintFinding{RelationshipID: r.ID, Message: fmt.Sprintf("relationship %s -> %s has no label", r.From, r.To)})
		}
	}
}

func lintEdgeKind(idx *Index, _ LintSetting, emit func(LintFinding)) {
	for _, r := range idx.Relationships {
		if r.Kind == "" {
			emit(LintFinding{RelationshipID: r.ID, Message: fmt.Sprintf("relationship %s -> %s has no kind", r.From, r.To)})
		}
	}
}

func lintMaxDepth(idx *Index, set LintSetting, emit func(LintFinding)) {
	limit := set.intParam("max_depth", DefaultMaxDepth)
	for _, id := range idx.Order {
		n := idx.Nodes[id]
		if n == nil {
			continue
		}
		depth, seen := 0, map[NodeID]bool{id: true}
		for p := n.ParentID; p != "" && !seen[p]; {
			seen[p] = true
			depth++
			parent := idx.Nodes[p]
			if parent == nil {
				break
			}
			p = parent.ParentID
		}
		if depth > limit {
			emit(LintFinding{NodeID: id, Message: fmt.Sprintf("node %s is nested %d levels deep (max_depth %d)", id, depth, limit)})
		}
	}
}

func lintGlobOverlap(idx *Index, _ LintSetting, emit func(LintFinding)) {
	reported := map[string]bool{}
	for _, id := range idx.Order {
		n := idx.Nodes[id]
		if n == nil {
			continue
		}
		for _, g := range n.Implementation {
			sample := globSample(g)
			if sample == "" {
				continue
			}
			owners, err := ownersOfPath(idx, sample)
			if err != nil || len(owners) < 2 || !hasNonNestedPair(idx, owners) {
				continue
			}
			ids := make([]string, len(owners))
			for i, o := range owners {
				ids[i] = string(o)
			}
			key := strings.Join(ids, ",")
			if reported[key] {
				continue
			}
			reported[key] = true
			emit(LintFinding{NodeID: owners[0], Message: fmt.Sprintf("implementation globs of %s overlap (for example %s)", strings.Join(ids, ", "), sample)})
		}
	}
}

// globSample builds one concrete path the glob matches, or "" when it cannot. Wildcards
// become "x", a brace group keeps its first alternative.
func globSample(g string) string {
	g = toSlash(strings.TrimSpace(g))
	if g == "" {
		return ""
	}
	var b strings.Builder
	for i := 0; i < len(g); i++ {
		switch c := g[i]; c {
		case '*', '?':
			for i+1 < len(g) && g[i+1] == '*' {
				i++
			}
			b.WriteByte('x')
		case '{':
			end := strings.IndexByte(g[i:], '}')
			if end < 0 {
				return ""
			}
			alt := g[i+1 : i+end]
			if k := strings.IndexByte(alt, ','); k >= 0 {
				alt = alt[:k]
			}
			b.WriteString(globSample(alt))
			i += end
		case '[':
			end := strings.IndexByte(g[i:], ']')
			if end < 0 {
				return ""
			}
			class := g[i+1 : i+end]
			if class == "" || class[0] == '!' || class[0] == '^' {
				b.WriteByte('x')
			} else {
				b.WriteByte(class[0])
			}
			i += end
		case '\\':
			if i+1 < len(g) {
				i++
				b.WriteByte(g[i])
			}
		default:
			b.WriteByte(c)
		}
	}
	s := b.String()
	if ok, err := doublestar.Match(g, s); err != nil || !ok {
		return ""
	}
	return s
}

func lintOrphanNode(idx *Index, _ LintSetting, emit func(LintFinding)) {
	if len(idx.Order) < 2 {
		return
	}
	linked := map[NodeID]bool{}
	for _, r := range idx.Relationships {
		linked[r.From], linked[r.To] = true, true
	}
	for _, id := range idx.Order {
		n := idx.Nodes[id]
		if n == nil {
			continue
		}
		for _, d := range n.DependsOn {
			linked[id], linked[d] = true, true
		}
		if n.ParentID != "" {
			linked[id], linked[n.ParentID] = true, true
		}
		if n.RelationshipID != "" {
			linked[id] = true
		}
	}
	for _, id := range idx.Order {
		if idx.Nodes[id] != nil && !linked[id] {
			emit(LintFinding{NodeID: id, Message: fmt.Sprintf("node %s has no parent, children, relationships or dependencies", id)})
		}
	}
}

func lintDependencyCycle(idx *Index, _ LintSetting, emit func(LintFinding)) {
	res, err := idx.buildOrderLocked("")
	if err != nil {
		return
	}
	for _, cycle := range res.Cycles {
		if len(cycle) == 0 {
			continue
		}
		ids := make([]string, len(cycle))
		for i, c := range cycle {
			ids[i] = string(c)
		}
		emit(LintFinding{NodeID: cycle[0], Message: fmt.Sprintf("dependency cycle among %s", strings.Join(ids, ", "))})
	}
}
