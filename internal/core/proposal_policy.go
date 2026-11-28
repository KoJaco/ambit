package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Architect auto-accept (decision 0017). A human turns the policy on in config.json
// ("proposal_policy": {"auto_accept": true}); it has effect only while the project stage
// is build. The classifier below decides, per proposal operation, whether the operation is
// low-risk enough for an architect agent to accept. It is pure: it reads a model view and
// the operations and never touches the disk, so every rule is table-testable.

// ErrPolicyRefused is returned when auto-accept may not run at all (policy off, wrong
// stage, self-accept). A proposal that merely contains operations the policy does not
// cover is not an error: the result lists the blocking operations.
var ErrPolicyRefused = errors.New("auto-accept refused")

// ErrBadPolicy is returned for an invalid proposal policy value.
var ErrBadPolicy = errors.New("invalid proposal policy")

// ProposalPolicy is config.json "proposal_policy".
type ProposalPolicy struct {
	AutoAccept bool `json:"auto_accept"`
}

// PolicyInfo is the policy plus whether it is in effect right now.
type PolicyInfo struct {
	AutoAccept bool   `json:"auto_accept"`
	Stage      string `json:"stage"`
	// Effective is true only when AutoAccept is on and the stage is build.
	Effective bool   `json:"effective"`
	Reason    string `json:"reason,omitempty"`
}

// BuildStage is the only stage in which auto-accept is effective.
const BuildStage = "build"

func (idx *Index) parseProposalPolicy(obj map[string]jsonRaw) ProposalPolicy {
	raw, ok := obj[configKeyPolicy]
	if !ok || isNull(raw) {
		return ProposalPolicy{}
	}
	var p ProposalPolicy
	if err := json.Unmarshal(raw, &p); err != nil {
		idx.warn("config.json", fmt.Sprintf("%s must be an object like {\"auto_accept\": true}; ignoring", configKeyPolicy))
		return ProposalPolicy{}
	}
	return p
}

// ProposalPolicyInfo returns the policy and whether it is effective.
func (idx *Index) ProposalPolicyInfo() PolicyInfo {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.policyInfoLocked()
}

func (idx *Index) policyInfoLocked() PolicyInfo {
	info := PolicyInfo{AutoAccept: idx.ProposalPolicy.AutoAccept, Stage: idx.effectiveStageLocked()}
	switch {
	case !info.AutoAccept:
		info.Reason = "auto_accept is off"
	case info.Stage != BuildStage:
		info.Reason = fmt.Sprintf("the project stage is %q; auto-accept is effective only in the %q stage", info.Stage, BuildStage)
	default:
		info.Effective = true
	}
	return info
}

// SetProposalPolicy writes the policy to config.json. Only a human surface (the HTTP API)
// calls it; no MCP tool does. Turning it off removes the key.
func (idx *Index) SetProposalPolicy(p ProposalPolicy) (PolicyInfo, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	var v any
	if p.AutoAccept {
		v = ProposalPolicy{AutoAccept: true}
	}
	if err := idx.setConfigKeysLocked(map[string]any{configKeyPolicy: v}); err != nil {
		return PolicyInfo{}, err
	}
	return idx.policyInfoLocked(), nil
}

// Policy rule ids. The allowing rules appear in the audit log; the blocking ones in the
// refusal list.
const (
	RuleSpecText         = "spec-text"
	RuleDependencyLoosen = "dependency-loosen"
	RuleDependencyAdd    = "dependency-add-acyclic"
	RuleScopeDeliverable = "scope-deliverable"
	RuleCreateContainer  = "create-under-container"
	RuleEdgeExisting     = "relationship-existing"

	BlockProtected   = "protected-node"
	BlockExternal    = "external"
	BlockSensitive   = "sensitive-field"
	BlockNotAuto     = "not-auto-eligible"
	BlockDelete      = "delete"
	BlockRename      = "rename"
	BlockConstraints = "constraints"
	BlockPart        = "part"
	BlockStale       = "stale"
	BlockLint        = "lint-enforced"
	BlockTrial       = "apply-check"
	BlockUnknown     = "unknown-op"
)

// PolicyVerdict is the classification of one operation.
type PolicyVerdict struct {
	Index int    `json:"index"`
	Op    string `json:"op"`
	Node  string `json:"node"`
	// Rule is the allowing rule(s) (comma separated) when Auto, else the blocking rule.
	Rule string `json:"rule"`
	Auto bool   `json:"auto"`
	// Reason says why the operation needs a human (empty when Auto).
	Reason  string `json:"reason,omitempty"`
	Summary string `json:"diff_summary"`
}

// PolicyOp is one proposal operation with the model state the classifier needs. Current
// is the node on disk (nil when it does not exist), Proposed the staged body for
// create_node and update_node.
type PolicyOp struct {
	Index    int
	Op       string
	NodeID   string
	NewID    string
	Fields   []string
	Current  *Node
	Proposed *Node
	// BodyHash fingerprints the staged bytes Proposed was decoded from.
	BodyHash string

	// Edge fields for set_relationship.
	From, To   NodeID
	RelID      string
	Label      string
	Kind       string
	CurrentRel *Relationship
	// Contract is the staged edge contract: nil means unchanged, an empty one clears it.
	Contract *EdgeContract

	// Stale and StaleReason are set by the caller from the proposal machinery.
	Stale       bool
	StaleReason string
}

// PolicyModel is the model state before the operations apply.
type PolicyModel struct {
	Nodes map[NodeID]*Node
	// Files is the repo's file list (tracked plus untracked, not ignored). Scope additions
	// are judged against it. FilesErr is set when the list could not be read; scope
	// additions then always need a human.
	Files    []string
	FilesErr string
}

// sensitiveFields names the node fields (JSON keys) that always need a human, on an
// update and on a create alike. It is the single list classifyUpdate and classifyCreate
// share. Every other field an update changes needs a human too unless a rule allows it.
var sensitiveFields = map[string]bool{
	"stage": true, "risk": true, "review": true, "human_gates": true, "shared": true,
	"protected": true, "verify": true, "gates": true, "external": true,
}

// nodeKeyValues flattens a node to its JSON keys (including unknown keys) plus "spec" and
// "type", as canonical strings, so two nodes can be diffed field by field.
func nodeKeyValues(n *Node) map[string]string {
	out := map[string]string{"spec": n.Spec}
	raw, err := encodeNode(n)
	if err != nil {
		out["<encode-error>"] = err.Error()
		return out
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		out["<encode-error>"] = err.Error()
		return out
	}
	for k, v := range obj {
		var buf strings.Builder
		var tmp any
		if json.Unmarshal(v, &tmp) == nil {
			if b, err := marshalNoEscape(tmp); err == nil {
				buf.Write(b)
			}
		}
		if buf.Len() == 0 {
			buf.WriteString(string(v))
		}
		out[k] = buf.String()
	}
	return out
}

// changedFields lists the keys whose value differs between cur and next, sorted. It is
// computed from the node bodies, never from what the operation declares.
func changedFields(cur, next *Node) []string {
	a, b := nodeKeyValues(cur), nodeKeyValues(next)
	var out []string
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			out = append(out, k)
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// presentSensitiveFields lists the sensitive keys a node body sets (non-empty).
func presentSensitiveFields(n *Node) []string {
	var out []string
	for k := range nodeKeyValues(n) {
		if sensitiveFields[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// ClassifyOps classifies every operation. The result is in operation order and has one
// verdict per input. A proposal is auto-acceptable only when every verdict is Auto.
func ClassifyOps(model PolicyModel, ops []PolicyOp) []PolicyVerdict {
	c := &classifier{model: model, ops: ops, created: map[NodeID]bool{}}
	c.final = c.finalDeps()
	out := make([]PolicyVerdict, 0, len(ops))
	for _, op := range ops {
		out = append(out, c.classify(op))
	}
	return out
}

// AllAuto reports whether every verdict is Auto (and there is at least one).
func AllAuto(vs []PolicyVerdict) bool {
	if len(vs) == 0 {
		return false
	}
	for _, v := range vs {
		if !v.Auto {
			return false
		}
	}
	return true
}

// Blocking returns the verdicts that need a human.
func Blocking(vs []PolicyVerdict) []PolicyVerdict {
	var out []PolicyVerdict
	for _, v := range vs {
		if !v.Auto {
			out = append(out, v)
		}
	}
	return out
}

type classifier struct {
	model PolicyModel
	ops   []PolicyOp
	// created holds nodes an earlier auto-accepted create in this proposal adds.
	created map[NodeID]bool
	// final is the depends_on graph after every create and update of the proposal.
	final map[NodeID][]NodeID
}

// finalDeps is the dependency graph after the proposal: disk deps, overridden by the
// staged body of every create and update.
func (c *classifier) finalDeps() map[NodeID][]NodeID {
	g := map[NodeID][]NodeID{}
	for id, n := range c.model.Nodes {
		g[id] = n.DependsOn
	}
	for _, op := range c.ops {
		if (op.Op == OpCreateNode || op.Op == OpUpdateNode) && op.Proposed != nil {
			g[op.Proposed.ID] = op.Proposed.DependsOn
		}
	}
	return g
}

// reaches reports whether to is reachable from from along depends_on in the final graph.
func (c *classifier) reaches(from, to NodeID) bool {
	seen := map[NodeID]bool{}
	var walk func(NodeID) bool
	walk = func(n NodeID) bool {
		if n == to {
			return true
		}
		if seen[n] {
			return false
		}
		seen[n] = true
		for _, d := range c.final[n] {
			if walk(d) {
				return true
			}
		}
		return false
	}
	return walk(from)
}

func (c *classifier) node(id NodeID) *Node {
	if n := c.model.Nodes[id]; n != nil {
		return n
	}
	for _, op := range c.ops {
		if op.Op == OpCreateNode && op.Proposed != nil && op.Proposed.ID == id && c.created[id] {
			return op.Proposed
		}
	}
	return nil
}

func blocked(op PolicyOp, rule, reason, summary string) PolicyVerdict {
	return PolicyVerdict{Index: op.Index, Op: op.Op, Node: op.NodeID, Rule: rule, Reason: reason, Summary: summary}
}

func allowed(op PolicyOp, rules []string, summary string) PolicyVerdict {
	set := map[string]bool{}
	var uniq []string
	for _, r := range rules {
		if !set[r] {
			set[r] = true
			uniq = append(uniq, r)
		}
	}
	sort.Strings(uniq)
	return PolicyVerdict{Index: op.Index, Op: op.Op, Node: op.NodeID, Rule: strings.Join(uniq, ","), Auto: true, Summary: summary}
}

func isExternalNode(n *Node) bool {
	return n != nil && (n.External || strings.EqualFold(strings.TrimSpace(n.Type), "external"))
}

func (c *classifier) classify(op PolicyOp) PolicyVerdict {
	summary := summarizeOp(op)
	if op.Stale {
		return blocked(op, BlockStale, "operation is stale: "+op.StaleReason, summary)
	}
	switch op.Op {
	case OpUpdateNode:
		return c.classifyUpdate(op, summary)
	case OpCreateNode:
		return c.classifyCreate(op, summary)
	case OpSetRelationship:
		return c.classifyEdge(op, summary)
	case OpDeleteNode, OpDeleteRelationship:
		return blocked(op, BlockDelete, "deletes always need a human", summary)
	case OpRenameNode:
		return blocked(op, BlockRename, "renames always need a human", summary)
	case OpSetConstraints:
		return blocked(op, BlockConstraints, "constraint changes always need a human", summary)
	case OpSetPart, OpDeletePart:
		return blocked(op, BlockPart, "part changes always need a human", summary)
	}
	return blocked(op, BlockUnknown, fmt.Sprintf("operation %q is not covered by the policy", op.Op), summary)
}

func (c *classifier) classifyUpdate(op PolicyOp, summary string) PolicyVerdict {
	cur, next := op.Current, op.Proposed
	if cur == nil || next == nil {
		return blocked(op, BlockNotAuto, "node to update is not in the model", summary)
	}
	if cur.Protected || next.Protected {
		return blocked(op, BlockProtected, fmt.Sprintf("node %s is protected", cur.ID), summary)
	}
	var rules []string
	// The declared op.Fields are not trusted: accept applies the whole staged body, so the
	// changed fields come from diffing the node on disk against it.
	for _, f := range changedFields(cur, next) {
		switch f {
		case "spec", "contract":
			rules = append(rules, RuleSpecText)
		case "depends_on":
			depRules, reason := c.checkDeps(cur, next)
			if reason != "" {
				return blocked(op, depRules[0], reason, summary)
			}
			rules = append(rules, depRules...)
		case "implementation", "scope":
			rule, reason := c.checkScope(f, cur, next, cur.Deliverables)
			if reason != "" {
				return blocked(op, rule, reason, summary)
			}
			rules = append(rules, RuleScopeDeliverable)
		default:
			if sensitiveFields[f] {
				return blocked(op, BlockSensitive, fmt.Sprintf("changing %s always needs a human", f), summary)
			}
			return blocked(op, BlockNotAuto, fmt.Sprintf("changing %s is not covered by the auto-accept policy", f), summary)
		}
	}
	if len(rules) == 0 {
		return blocked(op, BlockNotAuto, "update changes no field the policy recognises", summary)
	}
	return allowed(op, rules, summary)
}

// checkDeps classifies a depends_on change. It returns the allowing rules, or a blocking
// rule id (as the single element of the first result) with a reason.
func (c *classifier) checkDeps(cur, next *Node) ([]string, string) {
	have := map[NodeID]bool{}
	for _, d := range cur.DependsOn {
		have[d] = true
	}
	want := map[NodeID]bool{}
	for _, d := range next.DependsOn {
		want[d] = true
	}
	var added, removed []NodeID
	for d := range want {
		if !have[d] {
			added = append(added, d)
		}
	}
	for d := range have {
		if !want[d] {
			removed = append(removed, d)
		}
	}
	sortNodeIDs(added)
	sortNodeIDs(removed)
	for _, d := range append(append([]NodeID{}, added...), removed...) {
		if t := c.node(d); t != nil && t.Protected {
			return []string{BlockProtected}, fmt.Sprintf("dependency %s is a protected node", d)
		}
	}
	for _, d := range added {
		if isExternalNode(c.node(d)) {
			return []string{BlockExternal}, fmt.Sprintf("dependency %s is external", d)
		}
	}
	var rules []string
	if len(removed) > 0 {
		rules = append(rules, RuleDependencyLoosen)
	}
	if len(added) > 0 {
		for _, d := range added {
			if d == cur.ID || c.reaches(d, cur.ID) {
				return []string{BlockNotAuto}, fmt.Sprintf("adding dependency %s would create a cycle", d)
			}
		}
		rules = append(rules, RuleDependencyAdd)
	}
	if len(rules) == 0 {
		return []string{BlockNotAuto}, "depends_on is unchanged"
	}
	return rules, ""
}

// checkScope classifies an implementation or scope change. It returns a blocking rule and
// reason, or "" when the change is a pure addition of safe globs (see checkGlobAdditions).
func (c *classifier) checkScope(field string, cur, next *Node, deliverables []string) (string, string) {
	oldGlobs, newGlobs := cur.Implementation, next.Implementation
	if field == "scope" {
		oldGlobs, newGlobs = cur.Scope, next.Scope
	}
	owned := append(append([]string{}, cur.Implementation...), cur.Scope...)
	return c.checkGlobAdditions(field, cur.ID, oldGlobs, newGlobs, deliverables, owned, true)
}

// checkGlobAdditions allows only added globs that are narrow and safe:
//   - no recursive "**" wildcard;
//   - (needDeliverable) the glob matches at least one declared deliverable;
//   - every existing repo file the glob matches is a declared deliverable or is already
//     owned by this node (matched by owned globs), so the change claims no one else's files;
//   - the glob does not overlap a protected node's globs, judged against the real files and
//     glob against glob.
//
// It returns the blocking rule id and a reason, or "" "" when the additions are safe.
func (c *classifier) checkGlobAdditions(field string, self NodeID, oldGlobs, newGlobs, deliverables, owned []string, needDeliverable bool) (string, string) {
	have := map[string]bool{}
	for _, g := range oldGlobs {
		have[g] = true
	}
	want := map[string]bool{}
	for _, g := range newGlobs {
		want[g] = true
	}
	for g := range have {
		if !want[g] {
			return BlockNotAuto, fmt.Sprintf("%s: removing glob %q is not covered by the policy", field, g)
		}
	}
	for _, g := range newGlobs {
		if have[g] {
			continue
		}
		if other := c.protectedOverlap(self, g); other != "" {
			return BlockProtected, fmt.Sprintf("%s: glob %q overlaps protected node %s", field, g, other)
		}
		if strings.Contains(g, "**") {
			return BlockNotAuto, fmt.Sprintf("%s: glob %q is recursive (**); only narrow globs are covered", field, g)
		}
		if needDeliverable {
			ok := false
			for _, d := range deliverables {
				if m, err := doublestar.Match(toSlash(g), toSlash(d)); err == nil && m {
					ok = true
					break
				}
			}
			if !ok {
				return BlockNotAuto, fmt.Sprintf("%s: glob %q matches none of the node's deliverables", field, g)
			}
		}
		if c.model.FilesErr != "" {
			return BlockNotAuto, fmt.Sprintf("%s: the repo file list is unavailable (%s)", field, c.model.FilesErr)
		}
		for _, f := range c.model.Files {
			if m, err := doublestar.Match(toSlash(g), f); err != nil || !m {
				continue
			}
			if matchesAny(deliverables, f) || matchesAny(owned, f) {
				continue
			}
			return BlockNotAuto, fmt.Sprintf("%s: glob %q matches existing file %s, which is neither a deliverable nor already owned by this node", field, g, f)
		}
	}
	return "", ""
}

// matchesAny reports whether path equals or matches any of the patterns.
func matchesAny(patterns []string, path string) bool {
	for _, p := range patterns {
		p = toSlash(p)
		if p == path {
			return true
		}
		if m, err := doublestar.Match(p, path); err == nil && m {
			return true
		}
	}
	return false
}

// protectedOverlap returns a protected node (other than self) whose globs overlap glob.
// Overlap is judged three ways: glob against glob by segments, by sample paths, and by
// the real repo files (a file both globs match).
func (c *classifier) protectedOverlap(self NodeID, glob string) NodeID {
	ids := make([]NodeID, 0, len(c.model.Nodes))
	for id := range c.model.Nodes {
		ids = append(ids, id)
	}
	sortNodeIDs(ids)
	gs := globSample(glob)
	for _, id := range ids {
		n := c.model.Nodes[id]
		if n == nil || !n.Protected || id == self {
			continue
		}
		for _, pg := range append(append([]string{}, n.Implementation...), n.Scope...) {
			if globsOverlap(glob, gs, pg) {
				return id
			}
			for _, f := range c.model.Files {
				m1, e1 := doublestar.Match(toSlash(glob), f)
				m2, e2 := doublestar.Match(toSlash(pg), f)
				if e1 == nil && e2 == nil && m1 && m2 {
					return id
				}
			}
		}
	}
	return ""
}

// globsOverlap reports whether two globs might match a common path. It errs on the side
// of reporting an overlap: equal globs, a sample path of either matching the other, or a
// segment-wise comparison (globsSegmentsOverlap) that treats any wildcard-against-wildcard
// segment and anything after "**" as overlapping.
func globsOverlap(a, aSample, b string) bool {
	if a == b {
		return true
	}
	if aSample != "" {
		if m, err := doublestar.Match(toSlash(b), aSample); err == nil && m {
			return true
		}
	}
	if bs := globSample(b); bs != "" {
		if m, err := doublestar.Match(toSlash(a), bs); err == nil && m {
			return true
		}
	}
	return globsSegmentsOverlap(strings.Split(toSlash(a), "/"), strings.Split(toSlash(b), "/"))
}

func hasGlobMeta(s string) bool { return strings.ContainsAny(s, "*?[{") }

func onlyDoubleStars(segs []string) bool {
	for _, s := range segs {
		if s != "**" {
			return false
		}
	}
	return true
}

func globsSegmentsOverlap(a, b []string) bool {
	for {
		if len(a) == 0 || len(b) == 0 {
			return onlyDoubleStars(a) && onlyDoubleStars(b)
		}
		if a[0] == "**" || b[0] == "**" {
			return true
		}
		x, y := a[0], b[0]
		switch {
		case !hasGlobMeta(x) && !hasGlobMeta(y):
			if x != y {
				return false
			}
		case hasGlobMeta(x) && hasGlobMeta(y):
			// both wildcards: assume they can match the same name
		case hasGlobMeta(x):
			if m, err := doublestar.Match(x, y); err != nil || !m {
				return false
			}
		default:
			if m, err := doublestar.Match(y, x); err != nil || !m {
				return false
			}
		}
		a, b = a[1:], b[1:]
	}
}

func (c *classifier) classifyCreate(op PolicyOp, summary string) PolicyVerdict {
	n := op.Proposed
	if n == nil {
		return blocked(op, BlockNotAuto, "staged node body is missing", summary)
	}
	if isExternalNode(n) {
		return blocked(op, BlockExternal, "new external nodes always need a human", summary)
	}
	// The same sensitive-field list as classifyUpdate: any of these set on a new node
	// needs a human.
	if f := presentSensitiveFields(n); len(f) > 0 {
		return blocked(op, BlockSensitive, fmt.Sprintf("a new node that sets %s always needs a human", strings.Join(f, ", ")), summary)
	}
	switch n.Status {
	case "", StatusDraft, StatusSpecified:
	default:
		return blocked(op, BlockNotAuto, fmt.Sprintf("a new node with status %s is not covered by the policy", n.Status), summary)
	}
	if n.RelationshipID != "" || n.ParentID == "" {
		return blocked(op, BlockNotAuto, "only a node under an existing container is covered (needs parent_id)", summary)
	}
	parent := c.node(n.ParentID)
	if parent == nil {
		return blocked(op, BlockNotAuto, fmt.Sprintf("parent %s is not an existing node", n.ParentID), summary)
	}
	if parent.Protected {
		return blocked(op, BlockProtected, fmt.Sprintf("parent %s is protected", parent.ID), summary)
	}
	if isExternalNode(parent) {
		return blocked(op, BlockExternal, fmt.Sprintf("parent %s is external", parent.ID), summary)
	}
	for _, d := range n.DependsOn {
		t := c.node(d)
		if t == nil {
			return blocked(op, BlockNotAuto, fmt.Sprintf("dependency %s is not an existing node", d), summary)
		}
		if isExternalNode(t) {
			return blocked(op, BlockExternal, fmt.Sprintf("dependency %s is external", d), summary)
		}
		if t.Protected {
			return blocked(op, BlockProtected, fmt.Sprintf("dependency %s is a protected node", d), summary)
		}
		if c.reaches(d, n.ID) {
			return blocked(op, BlockNotAuto, fmt.Sprintf("dependency %s would create a cycle", d), summary)
		}
	}
	for _, f := range []struct {
		name  string
		globs []string
	}{{"implementation", n.Implementation}, {"scope", n.Scope}} {
		if rule, reason := c.checkGlobAdditions(f.name, n.ID, nil, f.globs, n.Deliverables, nil, false); reason != "" {
			return blocked(op, rule, reason, summary)
		}
	}
	c.created[n.ID] = true
	return allowed(op, []string{RuleCreateContainer}, summary)
}

func (c *classifier) classifyEdge(op PolicyOp, summary string) PolicyVerdict {
	from, to := c.node(op.From), c.node(op.To)
	if from == nil || to == nil {
		return blocked(op, BlockNotAuto, "both endpoints must be existing nodes", summary)
	}
	if from.Protected || to.Protected {
		return blocked(op, BlockProtected, "an endpoint is a protected node", summary)
	}
	if isExternalNode(from) || isExternalNode(to) {
		return blocked(op, BlockExternal, "an endpoint is an external node (cost implications)", summary)
	}
	if rel := op.CurrentRel; rel != nil {
		if rel.From != op.From || rel.To != op.To {
			return blocked(op, BlockNotAuto, "moving an existing edge to other endpoints is not covered", summary)
		}
		if op.Kind != rel.Kind {
			return blocked(op, BlockNotAuto, fmt.Sprintf("changing the edge kind (%q to %q) is not covered", rel.Kind, op.Kind), summary)
		}
	} else if op.Kind != "" && op.Kind != KindSync {
		return blocked(op, BlockNotAuto, fmt.Sprintf("a new %s edge has kind or cost implications", op.Kind), summary)
	}
	return allowed(op, []string{RuleEdgeExisting}, summary)
}

// summarizeOp is the short diff text for audit lines and refusals.
func summarizeOp(op PolicyOp) string {
	switch op.Op {
	case OpUpdateNode:
		parts := []string{"fields " + strings.Join(op.Fields, ", ")}
		if op.Current != nil && op.Proposed != nil {
			for _, f := range op.Fields {
				switch f {
				case "depends_on":
					parts = append(parts, diffIDs("depends_on", op.Current.DependsOn, op.Proposed.DependsOn))
				case "implementation":
					parts = append(parts, diffStrings("implementation", op.Current.Implementation, op.Proposed.Implementation))
				case "scope":
					parts = append(parts, diffStrings("scope", op.Current.Scope, op.Proposed.Scope))
				}
			}
		}
		return fmt.Sprintf("update %s: %s", op.NodeID, strings.Join(parts, "; "))
	case OpCreateNode:
		if op.Proposed != nil {
			under := string(op.Proposed.ParentID)
			if under == "" {
				under = "(top level)"
			}
			return fmt.Sprintf("create %s (%s) under %s", op.NodeID, op.Proposed.Type, under)
		}
		return "create " + op.NodeID
	case OpSetRelationship:
		state := "new"
		if op.CurrentRel != nil {
			state = "update"
		}
		text := fmt.Sprintf("%s edge %s -> %s label=%q kind=%q", state, op.From, op.To, op.Label, op.Kind)
		switch {
		case op.Contract == nil:
		case op.Contract.IsEmpty():
			text += "; contract cleared"
		default:
			text += "; contract set"
			if op.CurrentRel != nil && op.CurrentRel.Contract != nil && !op.CurrentRel.Contract.IsEmpty() {
				text = strings.TrimSuffix(text, "set") + "changed"
			}
		}
		return text
	case OpRenameNode:
		return fmt.Sprintf("rename %s -> %s", op.NodeID, op.NewID)
	}
	return fmt.Sprintf("%s %s", op.Op, op.NodeID)
}

func diffIDs(name string, before, after []NodeID) string {
	a := make([]string, len(before))
	for i, v := range before {
		a[i] = string(v)
	}
	b := make([]string, len(after))
	for i, v := range after {
		b[i] = string(v)
	}
	return diffStrings(name, a, b)
}

func diffStrings(name string, before, after []string) string {
	have := map[string]bool{}
	for _, v := range before {
		have[v] = true
	}
	want := map[string]bool{}
	for _, v := range after {
		want[v] = true
	}
	var parts []string
	for _, v := range after {
		if !have[v] {
			parts = append(parts, "+"+v)
		}
	}
	for _, v := range before {
		if !want[v] {
			parts = append(parts, "-"+v)
		}
	}
	return name + " " + strings.Join(parts, " ")
}
