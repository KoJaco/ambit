package core

import (
	"fmt"
	"sort"
)

// Build item classes.
const (
	ClassAgentParallel = "agent-parallel"
	ClassNeedsHuman    = "needs-human"
	ClassBlocked       = "blocked"
)

// GateState is one human gate of a node with its machine-local cleared flag.
type GateState struct {
	Index   int    `json:"index"`
	Kind    string `json:"kind"`
	Note    string `json:"note,omitempty"`
	Cleared bool   `json:"cleared"`
}

// BuildItem is one buildable node in a build order.
type BuildItem struct {
	ID     NodeID `json:"id"`
	Name   string `json:"name"`
	Status Status `json:"status"`
	// Done is true when the node counts as done (a container is done when its children are).
	Done bool `json:"done"`
	// Wave is the 1-based longest-path layer; nodes in one wave can be built in parallel.
	Wave int `json:"wave"`
	// Class is agent-parallel (an agent can build it once unblocked), needs-human (it has
	// human gates not yet cleared) or blocked (an unfinished dependency, or status blocked).
	Class string `json:"class"`
	// DependsOn is the node's declared depends_on, ordered by model order.
	DependsOn []NodeID `json:"depends_on"`
	// ContractChanged is true when the node's contract, or one of its relationship
	// contracts, changed after the node was marked in_progress or done.
	ContractChanged bool `json:"contract_changed,omitempty"`
	// BlockedBy lists declared dependencies (own, those cascaded from non-buildable
	// container ancestors, and spikes that gate the node) that are not done. External
	// nodes never block.
	BlockedBy []NodeID `json:"blocked_by"`
	// Gates lists the nodes this node gates (a spike); GatedBy lists the spikes gating it.
	Gates   []NodeID `json:"gates,omitempty"`
	GatedBy []NodeID `json:"gated_by,omitempty"`
	// HumanGates is the node's human gates with their cleared state.
	HumanGates []GateState `json:"human_gates,omitempty"`
}

// ExternalItem is a node flagged external: outside the build, so not in order or waves.
type ExternalItem struct {
	ID         NodeID      `json:"id"`
	Name       string      `json:"name"`
	Status     Status      `json:"status"`
	HumanGates []GateState `json:"human_gates,omitempty"`
}

// BuildOrderOptions selects the shape of a build order.
type BuildOrderOptions struct {
	// Scope limits the order to a subtree ("" is the whole model).
	Scope NodeID
	// Compact keeps only id, class and wave per item (see CompactBuildOrder). Only
	// BuildOrderView honours it; BuildOrderWith always returns the full result.
	Compact bool
	// ReadyOnly keeps only the items in ready_now, and returns no waves.
	ReadyOnly bool
}

// CompactItem is a build item reduced to what scheduling needs.
type CompactItem struct {
	ID    NodeID `json:"id"`
	Class string `json:"class"`
	Wave  int    `json:"wave"`
	// Done is only present when true.
	Done bool `json:"done,omitempty"`
}

// CompactBuildOrder is BuildOrderResult with each item reduced to id, class and wave, for
// large models (roughly a tenth of the size).
type CompactBuildOrder struct {
	Scope       NodeID        `json:"scope,omitempty"`
	Order       []CompactItem `json:"order"`
	Waves       [][]NodeID    `json:"waves"`
	Cycles      [][]NodeID    `json:"cycles"`
	Unordered   []NodeID      `json:"unordered"`
	Externals   []NodeID      `json:"externals"`
	ReadyNow    []NodeID      `json:"ready_now"`
	MaxParallel int           `json:"max_parallel"`
	ParallelSet []NodeID      `json:"parallel_set"`
}

// Compact reduces the result to a CompactBuildOrder.
func (r BuildOrderResult) Compact() CompactBuildOrder {
	c := CompactBuildOrder{Scope: r.Scope, Order: make([]CompactItem, 0, len(r.Order)), Waves: r.Waves, Cycles: r.Cycles,
		Unordered: r.Unordered, Externals: []NodeID{}, ReadyNow: r.ReadyNow, MaxParallel: r.MaxParallel, ParallelSet: r.ParallelSet}
	for _, it := range r.Order {
		c.Order = append(c.Order, CompactItem{ID: it.ID, Class: it.Class, Wave: it.Wave, Done: it.Done})
	}
	for _, e := range r.Externals {
		c.Externals = append(c.Externals, e.ID)
	}
	return c
}

// BuildOrderView returns the build order as the wire value for the given options: a
// CompactBuildOrder when opts.Compact, otherwise the full BuildOrderResult.
func BuildOrderView(idx *Index, opts BuildOrderOptions) (any, error) {
	res, err := BuildOrderWith(idx, opts)
	if err != nil {
		return nil, err
	}
	if opts.Compact {
		return res.Compact(), nil
	}
	return res, nil
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
	// Externals lists in-scope nodes flagged external, in model order.
	Externals []ExternalItem `json:"externals"`
	// ReadyNow lists the ordered nodes that are not blocked, not done and not external.
	ReadyNow []NodeID `json:"ready_now"`
	// MaxParallel is the size of ParallelSet: a greedy (model order) packing of ReadyNow
	// into nodes whose implementation globs do not overlap, so they can be built at once.
	MaxParallel int      `json:"max_parallel"`
	ParallelSet []NodeID `json:"parallel_set"`
}

// BuildOrder derives the build order from depends_on. scope "" means the whole model;
// otherwise only buildable nodes in that node's subtree (itself included) are ordered.
// Dependencies on nodes outside the scope still count in blocked_by. Advisory only.
func BuildOrder(idx *Index, scope NodeID) (BuildOrderResult, error) {
	return BuildOrderWith(idx, BuildOrderOptions{Scope: scope})
}

// BuildOrderWith is BuildOrder with the ReadyOnly option applied.
func BuildOrderWith(idx *Index, opts BuildOrderOptions) (BuildOrderResult, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	res, err := idx.buildOrderLocked(opts.Scope)
	if err != nil {
		return res, err
	}
	if opts.ReadyOnly {
		ready := map[NodeID]bool{}
		for _, id := range res.ReadyNow {
			ready[id] = true
		}
		kept := []BuildItem{}
		for _, it := range res.Order {
			if ready[it.ID] {
				kept = append(kept, it)
			}
		}
		res.Order = kept
		res.Waves = [][]NodeID{}
	}
	return res, nil
}

func (idx *Index) buildOrderLocked(scope NodeID) (BuildOrderResult, error) {
	res := BuildOrderResult{Scope: scope, Order: []BuildItem{}, Waves: [][]NodeID{}, Cycles: [][]NodeID{}, Unordered: []NodeID{},
		Externals: []ExternalItem{}, ReadyNow: []NodeID{}, ParallelSet: []NodeID{}}
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
	g := newDepGraph(idx.Nodes, idx.Children)
	buildable := g.buildable

	done := g.doneFn()

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
	cleared := idx.gatesClearedLocked()
	var members []NodeID
	for _, id := range order {
		if !inScope[id] {
			continue
		}
		if n := idx.Nodes[id]; n.External {
			// Outside the build: listed on its own, never ordered, never blocking.
			delete(inScope, id)
			res.Externals = append(res.Externals, ExternalItem{ID: id, Name: n.Name, Status: n.Status, HumanGates: gateStates(n, cleared)})
			continue
		}
		members = append(members, id)
	}

	// Edges t -> b (t before b), restricted to the scope.
	succ, preds, indeg, decl := g.succ(members, inScope, func(l []NodeID) []NodeID { return sortByPos(l, pos) })

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
		item := BuildItem{ID: b, Name: n.Name, Status: n.Status, Done: done(b), ContractChanged: idx.contractChangedLocked(n), Wave: w, DependsOn: []NodeID{}, BlockedBy: []NodeID{}}
		for _, d := range sortByPos(n.DependsOn, pos) {
			item.DependsOn = append(item.DependsOn, d)
		}
		for _, d := range decl[b] {
			if !done(d) && !idx.Nodes[d].External {
				item.BlockedBy = append(item.BlockedBy, d)
			}
		}
		if len(n.Gates) > 0 {
			item.Gates = sortByPos(n.Gates, pos)
		}
		item.GatedBy = sortByPos(g.gaters[b], pos)
		item.HumanGates = gateStates(n, cleared)
		item.Class = classOf(item)
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
	for _, it := range res.Order {
		if !it.Done && it.Class != ClassBlocked {
			res.ReadyNow = append(res.ReadyNow, it.ID)
		}
	}
	res.ParallelSet = packParallel(idx, res.ReadyNow)
	res.MaxParallel = len(res.ParallelSet)
	return res, nil
}

// classOf classifies an item: blocked wins over needs-human, which wins over agent-parallel.
func classOf(it BuildItem) string {
	if len(it.BlockedBy) > 0 || it.Status == StatusBlocked {
		return ClassBlocked
	}
	for _, g := range it.HumanGates {
		if !g.Cleared {
			return ClassNeedsHuman
		}
	}
	return ClassAgentParallel
}

// packParallel greedily (in the given order) picks nodes whose implementation globs do not
// overlap any already picked node's, so they can be built at the same time.
func packParallel(idx *Index, ids []NodeID) []NodeID {
	set := []NodeID{}
	for _, id := range ids {
		n := idx.Nodes[id]
		clash := false
		for _, other := range set {
			if globSetsOverlap(n.Implementation, idx.Nodes[other].Implementation) {
				clash = true
				break
			}
		}
		if !clash {
			set = append(set, id)
		}
	}
	return set
}

// globSetsOverlap reports whether some path can match a glob of a and a glob of b. It is
// the same sample-path approximation the glob-overlap lint uses, applied in both directions.
func globSetsOverlap(a, b []string) bool {
	for _, pair := range [][2][]string{{a, b}, {b, a}} {
		for _, g := range pair[0] {
			sample := globSample(g)
			if sample == "" {
				continue
			}
			if ok, err := matchGlobs(pair[1], sample); err == nil && ok {
				return true
			}
		}
	}
	return false
}

// depGraph is the depends_on graph with pure containers expanded: a dependency on a pure
// container means its buildable descendants, and a pure container's depends_on applies to
// each buildable node under it. Build order and mutation-time cycle checks share it.
type depGraph struct {
	nodes    map[NodeID]*Node
	children map[NodeID][]NodeID
	gaters   map[NodeID][]NodeID
}

// newDepGraphFromNodes derives children from ParentID, ordered by id.
func newDepGraphFromNodes(nodes map[NodeID]*Node) depGraph {
	ids := make([]NodeID, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sortNodeIDs(ids)
	children := map[NodeID][]NodeID{}
	for _, id := range ids {
		n := nodes[id]
		if n == nil || n.ParentID == "" {
			continue
		}
		if _, ok := nodes[n.ParentID]; ok {
			children[n.ParentID] = append(children[n.ParentID], id)
		}
	}
	return newDepGraph(nodes, children)
}

// newDepGraph builds the graph and the reverse index of gates: gaters[x] lists the nodes
// whose gates list names x, so x effectively depends on each of them.
func newDepGraph(nodes map[NodeID]*Node, children map[NodeID][]NodeID) depGraph {
	gaters := map[NodeID][]NodeID{}
	ids := make([]NodeID, 0, len(nodes))
	for id, n := range nodes {
		if n != nil && len(n.Gates) > 0 {
			ids = append(ids, id)
		}
	}
	sortNodeIDs(ids)
	for _, id := range ids {
		for _, t := range nodes[id].Gates {
			if _, ok := nodes[t]; ok && t != id {
				gaters[t] = append(gaters[t], id)
			}
		}
	}
	return depGraph{nodes: nodes, children: children, gaters: gaters}
}

// buildable: a leaf, or a container with implementation globs.
func (g depGraph) buildable(id NodeID) bool {
	return len(g.children[id]) == 0 || len(g.nodes[id].Implementation) > 0
}

// targets lists the buildable nodes that must finish for a dependency on id.
func (g depGraph) targets(id NodeID) []NodeID {
	var out []NodeID
	if g.buildable(id) {
		out = append(out, id)
	}
	for _, k := range g.children[id] {
		out = append(out, g.targets(k)...)
	}
	return out
}

// doneFn returns the done rule: a node with children is done when all its children are
// (its own status is ignored), otherwise when its status is done.
func (g depGraph) doneFn() func(NodeID) bool {
	memo := map[NodeID]bool{}
	var done func(id NodeID) bool
	done = func(id NodeID) bool {
		if v, ok := memo[id]; ok {
			return v
		}
		kids := g.children[id]
		v := g.nodes[id].Status == StatusDone
		if len(kids) > 0 {
			v = true
			for _, k := range kids {
				if !done(k) {
					v = false
					break
				}
			}
		}
		memo[id] = v
		return v
	}
	return done
}

// declared is b's own depends_on and gaters (spikes that gate it) plus that of its non-buildable ancestors (existing nodes
// only, unordered).
func (g depGraph) declared(b NodeID) []NodeID {
	seen := map[NodeID]bool{}
	var out []NodeID
	add := func(list []NodeID) {
		for _, d := range list {
			if _, ok := g.nodes[d]; ok && !seen[d] {
				seen[d] = true
				out = append(out, d)
			}
		}
	}
	add(g.nodes[b].DependsOn)
	add(g.gaters[b])
	visited := map[NodeID]bool{b: true}
	for cur := g.nodes[b].ParentID; cur != ""; {
		p := g.nodes[cur]
		if p == nil || visited[cur] {
			break
		}
		visited[cur] = true
		if !g.buildable(cur) {
			add(p.DependsOn)
			add(g.gaters[cur])
		}
		cur = p.ParentID
	}
	return out
}

// succ returns edges t -> b (t must be built before b) over the given buildable nodes.
func (g depGraph) succ(members []NodeID, inScope map[NodeID]bool, sortDeclared func([]NodeID) []NodeID) (succ, preds map[NodeID][]NodeID, indeg map[NodeID]int, decl map[NodeID][]NodeID) {
	succ = map[NodeID][]NodeID{}
	preds = map[NodeID][]NodeID{}
	indeg = map[NodeID]int{}
	decl = map[NodeID][]NodeID{}
	for _, b := range members {
		decl[b] = g.declared(b)
		if sortDeclared != nil {
			decl[b] = sortDeclared(decl[b])
		}
		seen := map[NodeID]bool{}
		for _, d := range decl[b] {
			for _, t := range g.targets(d) {
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
	return
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
