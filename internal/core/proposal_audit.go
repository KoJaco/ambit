package core

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Actors recorded in the audit log.
const (
	ActorArchitectAgent = "architect-agent"
	ActorHuman          = "human"
)

// AuditFile is the append-only accept log, inside the gitignored proposals directory.
const AuditFile = "audit.jsonl"

// maxAuditRead bounds how many (newest) entries a read returns.
const maxAuditRead = 1000

// AuditOp is one accepted operation in an audit entry.
type AuditOp struct {
	Index int    `json:"index"`
	Op    string `json:"op"`
	Node  string `json:"node"`
	// Rule is the policy rule that allowed the operation, or "human" for a human accept.
	Rule        string `json:"rule"`
	DiffSummary string `json:"diff_summary"`
}

// AuditEntry is one accepted proposal.
type AuditEntry struct {
	TS         string `json:"ts"`
	ProposalID string `json:"proposal_id"`
	Actor      string `json:"actor"`
	// OriginHash is a short hash of the staging session's origin (never the raw value);
	// empty for a proposal staged from the UI.
	OriginHash string `json:"origin_hash,omitempty"`
	// AccepterHash is the same for the accepting session (architect-agent entries only).
	AccepterHash string    `json:"accepter_hash,omitempty"`
	Ops          []AuditOp `json:"ops"`
}

// hashOrigin returns a short, stable, non-reversible fingerprint of an origin id.
func hashOrigin(origin string) string {
	if origin == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(origin))
	return hex.EncodeToString(sum[:8])
}

func (idx *Index) auditPath() string {
	return filepath.Join(idx.proposalsDir(), AuditFile)
}

// checkAuditWritable makes sure an entry can be appended, so auto-accept can refuse
// before it changes the model rather than after.
func (idx *Index) checkAuditWritable() error {
	if err := os.MkdirAll(idx.proposalsDir(), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(idx.auditPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	return f.Close()
}

// appendAudit writes one JSON line. The file is only ever appended to.
func (idx *Index) appendAudit(e AuditEntry) error {
	if e.TS == "" {
		e.TS = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if e.Ops == nil {
		e.Ops = []AuditOp{}
	}
	line, err := marshalNoEscape(e)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(idx.proposalsDir(), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(idx.auditPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// AuditQuery filters the log.
type AuditQuery struct {
	// Since keeps entries with ts after this RFC 3339 time. Empty keeps all.
	Since string
	// Actor keeps entries by that actor. Empty keeps all.
	Actor string
}

// ReadAudit returns the matching entries, oldest first, at most the newest 1000. A missing
// log is empty. Malformed lines are skipped.
func (idx *Index) ReadAudit(q AuditQuery) ([]AuditEntry, error) {
	var since time.Time
	if q.Since != "" {
		t, err := time.Parse(time.RFC3339Nano, q.Since)
		if err != nil {
			return nil, fmt.Errorf("%w: since %q must be an RFC 3339 time", ErrBadOperation, q.Since)
		}
		since = t
	}
	f, err := os.Open(idx.auditPath())
	if err != nil {
		if os.IsNotExist(err) {
			return []AuditEntry{}, nil
		}
		return nil, err
	}
	defer f.Close()
	out := []AuditEntry{}
	r := bufio.NewReaderSize(f, 64*1024)
	for {
		line, err := r.ReadBytes('\n')
		if len(strings.TrimSpace(string(line))) > 0 {
			var e AuditEntry
			if json.Unmarshal(line, &e) == nil {
				keep := q.Actor == "" || e.Actor == q.Actor
				if keep && !since.IsZero() {
					ts, perr := time.Parse(time.RFC3339Nano, e.TS)
					keep = perr == nil && ts.After(since)
				}
				if keep {
					out = append(out, e)
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	if len(out) > maxAuditRead {
		out = out[len(out)-maxAuditRead:]
	}
	return out, nil
}

// AuditRuleCount is a count of accepted operations per rule.
type AuditRuleCount struct {
	Rule  string `json:"rule"`
	Count int    `json:"count"`
}

// AuditDigest summarises accepted proposals: counts by rule, plus the entries.
type AuditDigest struct {
	Since     string           `json:"since,omitempty"`
	Actor     string           `json:"actor,omitempty"`
	Proposals int              `json:"proposals"`
	Ops       int              `json:"ops"`
	ByRule    []AuditRuleCount `json:"by_rule"`
	Entries   []AuditEntry     `json:"entries"`
}

// AuditDigestFor builds the digest for q.
func (idx *Index) AuditDigestFor(q AuditQuery) (AuditDigest, error) {
	entries, err := idx.ReadAudit(q)
	if err != nil {
		return AuditDigest{}, err
	}
	d := AuditDigest{Since: q.Since, Actor: q.Actor, Proposals: len(entries), ByRule: []AuditRuleCount{}, Entries: entries}
	counts := map[string]int{}
	for _, e := range entries {
		for _, op := range e.Ops {
			d.Ops++
			// A mixed rule ("dependency-add-acyclic,spec-text") counts under each rule.
			for _, r := range strings.Split(op.Rule, ",") {
				counts[r]++
			}
		}
	}
	for r, n := range counts {
		d.ByRule = append(d.ByRule, AuditRuleCount{Rule: r, Count: n})
	}
	sort.Slice(d.ByRule, func(i, j int) bool {
		if d.ByRule[i].Count != d.ByRule[j].Count {
			return d.ByRule[i].Count > d.ByRule[j].Count
		}
		return d.ByRule[i].Rule < d.ByRule[j].Rule
	})
	return d, nil
}

// policyOpsLocked builds the classifier input for the pending operations of m.
func (idx *Index) policyOpsLocked(m manifest) []PolicyOp {
	var out []PolicyOp
	for i, op := range m.Operations {
		if op.Status != opStatusPending {
			continue
		}
		po := PolicyOp{Index: i, Op: op.Op, NodeID: op.NodeID, NewID: op.NewID, Fields: op.Fields}
		po.Stale, po.StaleReason = idx.staleness(m.Operations, op)
		switch op.Op {
		case OpCreateNode, OpUpdateNode:
			po.Current = idx.Nodes[NodeID(op.NodeID)]
			if op.Op == OpCreateNode {
				po.Current = nil
			}
			po.Proposed = idx.readProposedNode(m.ProposalID, op.NodeID)
		case OpDeleteNode, OpRenameNode:
			po.Current = idx.Nodes[NodeID(op.NodeID)]
		case OpSetRelationship:
			po.From, po.To = NodeID(op.From), NodeID(op.To)
			po.RelID = op.RelID
			po.Label, po.Kind = deref(op.Label), deref(op.Kind)
			if rel, _ := idx.relByID(RelID(op.RelID)); rel != nil {
				po.CurrentRel = rel
			}
		}
		out = append(out, po)
	}
	return out
}

// readProposedNode decodes the staged body of a create or update, or nil when unreadable.
func (idx *Index) readProposedNode(proposalID, nodeID string) *Node {
	if !slugPattern.MatchString(nodeID) {
		return nil
	}
	jsonBytes, mdBytes, err := readPair(filepath.Join(idx.mustProposalPath(proposalID), "nodes", nodeID))
	if err != nil {
		return nil
	}
	n, err := decodeNode(jsonBytes)
	if err != nil {
		return nil
	}
	n.Spec = string(mdBytes)
	return n
}

func (idx *Index) policyModelLocked() PolicyModel {
	return PolicyModel{Nodes: idx.Nodes}
}

// AcceptAllAs is AcceptAll that also appends an audit entry (actor human) for the
// operations it applied. The audit write is best effort for a human accept.
func (idx *Index) AcceptAllAs(id, actor string) (AcceptAllResult, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	var summaries map[int]PolicyVerdict
	var origin string
	if m, err := idx.loadReadable(id); err == nil {
		origin = m.Origin
		summaries = map[int]PolicyVerdict{}
		ops := idx.policyOpsLocked(m)
		for _, v := range ClassifyOps(idx.policyModelLocked(), ops) {
			summaries[v.Index] = v
		}
	}
	res, err := idx.acceptAllLocked(id)
	if err != nil || len(res.Applied) == 0 {
		return res, err
	}
	entry := AuditEntry{ProposalID: id, Actor: actor, OriginHash: hashOrigin(origin)}
	for _, i := range res.Applied {
		v := summaries[i]
		entry.Ops = append(entry.Ops, AuditOp{Index: i, Op: v.Op, Node: v.Node, Rule: "human", DiffSummary: v.Summary})
	}
	_ = idx.appendAudit(entry)
	return res, nil
}

// AutoAcceptResult is the outcome of AutoAcceptProposal.
type AutoAcceptResult struct {
	Accepted   bool            `json:"accepted"`
	ProposalID string          `json:"proposal_id"`
	Ops        []PolicyVerdict `json:"ops"`
	// Blocking lists the operations that need a human (and why) when Accepted is false.
	Blocking []PolicyVerdict `json:"blocking,omitempty"`
	Applied  []int           `json:"applied,omitempty"`
}

// AutoAcceptProposal accepts a pending proposal on behalf of an architect agent when the
// policy allows it. It refuses (ErrPolicyRefused) when auto_accept is off, the stage is not
// build, or acceptedBy (the accepting session's origin) is empty or equals the proposal's
// origin, so an agent cannot accept its own proposal. When any operation is not auto-
// eligible, or the proposal would introduce an enforced lint finding or does not apply
// cleanly to a scratch copy of the model, nothing is applied and the blocking operations
// are returned. On success it applies the proposal with the normal accept-all path (snapshot
// and rollback) and appends an audit entry.
func (idx *Index) AutoAcceptProposal(id, acceptedBy string) (AutoAcceptResult, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	res := AutoAcceptResult{ProposalID: id, Ops: []PolicyVerdict{}}
	// Honour a stage or policy change made by another process since the last load.
	if err := idx.reload(); err != nil {
		return res, err
	}
	info := idx.policyInfoLocked()
	if !info.Effective {
		return res, fmt.Errorf("%w: %s", ErrPolicyRefused, info.Reason)
	}
	if acceptedBy == "" {
		return res, fmt.Errorf("%w: no accepting session identity", ErrPolicyRefused)
	}
	m, err := idx.loadReadable(id)
	if err != nil {
		return res, err
	}
	if m.Origin != "" && m.Origin == acceptedBy {
		return res, fmt.Errorf("%w: this session staged proposal %s and cannot accept it; a different architect session or a human must", ErrPolicyRefused, id)
	}
	if err := idx.dependsOnPending(m); err != nil {
		return res, err
	}
	ops := idx.policyOpsLocked(m)
	if len(ops) == 0 {
		return res, fmt.Errorf("%w: proposal %s has no pending operations", ErrBadOperation, id)
	}
	res.Ops = ClassifyOps(idx.policyModelLocked(), ops)
	if !AllAuto(res.Ops) {
		res.Blocking = Blocking(res.Ops)
		return res, nil
	}
	if verdict, err := idx.trialLocked(id); err != nil {
		return res, err
	} else if verdict != nil {
		res.Blocking = []PolicyVerdict{*verdict}
		return res, nil
	}
	if err := idx.checkAuditWritable(); err != nil {
		return res, fmt.Errorf("%w: audit log is not writable: %v", ErrPolicyRefused, err)
	}
	applied, err := idx.acceptAllLocked(id)
	if err != nil {
		return res, err
	}
	if len(applied.Failed) > 0 || len(applied.Skipped) > 0 || len(applied.Applied) != len(ops) {
		// The scratch trial said this applies cleanly, so reaching here means the model
		// changed underneath us. Failures were rolled back; skipped ops stay pending.
		res.Applied = applied.Applied
		res.Blocking = []PolicyVerdict{{Index: -1, Op: "proposal", Node: id, Rule: BlockTrial, Reason: fmt.Sprintf("accept did not apply every operation (skipped %d, failed %d)", len(applied.Skipped), len(applied.Failed)), Summary: "accept-all result"}}
		if len(applied.Applied) > 0 {
			idx.auditAutoLocked(id, m.Origin, acceptedBy, res.Ops, applied.Applied)
		}
		return res, nil
	}
	res.Accepted, res.Applied = true, applied.Applied
	if err := idx.auditAutoLocked(id, m.Origin, acceptedBy, res.Ops, applied.Applied); err != nil {
		return res, fmt.Errorf("proposal %s was accepted but the audit entry could not be written: %w", id, err)
	}
	return res, nil
}

func (idx *Index) auditAutoLocked(id, origin, acceptedBy string, verdicts []PolicyVerdict, applied []int) error {
	byIndex := map[int]PolicyVerdict{}
	for _, v := range verdicts {
		byIndex[v.Index] = v
	}
	entry := AuditEntry{ProposalID: id, Actor: ActorArchitectAgent, OriginHash: hashOrigin(origin), AccepterHash: hashOrigin(acceptedBy)}
	for _, i := range applied {
		v := byIndex[i]
		entry.Ops = append(entry.Ops, AuditOp{Index: i, Op: v.Op, Node: v.Node, Rule: v.Rule, DiffSummary: v.Summary})
	}
	return idx.appendAudit(entry)
}

// trialLocked applies the proposal to a scratch copy of the model and compares enforced
// lint findings. It returns a blocking verdict when the copy does not take the whole
// proposal, or when the result has an enforced finding the live model does not have.
func (idx *Index) trialLocked(id string) (*PolicyVerdict, error) {
	tmp, err := os.MkdirTemp("", "ambit-trial-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if err := copyTree(idx.arch(), filepath.Join(tmp, ".arch"), func(rel string) bool {
		return rel == ".cache" || rel == ".proposals"
	}); err != nil {
		return nil, err
	}
	if err := copyTree(idx.mustProposalPath(id), filepath.Join(tmp, ".arch", ".proposals", id), nil); err != nil {
		return nil, err
	}
	trial, err := Open(tmp)
	if err != nil {
		return nil, err
	}
	block := func(rule, reason string) *PolicyVerdict {
		return &PolicyVerdict{Index: -1, Op: "proposal", Node: id, Rule: rule, Reason: reason, Summary: "trial apply on a scratch copy"}
	}
	// CheckModel locks the index it is given, and idx is locked here, so lint the live
	// model through a second Index over the same directory.
	live, err := Open(idx.Dir)
	if err != nil {
		return nil, err
	}
	before, err := CheckModel(live, CheckModelOptions{IncludeDrift: boolRef(false)})
	if err != nil {
		return nil, err
	}
	pending := 0
	if m, err := trial.loadReadable(id); err == nil {
		for _, op := range m.Operations {
			if op.Status == opStatusPending {
				pending++
			}
		}
	}
	applied, err := trial.AcceptAll(id)
	if err != nil {
		return block(BlockTrial, "the proposal does not apply cleanly: "+err.Error()), nil
	}
	if len(applied.Failed) > 0 {
		return block(BlockTrial, "the proposal fails validation when applied: "+applied.Failed[0].Error), nil
	}
	if len(applied.Skipped) > 0 || len(applied.Applied) != pending {
		return block(BlockTrial, fmt.Sprintf("%d operation(s) would be skipped as stale or unmet when applied", len(applied.Skipped))), nil
	}
	after, err := CheckModel(trial, CheckModelOptions{IncludeDrift: boolRef(false)})
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, f := range before.Findings {
		if f.Severity == LintEnforced {
			known[lintKey(f)] = true
		}
	}
	var fresh []string
	for _, f := range after.Findings {
		if f.Severity == LintEnforced && !known[lintKey(f)] {
			fresh = append(fresh, fmt.Sprintf("%s: %s", f.Rule, f.Message))
		}
	}
	if len(fresh) > 0 {
		if len(fresh) > 5 {
			fresh = append(fresh[:5], fmt.Sprintf("and %d more", len(fresh)-5))
		}
		return block(BlockLint, "applying would add enforced lint finding(s): "+strings.Join(fresh, "; ")), nil
	}
	return nil, nil
}

func lintKey(f LintFinding) string {
	return f.Rule + "\x00" + string(f.NodeID) + "\x00" + string(f.RelationshipID) + "\x00" + f.Message
}

func boolRef(b bool) *bool { return &b }

// copyTree copies regular files and directories from src to dst. Symlinks and other
// special files are skipped. skip, when non-nil, excludes a path relative to src (and,
// for a directory, everything under it).
func copyTree(src, dst string, skip func(rel string) bool) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel != "." && skip != nil && skip(filepath.ToSlash(rel)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type().IsRegular():
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			return os.WriteFile(target, data, 0o600)
		}
		return nil
	})
}
