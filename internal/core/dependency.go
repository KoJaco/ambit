package core

import (
	"fmt"
	"sort"
	"strings"
)

func sortedDeps(in []NodeID) []NodeID {
	if len(in) == 0 {
		return nil
	}
	out := append([]NodeID{}, in...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// validateDependsOn checks a proposed depends_on list for self. self may not be in
// idx.Nodes yet (create). Ids must exist, no self-reference, no duplicates, no cycle.
func (idx *Index) validateDependsOn(self, parent NodeID, deps []NodeID) error {
	return validateDependsOnIn(idx.Nodes, self, parent, deps)
}

// validateDependsOnIn is validateDependsOn against an explicit node map, so staging can
// validate against a scratch copy that already holds earlier operations of a proposal.
func validateDependsOnIn(nodes map[NodeID]*Node, self, parent NodeID, deps []NodeID) error {
	seen := map[NodeID]bool{}
	for _, d := range deps {
		switch {
		case d == self:
			return fmt.Errorf("%w: node %s cannot depend on itself", ErrBadDependency, self)
		case seen[d]:
			return fmt.Errorf("%w: %q listed more than once", ErrBadDependency, d)
		}
		seen[d] = true
		if _, ok := nodes[d]; !ok {
			return fmt.Errorf("%w: %q does not exist", ErrBadDependency, d)
		}
	}
	visited := map[NodeID]bool{}
	var walk func(cur NodeID, path []NodeID) []NodeID
	walk = func(cur NodeID, path []NodeID) []NodeID {
		path = append(path, cur)
		if cur == self {
			return path
		}
		if visited[cur] {
			return nil
		}
		visited[cur] = true
		n := nodes[cur]
		if n == nil {
			return nil
		}
		for _, next := range sortedDeps(n.DependsOn) {
			if p := walk(next, path); p != nil {
				return p
			}
		}
		return nil
	}
	for _, d := range sortedDeps(deps) {
		if p := walk(d, []NodeID{self}); p != nil {
			parts := make([]string, len(p))
			for i, id := range p {
				parts[i] = string(id)
			}
			return fmt.Errorf("%w: %s", ErrDependencyCycle, strings.Join(parts, " -> "))
		}
	}
	return expandedCycleCheck(nodes, self, parent, deps)
}

// expandedCycleCheck rejects deps when, with pure containers expanded exactly as the build
// order does, they put self (or a node under it) in a dependency cycle. A dependency on a
// pure container means its descendants, so x -> container -> child -> x is a cycle even
// though the raw depends_on lists are acyclic.
func expandedCycleCheck(nodes map[NodeID]*Node, self, parent NodeID, deps []NodeID) error {
	scratch := make(map[NodeID]*Node, len(nodes)+1)
	for id, n := range nodes {
		scratch[id] = n
	}
	sn := &Node{ID: self}
	if cur := nodes[self]; cur != nil {
		cp := *cur
		sn = &cp
	}
	sn.ParentID = parent
	sn.DependsOn = deps
	scratch[self] = sn
	g := newDepGraphFromNodes(scratch)

	mine := map[NodeID]bool{}
	var collect func(id NodeID)
	collect = func(id NodeID) {
		mine[id] = true
		for _, k := range g.children[id] {
			collect(k)
		}
	}
	collect(self)

	var members []NodeID
	inScope := map[NodeID]bool{}
	for id := range scratch {
		if scratch[id] != nil && g.buildable(id) {
			members = append(members, id)
			inScope[id] = true
		}
	}
	sortNodeIDs(members)
	succ, _, _, _ := g.succ(members, inScope, nil)
	for _, comp := range sccCycles(members, succ) {
		for _, id := range comp {
			if mine[id] {
				names := make([]string, len(comp))
				for i, c := range comp {
					names[i] = string(c)
				}
				return fmt.Errorf("%w: through containers among %s", ErrDependencyCycle, strings.Join(names, ", "))
			}
		}
	}
	return nil
}

// dependentsOutside returns "Name (id)" for every node outside remove whose depends_on
// names a node in remove, sorted by id.
func (idx *Index) dependentsOutside(remove map[NodeID]bool) []string {
	return dependentsOutsideIn(idx.Nodes, remove)
}

func dependentsOutsideIn(nodes map[NodeID]*Node, remove map[NodeID]bool) []string {
	var ids []NodeID
	for id, n := range nodes {
		if n == nil || remove[id] {
			continue
		}
		for _, d := range n.DependsOn {
			if remove[d] {
				ids = append(ids, id)
				break
			}
		}
	}
	sortNodeIDs(ids)
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = fmt.Sprintf("%s (%s)", nodes[id].Name, id)
	}
	return out
}

func (idx *Index) checkNoDependents(remove map[NodeID]bool) error {
	return checkNoDependentsIn(idx.Nodes, remove)
}

func checkNoDependentsIn(nodes map[NodeID]*Node, remove map[NodeID]bool) error {
	deps := dependentsOutsideIn(nodes, remove)
	if len(deps) == 0 {
		return nil
	}
	var targets []NodeID
	for id := range remove {
		targets = append(targets, id)
	}
	sortNodeIDs(targets)
	names := make([]string, len(targets))
	for i, t := range targets {
		names[i] = string(t)
	}
	return fmt.Errorf("%w: %s is depended on by: %s", ErrHasDependents, strings.Join(names, ", "), strings.Join(deps, ", "))
}

// sccCycles returns the strongly connected components of adj that are cycles (more than one
// node, or a self-edge). Members are ordered by their position in order; components are
// ordered by first member. Nodes not in order are ignored.
func sccCycles(order []NodeID, adj map[NodeID][]NodeID) [][]NodeID {
	pos := make(map[NodeID]int, len(order))
	for i, id := range order {
		pos[id] = i
	}
	index := map[NodeID]int{}
	low := map[NodeID]int{}
	on := map[NodeID]bool{}
	var stack []NodeID
	next := 0
	var out [][]NodeID
	var strong func(v NodeID)
	strong = func(v NodeID) {
		index[v] = next
		low[v] = next
		next++
		stack = append(stack, v)
		on[v] = true
		for _, w := range adj[v] {
			if _, ok := pos[w]; !ok {
				continue
			}
			if _, seen := index[w]; !seen {
				strong(w)
				if low[w] < low[v] {
					low[v] = low[w]
				}
			} else if on[w] && index[w] < low[v] {
				low[v] = index[w]
			}
		}
		if low[v] != index[v] {
			return
		}
		var comp []NodeID
		for {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			on[w] = false
			comp = append(comp, w)
			if w == v {
				break
			}
		}
		cyclic := len(comp) > 1
		if !cyclic {
			for _, w := range adj[v] {
				if w == v {
					cyclic = true
				}
			}
		}
		if cyclic {
			sort.Slice(comp, func(i, j int) bool { return pos[comp[i]] < pos[comp[j]] })
			out = append(out, comp)
		}
	}
	for _, id := range order {
		if _, seen := index[id]; !seen {
			strong(id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return pos[out[i][0]] < pos[out[j][0]] })
	return out
}

// linkDependencies reports dangling depends_on entries (warning) and dependency cycles
// (error) found on load. Neither fails the load.
func (idx *Index) linkDependencies() {
	adj := map[NodeID][]NodeID{}
	var order []NodeID
	for _, id := range idx.Order {
		n := idx.Nodes[id]
		if n == nil {
			continue
		}
		order = append(order, id)
		for _, d := range n.DependsOn {
			if _, ok := idx.Nodes[d]; !ok {
				idx.warn(idx.nodeJSON(id), fmt.Sprintf("node %s depends_on %q which does not exist", id, d))
				continue
			}
			adj[id] = append(adj[id], d)
		}
	}
	for _, comp := range sccCycles(order, adj) {
		names := make([]string, len(comp))
		for i, id := range comp {
			names[i] = string(id)
		}
		msg := fmt.Sprintf("dependency cycle among: %s", strings.Join(names, ", "))
		if len(comp) == 1 {
			msg = fmt.Sprintf("node %s depends on itself", comp[0])
		}
		idx.fail(idx.nodeJSON(comp[0]), msg)
	}
}
