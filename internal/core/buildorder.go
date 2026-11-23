package core

import (
	"fmt"
	"sort"
)

// BuildItem is one buildable node in a build order.
type BuildItem struct {
	ID     NodeID `json:"id"`
	Name   string `json:"name"`
	Status Status `json:"status"`
	// Done is true when the node counts as done (a container is done when its children are).
	Done bool `json:"done"`
	// Wave is the 1-based longest-path layer; nodes in one wave can be built in parallel.
	Wave int `json:"wave"`
	// DependsOn is the node's declared depends_on, ordered by model order.
	DependsOn []NodeID `json:"depends_on"`
	// BlockedBy lists declared dependencies (own, and those cascaded from non-buildable
	// container ancestors) that are not done.
	BlockedBy []NodeID `json:"blocked_by"`
}

// BuildOrderResult is the advisory build order for a model or subtree.
type BuildOrderResult struct {
	Scope NodeID      `json:"scope,omitempty"`
	Order []BuildItem `json:"order"`
	Waves [][]NodeID  `json:"waves"`
	// Cycles lists each dependency cycle among the in-scope buildable nodes.
	Cycles [][]NodeID `json:"cycles"`
	// Unordered lists in-scope buildable nodes that could not be ordered: cycle members and
	// anything that depends on them.
	Unordered []NodeID `json:"unordered"`
}

// BuildOrder derives the build order from depends_on. scope "" means the whole model;
// otherwise only buildable nodes in that node's subtree (itself included) are ordered.
// Dependencies on nodes outside the scope still count in blocked_by. Advisory only.
func BuildOrder(idx *Index, scope NodeID) (BuildOrderResult, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.buildOrderLocked(scope)
}

func (idx *Index) buildOrderLocked(scope NodeID) (BuildOrderResult, error) {
	res := BuildOrderResult{Scope: scope, Order: []BuildItem{}, Waves: [][]NodeID{}, Cycles: [][]NodeID{}, Unordered: []NodeID{}}
	if scope != "" {
		if _, ok := idx.Nodes[scope]; !ok {
			return res, fmt.Errorf("%w: %q", ErrNotFound, scope)
		}
	}
	var order []NodeID
	pos := map[NodeID]int{}
	for _, id := range idx.Order {
		if idx.Nodes[id] != nil {
			pos[id] = len(order)
			order = append(order, id)
		}
	}
	buildable := func(id NodeID) bool {
		return len(idx.Children[id]) == 0 || len(idx.Nodes[id].Implementation) > 0
	}

	doneMemo := map[NodeID]bool{}
	var done func(id NodeID) bool
	done = func(id NodeID) bool {
		if v, ok := doneMemo[id]; ok {
			return v
		}
		kids := idx.Children[id]
		v := idx.Nodes[id].Status == StatusDone
		if len(kids) > 0 {
			v = true
			for _, k := range kids {
				if !done(k) {
					v = false
					break
				}
			}
		}
		doneMemo[id] = v
		return v
	}

	// targets(d): the buildable nodes that must finish for a dependency on d.
	var targets func(id NodeID) []NodeID
	targets = func(id NodeID) []NodeID {
		var out []NodeID
		if buildable(id) {
			out = append(out, id)
		}
		for _, k := range idx.Children[id] {
			out = append(out, targets(k)...)
		}
		return out
	}

	// declared(b): own depends_on plus depends_on of non-buildable ancestors, existing nodes only.
	declared := func(b NodeID) []NodeID {
		seen := map[NodeID]bool{}
		var out []NodeID
		add := func(list []NodeID) {
			for _, d := range list {
				if _, ok := idx.Nodes[d]; ok && !seen[d] {
					seen[d] = true
					out = append(out, d)
				}
			}
		}
		add(idx.Nodes[b].DependsOn)
		visited := map[NodeID]bool{b: true}
		for cur := idx.Nodes[b].ParentID; cur != ""; {
			p := idx.Nodes[cur]
			if p == nil || visited[cur] {
				break
			}
			visited[cur] = true
			if !buildable(cur) {
				add(p.DependsOn)
			}
			cur = p.ParentID
		}
		sort.Slice(out, func(i, j int) bool { return pos[out[i]] < pos[out[j]] })
		return out
	}

	// Scope set.
	inScope := map[NodeID]bool{}
	var collect func(id NodeID)
	collect = func(id NodeID) {
		if buildable(id) {
			inScope[id] = true
		}
		for _, k := range idx.Children[id] {
			collect(k)
		}
	}
	if scope != "" {
		collect(scope)
	} else {
		for _, id := range order {
			if buildable(id) {
				inScope[id] = true
			}
		}
	}
	var members []NodeID
	for _, id := range order {
		if inScope[id] {
			members = append(members, id)
		}
	}

	// Edges t -> b (t before b), restricted to the scope.
	succ := map[NodeID][]NodeID{}
	preds := map[NodeID][]NodeID{}
	indeg := map[NodeID]int{}
	decl := map[NodeID][]NodeID{}
	for _, b := range members {
		decl[b] = declared(b)
		seen := map[NodeID]bool{}
		for _, d := range decl[b] {
			for _, t := range targets(d) {
				if !inScope[t] || seen[t] || (t == b && d != b) {
					continue
				}
				seen[t] = true
				succ[t] = append(succ[t], b)
				preds[b] = append(preds[b], t)
				indeg[b]++
			}
		}
	}

	res.Cycles = append(res.Cycles, sccCycles(members, succ)...)

	// Kahn's algorithm; ties broken by model order.
	wave := map[NodeID]int{}
	processed := map[NodeID]bool{}
	var ready []NodeID
	for _, b := range members {
		if indeg[b] == 0 {
			ready = append(ready, b)
		}
	}
	for len(ready) > 0 {
		best := 0
		for i := range ready {
			if pos[ready[i]] < pos[ready[best]] {
				best = i
			}
		}
		b := ready[best]
		ready = append(ready[:best], ready[best+1:]...)
		processed[b] = true
		w := 1
		for _, p := range preds[b] {
			if wave[p]+1 > w {
				w = wave[p] + 1
			}
		}
		wave[b] = w
		n := idx.Nodes[b]
		item := BuildItem{ID: b, Name: n.Name, Status: n.Status, Done: done(b), Wave: w, DependsOn: []NodeID{}, BlockedBy: []NodeID{}}
		for _, d := range sortByPos(n.DependsOn, pos) {
			item.DependsOn = append(item.DependsOn, d)
		}
		for _, d := range decl[b] {
			if !done(d) {
				item.BlockedBy = append(item.BlockedBy, d)
			}
		}
		res.Order = append(res.Order, item)
		for len(res.Waves) < w {
			res.Waves = append(res.Waves, []NodeID{})
		}
		res.Waves[w-1] = append(res.Waves[w-1], b)
		for _, s := range succ[b] {
			indeg[s]--
			if indeg[s] == 0 {
				ready = append(ready, s)
			}
		}
	}
	for _, w := range res.Waves {
		sort.Slice(w, func(i, j int) bool { return pos[w[i]] < pos[w[j]] })
	}
	for _, b := range members {
		if !processed[b] {
			res.Unordered = append(res.Unordered, b)
		}
	}
	return res, nil
}

// sortByPos returns the existing ids of in ordered by model position.
func sortByPos(in []NodeID, pos map[NodeID]int) []NodeID {
	var out []NodeID
	for _, d := range in {
		if _, ok := pos[d]; ok {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return pos[out[i]] < pos[out[j]] })
	return out
}
