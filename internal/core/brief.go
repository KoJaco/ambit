package core

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// siblingContractCap is the per-sibling character budget for contract text in a brief.
const siblingContractCap = 800

// BriefClosing is the last line of every brief.
const BriefClosing = "When you are done, call check_drift to confirm the model and code agree, then call update_node_status for your node."

// ShallowModelGuidance is the one-line modeling rule shared by the brief and the MCP
// authoring tool descriptions (decision 0010).
const ShallowModelGuidance = "Prefer shallow models. Use a container node to group related nodes. Nest more than two levels only for genuinely separate subsystems."

// AuthoringGuidance is the model-first workflow guidance shared by the brief and the MCP
// authoring tool descriptions.
const AuthoringGuidance = "Model high-level first: start with the few top-level parts, then drill down only where asked. " +
	"Treat container nodes as deployment zones (for example browser, edge, API, data) and keep nesting to about two levels. " +
	"Before drilling down, ask the user how they want the top-level view organised. " +
	"Do not add an edge that carries no information (every-pair wiring, or what the hierarchy already says). " +
	"Set explicit ids on nodes you create so later calls can reference them in parent_id and relationships. " +
	"Call list_icons to pick icon ids and get_graph to see what already exists."

// Brief returns an imperative task description for an assigned agent working on id.
func (idx *Index) Brief(id NodeID) (string, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	n, ok := idx.Nodes[id]
	if !ok || n == nil {
		return "", fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	var b strings.Builder
	b.WriteString(idx.methodPreambleLocked())
	fmt.Fprintf(&b, "You are working on the architecture node %q (%s).\n\n", n.Name, id)

	allowed := EffectiveScope(n)
	fmt.Fprintf(&b, "You may modify only files matching these globs:\n")
	for _, g := range allowed {
		fmt.Fprintf(&b, "  - %s\n", g)
	}
	if len(allowed) == 0 {
		fmt.Fprintf(&b, "  (none declared — stop and report back; do not guess paths.)\n")
	}

	fmt.Fprintf(&b, "\nDo not modify anything outside those globs. If the task appears to require more access, stop and report back rather than pushing through.\n")

	var protectedLines []string
	for _, oid := range idx.Order {
		p := idx.Nodes[oid]
		if p == nil || !p.Protected || len(p.Implementation) == 0 {
			continue
		}
		protectedLines = append(protectedLines, fmt.Sprintf("  - node %q (%s): %s",
			p.Name, oid, strings.Join(p.Implementation, ", ")))
	}
	if len(protectedLines) > 0 {
		fmt.Fprintf(&b, "\nThese paths are off-limits (protected nodes):\n")
		for _, line := range protectedLines {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}

	siblings := siblingNodes(idx, n)
	if len(siblings) > 0 {
		fmt.Fprintf(&b, "\nSibling nodes (interfaces only — do not edit their implementation):\n")
		for _, s := range siblings {
			fmt.Fprintf(&b, "  - %q (%s, %s)\n", s.Name, s.ID, s.Type)
			if strings.TrimSpace(s.Spec) != "" {
				spec := strings.TrimSpace(s.Spec)
				if len(spec) > 400 {
					spec = spec[:400] + "..."
				}
				b.WriteString(indentLines(spec, "    "))
				b.WriteByte('\n')
			}
			if text := siblingContractText(s.Contract); text != "" {
				b.WriteString(indentLines(text, "    "))
				b.WriteByte('\n')
			}
		}
	}

	idx.writeProjectRules(&b)
	idx.writeInterfaces(&b, n)
	writeOwnContract(&b, n)
	idx.writeBuildOrder(&b, n)

	fmt.Fprintf(&b, "\nCall the check_scope tool with node_id %q periodically while you work, and again before you finish.\n", id)

	fmt.Fprintf(&b, "\nIf you propose changes to the architecture model: %s\n", ShallowModelGuidance)
	fmt.Fprintf(&b, "%s\n", AuthoringGuidance)

	if len(n.Spec) > 0 {
		fmt.Fprintf(&b, "\nSpecification for your node:\n%s\n", strings.TrimSpace(n.Spec))
	}
	fmt.Fprintf(&b, "\n%s\n", BriefClosing)
	return b.String(), nil
}

// capText trims s to at most max characters, marking the cut.
func capText(s string, max int) (string, bool) {
	if utf8.RuneCountInString(s) <= max {
		return s, false
	}
	r := []rune(s)
	return strings.TrimRight(string(r[:max]), " \n") + "...", true
}

// siblingContractText renders a sibling's contract payloads and errors, capped, with a
// pointer to the full text. Empty when the sibling has no such content.
func siblingContractText(c *NodeContract) string {
	if c == nil || (strings.TrimSpace(c.Payloads) == "" && len(c.Errors) == 0) {
		return ""
	}
	var parts []string
	if p := strings.TrimSpace(c.Payloads); p != "" {
		parts = append(parts, "Contract payloads: "+p)
	}
	if len(c.Errors) > 0 {
		parts = append(parts, "Contract errors: "+strings.Join(c.Errors, "; "))
	}
	text, cut := capText(strings.Join(parts, "\n"), siblingContractCap)
	if cut {
		text += "\n(contract truncated; full text via get_graph or the HTTP API)"
	}
	return text
}

func (idx *Index) writeProjectRules(b *strings.Builder) {
	if len(idx.Constraints) == 0 {
		return
	}
	b.WriteString("\nProject rules (apply to everything you write):\n")
	for _, sev := range []string{ConstraintMust, ConstraintShould} {
		for _, c := range idx.Constraints {
			if c.Severity == sev {
				fmt.Fprintf(b, "  - [%s] %s: %s\n", c.Severity, c.ID, c.Text)
			}
		}
	}
}

// interfacesCap is the character budget for the whole Interfaces section.
const interfacesCap = 4000

// writeInterfaces lists each relationship touching n that carries a contract. The edge
// A -> B reads "A calls B": B provides the interface and A consumes it.
func (idx *Index) writeInterfaces(b *strings.Builder, n *Node) {
	var sec strings.Builder
	omitted, truncated := 0, false
	for _, rel := range idx.Relationships {
		if rel.Contract.IsEmpty() || (rel.From != n.ID && rel.To != n.ID) {
			continue
		}
		var e strings.Builder
		other := rel.To
		role := "you CONSUME this interface; the provider is"
		if rel.To == n.ID {
			other = rel.From
			role = "you PROVIDE this interface; the consumer is"
		}
		fmt.Fprintf(&e, "  - %s %q (%s):", role, idx.nodeName(other), other)
		if rel.Label != "" {
			fmt.Fprintf(&e, " %s", rel.Label)
		}
		if rel.Kind != "" {
			fmt.Fprintf(&e, " [%s]", rel.Kind)
		}
		e.WriteByte('\n')
		c := rel.Contract
		if c.Format != "" {
			fmt.Fprintf(&e, "    format: %s\n", c.Format)
		}
		if c.Ref != "" {
			fmt.Fprintf(&e, "    ref (repo path): %s\n", c.Ref)
		}
		if strings.TrimSpace(c.Text) != "" {
			e.WriteString(indentLines(strings.TrimSpace(c.Text), "    "))
			e.WriteByte('\n')
		}
		entry := e.String()
		if sec.Len()+utf8.RuneCountInString(entry) > interfacesCap {
			if sec.Len() > 0 {
				omitted++
				continue
			}
			// The first interface alone is over budget: cut it rather than drop it.
			cut, _ := capText(entry, interfacesCap)
			entry = cut + "\n"
			sec.WriteString(entry)
			truncated = true
			continue
		}
		sec.WriteString(entry)
	}
	if sec.Len() == 0 {
		return
	}
	b.WriteString("\nInterfaces you must honour:\n")
	b.WriteString(sec.String())
	if omitted > 0 {
		fmt.Fprintf(b, "  (%d more interface(s) omitted to keep this brief short; more via get_graph or the HTTP API)\n", omitted)
	} else if truncated {
		b.WriteString("  (interface truncated to keep this brief short; more via get_graph or the HTTP API)\n")
	}
}

func (idx *Index) nodeName(id NodeID) string {
	if o := idx.Nodes[id]; o != nil {
		return o.Name
	}
	return string(id)
}

// writeOwnContract prints the node's own contract in full (never truncated).
func writeOwnContract(b *strings.Builder, n *Node) {
	c := n.Contract
	if c.IsEmpty() {
		return
	}
	b.WriteString("\nYour node's contract (you must satisfy it):\n")
	if p := strings.TrimSpace(c.Payloads); p != "" {
		b.WriteString("  Payloads:\n")
		b.WriteString(indentLines(p, "    "))
		b.WriteByte('\n')
	}
	for _, l := range []struct {
		title string
		items []string
	}{{"Invariants", c.Invariants}, {"Errors", c.Errors}, {"Owns data", c.OwnsData}} {
		if len(l.items) == 0 {
			continue
		}
		fmt.Fprintf(b, "  %s:\n", l.title)
		for _, it := range l.items {
			fmt.Fprintf(b, "    - %s\n", it)
		}
	}
}

// writeBuildOrder reports unmet dependencies (advisory, with a stop-and-report line) and
// the nodes that rely on n.
func (idx *Index) writeBuildOrder(b *strings.Builder, n *Node) {
	g := depGraph{nodes: idx.Nodes, children: idx.Children}
	var unmet []NodeID
	if res, err := idx.buildOrderLocked(""); err == nil {
		found := false
		for _, it := range res.Order {
			if it.ID == n.ID {
				unmet, found = it.BlockedBy, true
				break
			}
		}
		if !found {
			// A pure container is not in the build order: use the same declared set
			// (own plus cascaded from non-buildable ancestors) and done rule.
			done := g.doneFn()
			pos := map[NodeID]int{}
			for i, id := range idx.Order {
				pos[id] = i
			}
			for _, d := range sortByPos(g.declared(n.ID), pos) {
				if !done(d) {
					unmet = append(unmet, d)
				}
			}
		}
	}
	// Buildable nodes that rely on n: a declared dependency is n, or a pure container
	// that expands to n. Dependencies cascade from containers to their descendants.
	var dependents []NodeID
	for _, id := range idx.Order {
		if idx.Nodes[id] == nil || id == n.ID || !g.buildable(id) {
			continue
		}
		for _, d := range g.declared(id) {
			if d == n.ID || containsNodeID(g.targets(d), n.ID) {
				dependents = append(dependents, id)
				break
			}
		}
	}
	if len(n.DependsOn) == 0 && len(unmet) == 0 && len(dependents) == 0 {
		return
	}
	b.WriteString("\nBuild order:\n")
	if len(n.DependsOn) > 0 {
		b.WriteString("  You depend on:\n")
		for _, d := range sortedDeps(n.DependsOn) {
			if o := idx.Nodes[d]; o != nil {
				fmt.Fprintf(b, "    - %q (%s) (%s)\n", o.Name, d, o.Status)
			} else {
				fmt.Fprintf(b, "    - %s (missing)\n", d)
			}
		}
	}
	if len(unmet) > 0 {
		names := make([]string, len(unmet))
		for i, d := range unmet {
			names[i] = fmt.Sprintf("%q (%s) (%s)", idx.nodeName(d), d, nodeStatus(idx, d))
		}
		fmt.Fprintf(b, "  Not done yet: %s.\n", strings.Join(names, ", "))
		b.WriteString("  If your work needs one of these, stop and report back rather than guessing its interface.\n")
	}
	if len(dependents) > 0 {
		b.WriteString("  These nodes rely on you, so keep your contract and interfaces stable:\n")
		for _, d := range dependents {
			fmt.Fprintf(b, "    - %q (%s)\n", idx.nodeName(d), d)
		}
	}
}

func containsNodeID(list []NodeID, id NodeID) bool {
	for _, x := range list {
		if x == id {
			return true
		}
	}
	return false
}

func nodeStatus(idx *Index, id NodeID) Status {
	if o := idx.Nodes[id]; o != nil {
		return o.Status
	}
	return ""
}

func siblingNodes(idx *Index, self *Node) []*Node {
	var out []*Node
	for _, id := range idx.Order {
		if id == self.ID {
			continue
		}
		other := idx.Nodes[id]
		if other == nil {
			continue
		}
		if self.RelationshipID != "" {
			if other.RelationshipID == self.RelationshipID {
				out = append(out, other)
			}
			continue
		}
		if other.RelationshipID != "" {
			continue
		}
		if self.ParentID != other.ParentID {
			continue
		}
		out = append(out, other)
	}
	return out
}

func indentLines(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}
