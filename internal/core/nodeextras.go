package core

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Optional build-planning node fields (stage C). All of them are optional and bounded.
const (
	GateDecision       = "decision"
	GateCredential     = "credential"
	GatePhone          = "phone"
	GateDeployApproval = "deploy_approval"
	GateInstall        = "install"

	MilestonePending    = "pending"
	MilestoneInProgress = "in_progress"
	MilestoneDone       = "done"

	MaxHumanGates   = 16
	MaxGateNoteLen  = 300
	MaxDeliverables = 32
	MaxReferences   = 16
	MaxPathLen      = 300
	MaxMilestones   = 16
	MaxMilestoneLbl = 200
	MaxMilestoneID  = 64
	MaxGates        = 64
)

// HumanGate is a step only a person can do before or while a node is built.
type HumanGate struct {
	Kind string `json:"kind"`
	Note string `json:"note,omitempty"`
}

// Milestone is one checkpoint inside a node that is finished in stages.
type Milestone struct {
	ID     string `json:"id"`
	Label  string `json:"label,omitempty"`
	Status string `json:"status,omitempty"`
}

// NodeExtras groups the optional build-planning fields embedded in Node and CreateInput.
type NodeExtras struct {
	// External marks a node outside the build (a service, a person): it is left out of the
	// build order and listed separately.
	External bool `json:"external,omitempty"`
	// HumanGates lists decisions, credentials and other human steps the node needs.
	HumanGates []HumanGate `json:"human_gates,omitempty"`
	// Deliverables are repo-relative paths the node must produce (no globs needed).
	Deliverables []string `json:"deliverables,omitempty"`
	// References are repo-relative doc paths the node's spec or contract must stay in sync with.
	References []string `json:"references,omitempty"`
	// Shared marks the node as the owner of shared manifests (go.mod, package.json, lockfiles).
	Shared bool `json:"shared,omitempty"`
	// Milestones are checkpoints of a node finished in stages.
	Milestones []Milestone `json:"milestones,omitempty"`
	// Gates lists nodes that wait for this node (a spike): each listed node effectively
	// depends on this one.
	Gates []NodeID `json:"gates,omitempty"`
}

// NodeExtrasPatch is NodeExtras for UpdateInput: nil leaves a field unchanged and a
// non-nil empty slice clears a list.
type NodeExtrasPatch struct {
	External     *bool        `json:"external,omitempty"`
	HumanGates   *[]HumanGate `json:"human_gates,omitempty"`
	Deliverables *[]string    `json:"deliverables,omitempty"`
	References   *[]string    `json:"references,omitempty"`
	Shared       *bool        `json:"shared,omitempty"`
	Milestones   *[]Milestone `json:"milestones,omitempty"`
	Gates        *[]NodeID    `json:"gates,omitempty"`
}

var milestoneIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:[-_][a-z0-9]+)*$`)

func validHumanGateKind(k string) bool {
	switch k {
	case GateDecision, GateCredential, GatePhone, GateDeployApproval, GateInstall:
		return true
	}
	return false
}

// cleanRelPath trims and validates a repo-relative path: no absolute or parent segments.
func cleanRelPath(field, p string) (string, error) {
	p = strings.TrimSpace(toSlash(p))
	if p == "" {
		return "", nil
	}
	if utf8.RuneCountInString(p) > MaxPathLen {
		return "", fmt.Errorf("%w: a %s path exceeds %d characters", ErrBadNodeMeta, field, MaxPathLen)
	}
	if strings.HasPrefix(p, "/") || (len(p) > 1 && p[1] == ':') || strings.HasPrefix(p, "~") {
		return "", fmt.Errorf("%w: %s path %q must be relative to the repository", ErrBadNodeMeta, field, p)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return "", fmt.Errorf("%w: %s path %q must not contain ..", ErrBadNodeMeta, field, p)
		}
	}
	if strings.ContainsRune(p, 0) {
		return "", fmt.Errorf("%w: %s path contains a NUL byte", ErrBadNodeMeta, field)
	}
	c := path.Clean(p)
	if c == "." {
		return "", fmt.Errorf("%w: %s path %q is empty", ErrBadNodeMeta, field, p)
	}
	return c, nil
}

func cleanPathList(field string, in []string, max int) ([]string, error) {
	if len(in) > max {
		return nil, fmt.Errorf("%w: at most %d %s", ErrBadNodeMeta, max, field)
	}
	var out []string
	seen := map[string]bool{}
	for _, p := range in {
		c, err := cleanRelPath(field, p)
		if err != nil {
			return nil, err
		}
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out, nil
}

// normalizeExtras trims, bounds and validates every optional build-planning field. It does
// not check that gate ids exist; validateGatesIn does that against a node map.
func normalizeExtras(e NodeExtras) (NodeExtras, error) {
	var out NodeExtras
	out.External, out.Shared = e.External, e.Shared
	if len(e.HumanGates) > MaxHumanGates {
		return out, fmt.Errorf("%w: at most %d human_gates", ErrBadNodeMeta, MaxHumanGates)
	}
	for _, g := range e.HumanGates {
		g.Kind, g.Note = strings.ToLower(strings.TrimSpace(g.Kind)), strings.TrimSpace(g.Note)
		if !validHumanGateKind(g.Kind) {
			return out, fmt.Errorf("%w: human gate kind %q (valid: %s, %s, %s, %s, %s)", ErrBadNodeMeta, g.Kind,
				GateDecision, GateCredential, GatePhone, GateDeployApproval, GateInstall)
		}
		if utf8.RuneCountInString(g.Note) > MaxGateNoteLen {
			return out, fmt.Errorf("%w: a human gate note exceeds %d characters", ErrBadNodeMeta, MaxGateNoteLen)
		}
		out.HumanGates = append(out.HumanGates, g)
	}
	var err error
	if out.Deliverables, err = cleanPathList("deliverables", e.Deliverables, MaxDeliverables); err != nil {
		return out, err
	}
	if out.References, err = cleanPathList("references", e.References, MaxReferences); err != nil {
		return out, err
	}
	if len(e.Milestones) > MaxMilestones {
		return out, fmt.Errorf("%w: at most %d milestones", ErrBadNodeMeta, MaxMilestones)
	}
	seen := map[string]bool{}
	for _, m := range e.Milestones {
		m.ID, m.Label, m.Status = strings.TrimSpace(m.ID), strings.TrimSpace(m.Label), strings.ToLower(strings.TrimSpace(m.Status))
		if utf8.RuneCountInString(m.ID) > MaxMilestoneID || !milestoneIDPattern.MatchString(m.ID) {
			return out, fmt.Errorf("%w: milestone id %q must be a lowercase slug of at most %d characters", ErrBadNodeMeta, m.ID, MaxMilestoneID)
		}
		if seen[m.ID] {
			return out, fmt.Errorf("%w: milestone id %q listed more than once", ErrBadNodeMeta, m.ID)
		}
		seen[m.ID] = true
		if utf8.RuneCountInString(m.Label) > MaxMilestoneLbl {
			return out, fmt.Errorf("%w: a milestone label exceeds %d characters", ErrBadNodeMeta, MaxMilestoneLbl)
		}
		switch m.Status {
		case "":
			m.Status = MilestonePending
		case MilestonePending, MilestoneInProgress, MilestoneDone:
		default:
			return out, fmt.Errorf("%w: milestone status %q (valid: pending, in_progress, done)", ErrBadNodeMeta, m.Status)
		}
		out.Milestones = append(out.Milestones, m)
	}
	if len(e.Gates) > MaxGates {
		return out, fmt.Errorf("%w: at most %d gates", ErrBadNodeMeta, MaxGates)
	}
	out.Gates = sortedDeps(e.Gates)
	return out, nil
}

// applyTo applies the patch to cur and returns the normalised result and the names of the
// fields it touched.
func (p NodeExtrasPatch) applyTo(cur NodeExtras) (NodeExtras, []string, error) {
	var fields []string
	if p.External != nil {
		cur.External = *p.External
		fields = append(fields, "external")
	}
	if p.HumanGates != nil {
		cur.HumanGates = append([]HumanGate(nil), *p.HumanGates...)
		fields = append(fields, "human_gates")
	}
	if p.Deliverables != nil {
		cur.Deliverables = cloneStrings(*p.Deliverables)
		fields = append(fields, "deliverables")
	}
	if p.References != nil {
		cur.References = cloneStrings(*p.References)
		fields = append(fields, "references")
	}
	if p.Shared != nil {
		cur.Shared = *p.Shared
		fields = append(fields, "shared")
	}
	if p.Milestones != nil {
		cur.Milestones = append([]Milestone(nil), *p.Milestones...)
		fields = append(fields, "milestones")
	}
	if p.Gates != nil {
		cur.Gates = append([]NodeID(nil), *p.Gates...)
		fields = append(fields, "gates")
	}
	if len(fields) == 0 {
		return cur, nil, nil
	}
	out, err := normalizeExtras(cur)
	return out, fields, err
}

// cloneExtras copies the slices so a clone never aliases the original.
func cloneExtras(e NodeExtras) NodeExtras {
	e.HumanGates = append([]HumanGate(nil), e.HumanGates...)
	e.Deliverables = cloneStrings(e.Deliverables)
	e.References = cloneStrings(e.References)
	e.Milestones = append([]Milestone(nil), e.Milestones...)
	e.Gates = append([]NodeID(nil), e.Gates...)
	return e
}

// validateGatesIn checks the gates list of self: ids must exist, no self-reference, no
// duplicates, and (with depends_on and pure containers expanded as the build order does)
// no dependency cycle. self may not be in nodes yet (create).
func validateGatesIn(nodes map[NodeID]*Node, self, parent NodeID, deps, gates []NodeID) error {
	if len(gates) == 0 {
		return nil
	}
	seen := map[NodeID]bool{}
	for _, g := range gates {
		switch {
		case g == self:
			return fmt.Errorf("%w: node %s cannot gate itself", ErrBadDependency, self)
		case seen[g]:
			return fmt.Errorf("%w: gate %q listed more than once", ErrBadDependency, g)
		}
		seen[g] = true
		if _, ok := nodes[g]; !ok {
			return fmt.Errorf("%w: gated node %q does not exist", ErrBadDependency, g)
		}
	}
	return expandedCycleCheckGates(nodes, self, parent, deps, gates)
}

// gateLabel renders human gates for text output.
func gateLabel(g HumanGate) string {
	if g.Note == "" {
		return g.Kind
	}
	return g.Kind + ": " + g.Note
}

// decodeNodeExtras reads the optional build-planning keys from a node object into n.
func decodeNodeExtras(obj map[string]jsonRaw, n *Node) error {
	for _, f := range []struct {
		key string
		dst *bool
	}{{"external", &n.External}, {"shared", &n.Shared}} {
		if raw, ok := take(obj, f.key); ok && !isNull(raw) {
			v, err := decodeBool(raw)
			if err != nil {
				return fmt.Errorf("%s: %w", f.key, err)
			}
			*f.dst = v
		}
	}
	for _, f := range []struct {
		key string
		dst *[]string
	}{{"deliverables", &n.Deliverables}, {"references", &n.References}} {
		if raw, ok := take(obj, f.key); ok && !isNull(raw) {
			v, err := decodeStringSlice(raw)
			if err != nil {
				return fmt.Errorf("%s must be an array of strings", f.key)
			}
			*f.dst = v
		}
	}
	if raw, ok := take(obj, "gates"); ok && !isNull(raw) {
		v, err := decodeStringSlice(raw)
		if err != nil {
			return fmt.Errorf("gates must be an array of strings")
		}
		for _, g := range v {
			n.Gates = append(n.Gates, NodeID(g))
		}
	}
	if raw, ok := take(obj, "human_gates"); ok && !isNull(raw) {
		if err := json.Unmarshal(raw, &n.HumanGates); err != nil {
			return fmt.Errorf("human_gates must be an array of {kind, note}")
		}
	}
	if raw, ok := take(obj, "milestones"); ok && !isNull(raw) {
		if err := json.Unmarshal(raw, &n.Milestones); err != nil {
			return fmt.Errorf("milestones must be an array of {id, label, status}")
		}
	}
	return nil
}

// encodeNodeExtras writes the non-empty build-planning fields through put.
func encodeNodeExtras(n *Node, put func(string, any) error) error {
	if n.External {
		if err := put("external", true); err != nil {
			return err
		}
	}
	if len(n.HumanGates) > 0 {
		if err := put("human_gates", n.HumanGates); err != nil {
			return err
		}
	}
	if len(n.Deliverables) > 0 {
		if err := put("deliverables", n.Deliverables); err != nil {
			return err
		}
	}
	if len(n.References) > 0 {
		if err := put("references", n.References); err != nil {
			return err
		}
	}
	if n.Shared {
		if err := put("shared", true); err != nil {
			return err
		}
	}
	if len(n.Milestones) > 0 {
		if err := put("milestones", n.Milestones); err != nil {
			return err
		}
	}
	if len(n.Gates) > 0 {
		gates := make([]string, len(n.Gates))
		for i, g := range n.Gates {
			gates[i] = string(g)
		}
		sort.Strings(gates)
		if err := put("gates", gates); err != nil {
			return err
		}
	}
	return nil
}
