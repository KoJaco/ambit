package core

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// gatesClearedKey is the local.json key holding the human gates this user has cleared, as
// "<nodeID>#<index>" strings. local.json is gitignored: clearing a gate is a per-user,
// human-only act and has no MCP tool.
const gatesClearedKey = "gates_cleared"

func gateKey(id NodeID, index int) string { return string(id) + "#" + strconv.Itoa(index) }

// gatesClearedLocked returns the set of cleared gate keys. Caller holds idx.mu.
func (idx *Index) gatesClearedLocked() map[string]bool {
	out := map[string]bool{}
	raw, ok := idx.localUnknown[gatesClearedKey]
	if !ok {
		return out
	}
	var list []string
	if json.Unmarshal(raw, &list) != nil {
		return out
	}
	for _, k := range list {
		out[k] = true
	}
	return out
}

func gateStates(n *Node, cleared map[string]bool) []GateState {
	if len(n.HumanGates) == 0 {
		return nil
	}
	out := make([]GateState, len(n.HumanGates))
	for i, g := range n.HumanGates {
		out[i] = GateState{Index: i, Kind: g.Kind, Note: g.Note, Cleared: cleared[gateKey(n.ID, i)]}
	}
	return out
}

// SetGateCleared marks the human gate at index of a node cleared or not, in local.json.
// Entries for nodes or gates that no longer exist are dropped on write.
func (idx *Index) SetGateCleared(id NodeID, index int, cleared bool) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	n, ok := idx.Nodes[id]
	if !ok {
		return fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	if index < 0 || index >= len(n.HumanGates) {
		return fmt.Errorf("%w: node %s has no human gate %d (it has %d)", ErrBadOperation, id, index, len(n.HumanGates))
	}
	set := idx.gatesClearedLocked()
	key := gateKey(id, index)
	if cleared {
		set[key] = true
	} else {
		delete(set, key)
	}
	var list []string
	for _, nid := range idx.Order {
		node := idx.Nodes[nid]
		if node == nil {
			continue
		}
		for i := range node.HumanGates {
			if k := gateKey(nid, i); set[k] {
				list = append(list, k)
			}
		}
	}
	prev, had := idx.localUnknown[gatesClearedKey]
	next := make(map[string]jsonRaw, len(idx.localUnknown)+1)
	for k, v := range idx.localUnknown {
		next[k] = v
	}
	if len(list) == 0 {
		delete(next, gatesClearedKey)
	} else {
		raw, err := rawOf(list)
		if err != nil {
			return err
		}
		next[gatesClearedKey] = raw
	}
	idx.localUnknown = next
	if err := idx.saveLocal(); err != nil {
		if had {
			idx.localUnknown[gatesClearedKey] = prev
		} else {
			delete(idx.localUnknown, gatesClearedKey)
		}
		return err
	}
	return idx.reload()
}

// renameGatesCleared rewrites cleared keys when a node id changes. Caller holds idx.mu and
// saves local.json itself.
func (idx *Index) renameGatesCleared(old, nw NodeID) bool {
	raw, ok := idx.localUnknown[gatesClearedKey]
	if !ok {
		return false
	}
	var list []string
	if json.Unmarshal(raw, &list) != nil {
		return false
	}
	changed := false
	for i, k := range list {
		if strings.HasPrefix(k, string(old)+"#") {
			list[i] = string(nw) + k[len(old):]
			changed = true
		}
	}
	if !changed {
		return false
	}
	out, err := rawOf(list)
	if err != nil {
		return false
	}
	idx.localUnknown[gatesClearedKey] = out
	return true
}
