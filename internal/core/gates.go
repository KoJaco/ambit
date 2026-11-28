package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// gatesClearedKey is the local.json key holding the human gates this user has cleared, as
// "<nodeID>#<hash>" strings, where hash covers the gate's kind and note (so reordering or
// editing human_gates never moves a cleared state onto a different gate; identical gates on
// one node get a "~n" occurrence suffix). Legacy "<nodeID>#<index>" entries match nothing and
// are dropped on the next write. local.json is gitignored: clearing a gate is a per-user,
// human-only act and has no MCP tool.
const gatesClearedKey = "gates_cleared"

// gateKeys returns the cleared-state key of every human gate of n, in order.
func gateKeys(n *Node) []string {
	keys := make([]string, len(n.HumanGates))
	seen := map[string]int{}
	for i, g := range n.HumanGates {
		sum := sha256.Sum256([]byte(g.Kind + "\x00" + g.Note))
		h := hex.EncodeToString(sum[:6])
		k := string(n.ID) + "#" + h
		if c := seen[h]; c > 0 {
			k += "~" + strconv.Itoa(c)
		}
		seen[h]++
		keys[i] = k
	}
	return keys
}

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
	keys := gateKeys(n)
	for i, g := range n.HumanGates {
		out[i] = GateState{Index: i, Kind: g.Kind, Note: g.Note, Cleared: cleared[keys[i]]}
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
	key := gateKeys(n)[index]
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
		for _, k := range gateKeys(node) {
			if set[k] {
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

// dropGatesCleared removes every cleared entry of the given (deleted) nodes and saves
// local.json. Caller holds idx.mu.
func (idx *Index) dropGatesCleared(ids map[NodeID]bool) error {
	raw, ok := idx.localUnknown[gatesClearedKey]
	if !ok {
		return nil
	}
	var list []string
	if json.Unmarshal(raw, &list) != nil {
		return nil
	}
	var keep []string
	for _, k := range list {
		i := strings.LastIndex(k, "#")
		if i >= 0 && ids[NodeID(k[:i])] {
			continue
		}
		keep = append(keep, k)
	}
	if len(keep) == len(list) {
		return nil
	}
	if len(keep) == 0 {
		delete(idx.localUnknown, gatesClearedKey)
	} else {
		out, err := rawOf(keep)
		if err != nil {
			return err
		}
		idx.localUnknown[gatesClearedKey] = out
	}
	return idx.saveLocal()
}
