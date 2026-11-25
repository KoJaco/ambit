package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// contractFingerprint hashes a node's own contract and every relationship contract the
// node is an endpoint of, so a change to any interface the node must honour changes it.
// The caller holds the lock. The result is never empty.
func (idx *Index) contractFingerprint(id NodeID, own *NodeContract) string {
	type edge struct {
		ID       RelID         `json:"id"`
		Contract *EdgeContract `json:"contract"`
	}
	var edges []edge
	for _, r := range idx.Relationships {
		if (r.From == id || r.To == id) && !r.Contract.IsEmpty() {
			edges = append(edges, edge{ID: r.ID, Contract: r.Contract})
		}
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	var node *NodeContract
	if !own.IsEmpty() {
		node = own
	}
	data, _ := json.Marshal(struct {
		Node  *NodeContract `json:"node"`
		Edges []edge        `json:"edges"`
	}{node, edges})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

// baselinesContract reports whether setting this status records the contract baseline.
func baselinesContract(s Status) bool {
	return s == StatusInProgress || s == StatusNeedsReview || s == StatusDone
}

// contractChangedLocked is true when the node recorded a baseline and its contracts
// differ from it now. The caller holds the lock.
func (idx *Index) contractChangedLocked(n *Node) bool {
	if n == nil || n.ContractHash == "" || !baselinesContract(n.Status) {
		return false
	}
	return idx.contractFingerprint(n.ID, n.Contract) != n.ContractHash
}
