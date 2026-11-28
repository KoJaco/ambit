package core

import (
	"fmt"
	"sort"
)

// ContractSyncOptions selects a node and the checkout to look in.
type ContractSyncOptions struct {
	Root     string
	FilesDir string
	NodeID   NodeID
}

// ReferenceState is one doc path a node's spec or contract points at.
type ReferenceState struct {
	Path string `json:"path"`
	// Source is "references" (the node's references list) or "contract" (the ref of a
	// relationship contract the node takes part in).
	Source string `json:"source"`
	Exists bool   `json:"exists"`
	Status string `json:"status"`
}

// ContractSyncReport is CheckContractSync's result.
type ContractSyncReport struct {
	NodeID NodeID `json:"node_id"`
	Pass   bool   `json:"pass"`
	// HasContract is true when the node or one of its relationships carries a contract.
	HasContract bool `json:"has_contract"`
	// ContractChanged is true when a contract the node must honour changed after its status
	// was last set to in_progress or done.
	ContractChanged bool             `json:"contract_changed"`
	References      []ReferenceState `json:"references"`
	Problems        []string         `json:"problems"`
}

// CheckContractSync compares a node with the docs it points at: every path in references,
// and the ref of every relationship contract touching the node, must exist and be non-empty
// in the checkout, and the contract must not have changed since the node was last marked
// in_progress or done (then the docs may be stale). It reads files and never writes.
func CheckContractSync(idx *Index, opts ContractSyncOptions) (ContractSyncReport, error) {
	rep := ContractSyncReport{NodeID: opts.NodeID, References: []ReferenceState{}, Problems: []string{}}
	dir, err := checkoutDir(opts.Root, opts.FilesDir)
	if err != nil {
		return rep, err
	}
	idx.mu.Lock()
	n := idx.Nodes[opts.NodeID]
	if n == nil {
		idx.mu.Unlock()
		return rep, fmt.Errorf("%w: %q", ErrNotFound, opts.NodeID)
	}
	rep.HasContract = !n.Contract.IsEmpty()
	rep.ContractChanged = idx.contractChangedLocked(n)
	type ref struct{ path, source string }
	var refs []ref
	seen := map[string]bool{}
	for _, p := range n.References {
		if !seen[p] {
			seen[p] = true
			refs = append(refs, ref{p, "references"})
		}
	}
	for _, rel := range idx.Relationships {
		if rel.From != n.ID && rel.To != n.ID {
			continue
		}
		if !rel.Contract.IsEmpty() {
			rep.HasContract = true
		}
		if rel.Contract != nil && rel.Contract.Ref != "" && !seen[rel.Contract.Ref] {
			seen[rel.Contract.Ref] = true
			refs = append(refs, ref{rel.Contract.Ref, "contract"})
		}
	}
	idx.mu.Unlock()
	sort.SliceStable(refs, func(i, j int) bool { return refs[i].source < refs[j].source })
	for _, r := range refs {
		_, status := deliverableState(dir, r.path)
		st := ReferenceState{Path: r.path, Source: r.source, Exists: status == DeliverableOK || status == DeliverableEmpty, Status: status}
		switch status {
		case DeliverableMissing:
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s %s does not exist", r.source, r.path))
		case DeliverableEmpty:
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s %s is empty", r.source, r.path))
		case DeliverableUnsafe:
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s %s is not a path inside the repository", r.source, r.path))
		}
		rep.References = append(rep.References, st)
	}
	if rep.ContractChanged {
		rep.Problems = append(rep.Problems, "a contract this node must honour changed after its status was last set; re-check the referenced docs and the implementation")
	}
	rep.Pass = len(rep.Problems) == 0
	return rep, nil
}
