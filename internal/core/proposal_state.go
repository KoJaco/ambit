package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Proposal states returned by ProposalState.
const (
	ProposalPending   = "pending"
	ProposalAccepted  = "accepted"
	ProposalRejected  = "rejected"
	ProposalPartial   = "partial"
	ProposalWithdrawn = "withdrawn"
)

// maxResolvedProposals bounds how many finished proposals are kept for lookup.
var maxResolvedProposals = 100

// OpState is one operation's outcome in a ProposalState.
type OpState struct {
	Index       int    `json:"index"`
	Op          string `json:"op"`
	NodeID      string `json:"node_id,omitempty"`
	Status      string `json:"status"`
	Stale       bool   `json:"stale,omitempty"`
	StaleReason string `json:"stale_reason,omitempty"`
}

// ProposalStateInfo answers "what happened to proposal X": still pending, or how it ended.
type ProposalStateInfo struct {
	ID         string `json:"proposal_id"`
	Status     string `json:"status"`
	Source     string `json:"source,omitempty"`
	Summary    string `json:"summary,omitempty"`
	ChangeSet  string `json:"change_set,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
	ResolvedAt string `json:"resolved_at,omitempty"`
	// AcceptedOps counts operations that were applied; it survives withdrawal or
	// supersession of a partially accepted proposal.
	AcceptedOps int       `json:"accepted_ops,omitempty"`
	Operations  []OpState `json:"operations"`
}

func countAccepted(ops []operation) int {
	n := 0
	for _, op := range ops {
		if op.Status == opStatusAccepted {
			n++
		}
	}
	return n
}

type resolvedMeta struct {
	ResolvedAt string `json:"resolved_at"`
	Status     string `json:"status"`
}

func (idx *Index) resolvedDir() string { return filepath.Join(idx.proposalsDir(), ".resolved") }

func (idx *Index) resolvedPath(id string) (string, error) {
	if !proposalIDPattern.MatchString(id) {
		return "", fmt.Errorf("%w: %q", ErrProposalNotFound, id)
	}
	return filepath.Join(idx.resolvedDir(), id), nil
}

func outcomeOf(ops []operation) string {
	accepted, rejected := 0, 0
	for _, op := range ops {
		switch op.Status {
		case opStatusAccepted:
			accepted++
		case opStatusRejected:
			rejected++
		}
	}
	switch {
	case accepted > 0 && rejected == 0 && accepted == len(ops):
		return ProposalAccepted
	case rejected > 0 && accepted == 0 && rejected == len(ops):
		return ProposalRejected
	default:
		return ProposalPartial
	}
}

// archiveResolved copies a proposal directory into .resolved/<id> with its final
// statuses so its outcome stays queryable and its accepted operations can be restaged.
// Best effort: a failure here never blocks accepting or rejecting.
func (idx *Index) archiveResolved(m manifest, status string) {
	src, err := idx.proposalPath(m.ProposalID)
	if err != nil {
		return
	}
	dst, err := idx.resolvedPath(m.ProposalID)
	if err != nil {
		return
	}
	_ = os.RemoveAll(dst)
	if err := os.MkdirAll(filepath.Join(dst, "nodes"), 0o755); err != nil {
		return
	}
	files, err := readDirFiles(filepath.Join(src, "nodes"))
	if err != nil {
		return
	}
	for name, data := range files {
		if err := writeAtomic(filepath.Join(dst, "nodes", name), data); err != nil {
			_ = os.RemoveAll(dst)
			return
		}
	}
	if err := writeManifest(dst, m); err != nil {
		_ = os.RemoveAll(dst)
		return
	}
	meta, _ := json.Marshal(resolvedMeta{ResolvedAt: time.Now().UTC().Format(time.RFC3339), Status: status})
	if err := writeAtomic(filepath.Join(dst, "resolved.json"), meta); err != nil {
		_ = os.RemoveAll(dst)
		return
	}
	idx.pruneResolved()
}

func (idx *Index) pruneResolved() {
	entries, err := os.ReadDir(idx.resolvedDir())
	if err != nil {
		return
	}
	type rec struct{ id, at string }
	var recs []rec
	for _, e := range entries {
		if !e.IsDir() || !proposalIDPattern.MatchString(e.Name()) {
			continue
		}
		at := ""
		if data, err := os.ReadFile(filepath.Join(idx.resolvedDir(), e.Name(), "resolved.json")); err == nil {
			var meta resolvedMeta
			if json.Unmarshal(data, &meta) == nil {
				at = meta.ResolvedAt
			}
		}
		recs = append(recs, rec{e.Name(), at})
	}
	// Keep the most recently resolved; ids break ties and order records without a time.
	sort.Slice(recs, func(i, j int) bool {
		if recs[i].at != recs[j].at {
			return recs[i].at < recs[j].at
		}
		return recs[i].id < recs[j].id
	})
	for len(recs) > maxResolvedProposals {
		_ = os.RemoveAll(filepath.Join(idx.resolvedDir(), recs[0].id))
		recs = recs[1:]
	}
}

func opStates(idx *Index, ops []operation, live bool) []OpState {
	out := make([]OpState, len(ops))
	for i, op := range ops {
		st := OpState{Index: i, Op: op.Op, NodeID: op.NodeID, Status: op.Status}
		if live && op.Status == opStatusPending {
			st.Stale, st.StaleReason = idx.staleness(ops, op)
		}
		out[i] = st
	}
	return out
}

// ProposalState reports where a proposal stands: pending (with per-operation status), or
// how it ended (accepted, rejected, partial or withdrawn) while its record is kept.
func (idx *Index) ProposalState(id string) (ProposalStateInfo, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.proposalStateLocked(id)
}

func (idx *Index) proposalStateLocked(id string) (ProposalStateInfo, error) {
	if dir, err := idx.proposalPath(id); err != nil {
		return ProposalStateInfo{}, err
	} else if _, statErr := os.Stat(dir); statErr == nil {
		m, err := idx.loadReadable(id)
		if err == nil {
			return ProposalStateInfo{
				ID: m.ProposalID, Status: ProposalPending, Source: m.Source, Summary: m.Summary, ChangeSet: m.ChangeSet,
				CreatedAt: m.CreatedAt, AcceptedOps: countAccepted(m.Operations), Operations: opStates(idx, m.Operations, true),
			}, nil
		}
		// Another process may be finishing this proposal right now: its directory is
		// still there but the manifest is already gone. Prefer the archived record.
		if _, rerr := idx.resolvedStateLocked(id); rerr != nil {
			return ProposalStateInfo{}, err
		}
	}
	return idx.resolvedStateLocked(id)
}

func (idx *Index) resolvedStateLocked(id string) (ProposalStateInfo, error) {
	dir, err := idx.resolvedPath(id)
	if err != nil {
		return ProposalStateInfo{}, err
	}
	m, err := readManifest(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return ProposalStateInfo{}, fmt.Errorf("%w: %s", ErrProposalNotFound, id)
		}
		return ProposalStateInfo{}, err
	}
	info := ProposalStateInfo{
		ID: m.ProposalID, Status: outcomeOf(m.Operations), Source: m.Source, Summary: m.Summary, ChangeSet: m.ChangeSet,
		CreatedAt: m.CreatedAt, AcceptedOps: countAccepted(m.Operations), Operations: opStates(idx, m.Operations, false),
	}
	if data, err := os.ReadFile(filepath.Join(dir, "resolved.json")); err == nil {
		var meta resolvedMeta
		if json.Unmarshal(data, &meta) == nil {
			info.ResolvedAt = meta.ResolvedAt
			if meta.Status != "" {
				info.Status = meta.Status
			}
		}
	}
	return info, nil
}

// WaitForProposal blocks until the proposal is no longer pending, the timeout passes or
// ctx ends. timedOut is true when it returned while the proposal was still pending.
func (idx *Index) WaitForProposal(ctx context.Context, id string, timeout time.Duration, poll time.Duration) (info ProposalStateInfo, timedOut bool, err error) {
	if poll <= 0 {
		poll = 500 * time.Millisecond
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(poll)
	defer tick.Stop()
	transient := 0
	for {
		info, err = idx.ProposalState(id)
		if err != nil {
			// Another process may be mid-way through finishing the proposal.
			if transient++; transient <= 3 && (errors.Is(err, ErrUnreadableProposal) || errors.Is(err, ErrProposalNotFound)) {
				select {
				case <-ctx.Done():
					return info, true, ctx.Err()
				case <-time.After(poll):
					continue
				}
			}
			return info, false, err
		}
		transient = 0
		if info.Status != ProposalPending {
			return info, false, nil
		}
		select {
		case <-ctx.Done():
			return info, true, ctx.Err()
		case <-deadline.C:
			return info, true, nil
		case <-tick.C:
		}
	}
}

// Restage creates a new pending proposal from operations of an existing one that were
// accepted (the proposal may still be pending or already finished). It exists so undoing
// an accepted proposal can put its changes back in front of the architect. indexes empty
// means every accepted operation. Base hashes are kept, so a model that was restored to
// its pre-accept state accepts them without being stale.
func (idx *Index) Restage(id string, indexes []int) (string, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	srcDir, err := idx.proposalPath(id)
	if err != nil {
		return "", err
	}
	if _, statErr := os.Stat(srcDir); statErr != nil {
		if srcDir, err = idx.resolvedPath(id); err != nil {
			return "", err
		}
	}
	m, err := readManifest(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", ErrProposalNotFound, id)
		}
		return "", err
	}
	if m.ManifestVersion != proposalManifestVersion {
		return "", fmt.Errorf("%w: %s: manifest_version %d is not supported", ErrUnreadableProposal, id, m.ManifestVersion)
	}
	if m.ProposalID != id {
		return "", fmt.Errorf("%w: %s: proposal_id %q does not match the directory", ErrUnreadableProposal, id, m.ProposalID)
	}
	want := map[int]bool{}
	for _, i := range indexes {
		if i < 0 || i >= len(m.Operations) {
			return "", fmt.Errorf("%w: operation index %d", ErrBadOperation, i)
		}
		want[i] = true
	}
	var ops []operation
	nodeIDs := map[string]bool{}
	for i, op := range m.Operations {
		if op.Status != opStatusAccepted || (len(want) > 0 && !want[i]) {
			if want[i] {
				return "", fmt.Errorf("%w: operation %d is %s, not accepted", ErrBadOperation, i, op.Status)
			}
			continue
		}
		if op.NodeID != "" {
			if !slugPattern.MatchString(op.NodeID) {
				return "", fmt.Errorf("%w: node id %q", ErrUnreadableProposal, op.NodeID)
			}
			if op.Op == OpCreateNode || op.Op == OpUpdateNode {
				if _, ok, err := readOptional(filepath.Join(srcDir, "nodes", op.NodeID+".json")); err != nil {
					return "", err
				} else if !ok {
					return "", fmt.Errorf("%w: %s %s has no staged file to restage", ErrUnreadableProposal, op.Op, op.NodeID)
				}
			}
			nodeIDs[op.NodeID] = true
		}
		op.Status = opStatusPending
		ops = append(ops, op)
	}
	if len(ops) == 0 {
		return "", fmt.Errorf("%w: no accepted operations to restage", ErrBadOperation)
	}
	now := time.Now().UTC()
	newID, err := idx.allocProposalID(now)
	if err != nil {
		return "", err
	}
	dst, err := idx.proposalPath(newID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(dst, "nodes"), 0o755); err != nil {
		return "", err
	}
	fail := func(err error) (string, error) { _ = os.RemoveAll(dst); return "", err }
	for nid := range nodeIDs {
		for _, ext := range []string{".json", ".md"} {
			data, existed, err := readOptional(filepath.Join(srcDir, "nodes", nid+ext))
			if err != nil {
				return fail(err)
			}
			if !existed {
				continue
			}
			if err := writeAtomic(filepath.Join(dst, "nodes", nid+ext), data); err != nil {
				return fail(err)
			}
		}
	}
	summary := m.Summary
	if summary != "" {
		summary = "Restaged after undo: " + summary
	}
	nm := manifest{
		ManifestVersion: proposalManifestVersion, ProposalID: newID, CreatedAt: now.Format(time.RFC3339),
		Source: m.Source, Summary: summary, ChangeSet: m.ChangeSet, Operations: ops,
		// Keep the staging origin (so the self-accept guard still holds) and mark the
		// proposal restaged: auto-accept never takes it.
		Origin: m.Origin, Restaged: true,
	}
	if err := writeManifest(dst, nm); err != nil {
		return fail(err)
	}
	return newID, nil
}

// orderOps returns the operations in staging (and so accept) order. Creates are ordered by
// orderCreates. Within each contiguous run of update_node operations, those that move a
// node in the hierarchy come first, so a move and a depends_on change written next to each
// other are checked against the final tree whichever order they were written in. Updates
// never move across a delete, create or edge operation. Renames move to the end, so
// every other operation of a proposal refers to a node by its current id. Everything
// else keeps its position.
func orderOps(ops []StageOp) []StageOp {
	ops = orderCreates(ops)
	var renames []StageOp
	var kept []StageOp
	for _, op := range ops {
		if op.Op == OpRenameNode {
			renames = append(renames, op)
			continue
		}
		kept = append(kept, op)
	}
	out := append([]StageOp(nil), kept...)
	// Permute only within contiguous runs of update_node, so an update never passes a
	// delete, create or edge operation.
	for start := 0; start < len(out); {
		if out[start].Op != OpUpdateNode {
			start++
			continue
		}
		end := start
		for end < len(out) && out[end].Op == OpUpdateNode {
			end++
		}
		var moves, others []StageOp
		for _, op := range out[start:end] {
			if op.Update.Parent != nil || op.Update.Relationship != nil {
				moves = append(moves, op)
			} else {
				others = append(others, op)
			}
		}
		copy(out[start:end], append(moves, others...))
		start = end
	}
	return append(out, renames...)
}

// orderCreates reorders the create_node operations among themselves so a node comes
// after its parent and after every node it depends on when those are created in the same
// call. Other operations keep their positions. Operations that cannot be ordered (cycles,
// unknown ids) keep their relative order, so normal validation reports them.
func orderCreates(ops []StageOp) []StageOp {
	var slots []int
	for i, op := range ops {
		if op.Op == OpCreateNode {
			slots = append(slots, i)
		}
	}
	if len(slots) < 2 {
		return ops
	}
	creates := make([]StageOp, len(slots))
	byID := map[NodeID]int{}
	for k, s := range slots {
		creates[k] = ops[s]
		if id := ops[s].Create.ID; id != "" {
			byID[id] = k
		}
	}
	deps := func(c StageOp) []int {
		var out []int
		if k, ok := byID[c.Create.ParentID]; ok && c.Create.ParentID != "" {
			out = append(out, k)
		}
		for _, d := range c.Create.DependsOn {
			if k, ok := byID[d]; ok {
				out = append(out, k)
			}
		}
		for _, d := range c.Create.Gates {
			if k, ok := byID[d]; ok {
				out = append(out, k)
			}
		}
		return out
	}
	state := make([]int, len(creates)) // 0 new, 1 visiting, 2 done
	var sorted []StageOp
	var visit func(k int)
	visit = func(k int) {
		if state[k] != 0 {
			return
		}
		state[k] = 1
		for _, d := range deps(creates[k]) {
			visit(d)
		}
		state[k] = 2
		sorted = append(sorted, creates[k])
	}
	for k := range creates {
		visit(k)
	}
	out := append([]StageOp(nil), ops...)
	for k, s := range slots {
		out[s] = sorted[k]
	}
	return out
}
