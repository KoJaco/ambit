package core

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// siblingSpecCap is the per-sibling character budget for spec text in a brief. Contracts
// are never cut: an agent must be able to code against an interface from its brief alone.
const siblingSpecCap = 1200

// BriefClosing is the last line of every brief.
const BriefClosing = "When you are done, call check_done with your node_id (add worktree if you work in a git worktree): it checks the files changed on your branch against your scope, runs check_drift for your node and flags interface changes in one pass. Then call update_node_status for your node. Drift only sees files git can see, so gitignored files are invisible to it. " + SubagentGuidance

// SubagentGuidance is the delegation rule that ends every brief and is repeated in the MCP
// tool docs: a subagent that implements a node gets the read-only tools only, and the parent
// agent writes documentation deliverables itself.
const SubagentGuidance = "If you delegate to subagents, give implementers only the read-only tools (every get_ and check_ tool except check_dependents, plus list_icons; run_verify and check_dependents execute shell commands, so keep those in the parent); keep staging, model-editing and status tools, and write documentation deliverables yourself in the parent."

// closingFor is BriefClosing adjusted to the method's completion policy for node n.
func closingFor(m *Method, n *Node) string {
	const tail = "Then call update_node_status for your node."
	switch {
	case !m.Completion.AgentMayComplete:
		return strings.Replace(BriefClosing, tail, "Do not change your node's status to done or needs_review: report your result to the architect, who sets it.", 1)
	case reviewRequiredFor(m, n):
		return strings.Replace(BriefClosing, tail, "Then call update_node_status with needs_review for your node (not done): the architect marks it done after review.", 1)
	}
	return BriefClosing
}

// ShallowModelGuidance is the one-line modeling rule shared by the brief and the MCP
// authoring tool descriptions (decision 0010).
const ShallowModelGuidance = "Prefer shallow models. Use a container node to group related nodes. Nest more than three levels only for genuinely separate subsystems."

// AuthoringGuidance is the model-first workflow guidance shared by the brief and the MCP
// authoring tool descriptions.
const AuthoringGuidance = "Model high-level first: start with the few top-level parts, then drill down only where asked. " +
	"Treat container nodes as deployment zones (for example browser, edge, API, data) and keep nesting to about three levels. " +
	"Before drilling down, ask the user how they want the top-level view organised. " +
	"A container shown as a summary card on the canvas is a placeholder: before building, flesh it out into child nodes so each part has its own scope and spec. " +
	"Show what enters or leaves a container from outside as type external start/end nodes, with one labelled edge per channel. " +
	"Do not add an edge that carries no information (every-pair wiring, or what the hierarchy already says). " +
	"Set explicit ids on nodes you create so later calls can reference them in parent_id and relationships. " +
	"Call list_icons to pick icon ids and get_graph to see what already exists."

// PartialScopeGuidance is added to briefs of a partial-scope project and to the MCP
// authoring tool descriptions (decision 0016).
const PartialScopeGuidance = "In a partial-scope project model only the active part and put its files in include."

// BriefNodeIDs lists the nodes worth exporting a brief for, in model order: every node an
// agent can build (a leaf, or a container with implementation globs) that is not external.
func (idx *Index) BriefNodeIDs() []NodeID {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	g := newDepGraph(idx.Nodes, idx.Children)
	var out []NodeID
	for _, id := range idx.Order {
		if n := idx.Nodes[id]; n != nil && !n.External && g.buildable(id) {
			out = append(out, id)
		}
	}
	return out
}

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
	if idx.contractChangedLocked(n) {
		fmt.Fprintf(&b, "NOTE: a contract this node must honour changed after it was marked %s (git log -p .arch/ shows the change). Re-check the existing implementation against the contracts below before you continue.\n\n", n.Status)
	}

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
	siblingChars, siblingsCut := 0, 0
	if len(siblings) > 0 {
		fmt.Fprintf(&b, "\nSibling nodes (interfaces only — do not edit their implementation):\n")
		for _, s := range siblings {
			fmt.Fprintf(&b, "  - %q (%s, %s)\n", s.Name, s.ID, s.Type)
			if strings.TrimSpace(s.Spec) != "" {
				spec := strings.TrimSpace(s.Spec)
				if cut, was := capText(spec, siblingSpecCap); was {
					spec = cut + "\n(spec truncated)"
				}
				b.WriteString(indentLines(spec, "    "))
				b.WriteByte('\n')
			}
			if text := siblingContractText(s.Contract); text != "" {
				if siblingChars += utf8.RuneCountInString(text); siblingChars > interfacesCap {
					siblingsCut++
					continue
				}
				b.WriteString(indentLines(text, "    "))
				b.WriteByte('\n')
			}
		}
		if siblingsCut > 0 {
			fmt.Fprintf(&b, "  (%d more sibling contract(s) omitted to keep this brief bounded; get_graph with include_contracts returns them)\n", siblingsCut)
		}
	}

	idx.writeProjectRules(&b)
	idx.writeInterfaces(&b, n)
	writeOwnContract(&b, n)
	idx.writeBuildOrder(&b, n)

	fmt.Fprintf(&b, "\nCall the check_scope tool with node_id %q periodically while you work, and again before you finish.\n", id)
	if len(n.Verify) > 0 {
		b.WriteString("\nVerify your work with these commands before you finish:\n")
		for _, c := range n.Verify {
			if env := referencedEnv(c); len(env) > 0 {
				fmt.Fprintf(&b, "  - %s  (needs environment: %s)\n", c, strings.Join(env, ", "))
			} else {
				fmt.Fprintf(&b, "  - %s\n", c)
			}
		}
	}
	idx.writeBuildPlan(&b, n)
	if n.Risk != "" || n.Review != "" {
		var parts []string
		if n.Risk != "" {
			parts = append(parts, "risk "+n.Risk)
		}
		if n.Review == ReviewRequired {
			parts = append(parts, "review required")
		}
		if len(parts) > 0 {
			fmt.Fprintf(&b, "\nThis node is flagged: %s. Take extra care and keep the change small.\n", strings.Join(parts, ", "))
		}
		fmt.Fprintf(&b, "Risk and review: %s\n", riskReviewLine(idx.activeMethodLocked(), n))
	}

	fmt.Fprintf(&b, "\nIf you propose changes to the architecture model: %s\n", ShallowModelGuidance)
	fmt.Fprintf(&b, "%s\n", AuthoringGuidance)
	if idx.Scope == ScopePartial {
		fmt.Fprintf(&b, "%s\n", PartialScopeGuidance)
	}

	if len(n.Spec) > 0 {
		fmt.Fprintf(&b, "\nSpecification for your node:\n%s\n", strings.TrimSpace(n.Spec))
	}
	fmt.Fprintf(&b, "\n%s\n", closingFor(idx.activeMethodLocked(), n))
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

// siblingContractText renders a sibling's contract in full. Empty when it has none.
func siblingContractText(c *NodeContract) string {
	if c == nil || c.IsEmpty() {
		return ""
	}
	var parts []string
	if p := strings.TrimSpace(c.Payloads); p != "" {
		parts = append(parts, "Contract payloads: "+p)
	}
	for _, l := range []struct {
		title string
		items []string
	}{{"invariants", c.Invariants}, {"errors", c.Errors}, {"owns data", c.OwnsData}} {
		if len(l.items) > 0 {
			parts = append(parts, "Contract "+l.title+": "+strings.Join(l.items, "; "))
		}
	}
	return strings.Join(parts, "\n")
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

// interfacesCap is a safety bound on the whole Interfaces section. It is far above
// what a model with sensible contracts reaches; only a runaway model hits it.
const interfacesCap = 60000

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
		fmt.Fprintf(b, "  (%d more interface(s) omitted to keep this brief short; more via get_graph with include_contracts)\n", omitted)
	} else if truncated {
		b.WriteString("  (interface truncated to keep this brief short; more via get_graph with include_contracts)\n")
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
	g := newDepGraph(idx.Nodes, idx.Children)
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
	// Direct dependents only (not transitive): a declared dependency is n, or a pure
	// container that expands to n, or n gates the node.
	gated := map[NodeID]bool{}
	for _, t := range n.Gates {
		gated[t] = true
	}
	for _, id := range idx.Order {
		if idx.Nodes[id] == nil || id == n.ID || !g.buildable(id) {
			continue
		}
		if gated[id] {
			dependents = append(dependents, id)
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
				if len(o.Implementation) > 0 {
					fmt.Fprintf(b, "      code: %s\n", strings.Join(o.Implementation, ", "))
				}
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

// riskReviewLine links the node's risk to how it is finished.
func riskReviewLine(m *Method, n *Node) string {
	risk := n.Risk
	if risk == "" {
		risk = "not set"
	}
	switch {
	case n.Risk == RiskHigh:
		return "risk is high, so you finish at needs_review (check_done reports finish_status needs_review), never done; the architect marks it done after review."
	case reviewRequiredFor(m, n):
		return fmt.Sprintf("risk is %s and review is required, so you finish at needs_review, never done.", risk)
	}
	return fmt.Sprintf("risk is %s and no review is required, so once check_done passes you may set the status check_done reports.", risk)
}

// writeBuildPlan lists the optional build-planning fields: deliverables, references,
// milestones, human gates (with cleared state) and the nodes this one gates.
func (idx *Index) writeBuildPlan(b *strings.Builder, n *Node) {
	if len(n.Deliverables) > 0 {
		b.WriteString("\nDeliverables (check_deliverables verifies each exists and is non-empty):\n")
		for _, d := range n.Deliverables {
			fmt.Fprintf(b, "  - %s\n", d)
		}
	}
	if len(n.References) > 0 {
		b.WriteString("\nKeep these docs in sync with your spec and contract (check_contract_sync):\n")
		for _, r := range n.References {
			fmt.Fprintf(b, "  - %s\n", r)
		}
	}
	if len(n.Milestones) > 0 {
		b.WriteString("\nMilestones (finish with needs_review, not done, until all are done):\n")
		for _, m := range n.Milestones {
			label := m.Label
			if label == "" {
				label = m.ID
			}
			fmt.Fprintf(b, "  - %s: %s [%s]\n", m.ID, label, m.Status)
		}
	}
	if gs := gateStates(n, idx.gatesClearedLocked()); len(gs) > 0 {
		b.WriteString("\nHuman gates (only a person can clear these; if one is not cleared and you need it, stop and report back):\n")
		for _, g := range gs {
			state := "NOT cleared"
			if g.Cleared {
				state = "cleared"
			}
			fmt.Fprintf(b, "  - %s [%s]\n", gateLabel(HumanGate{Kind: g.Kind, Note: g.Note}), state)
		}
	}
	if len(n.Gates) > 0 {
		ids := make([]string, len(n.Gates))
		for i, g := range sortedDeps(n.Gates) {
			ids[i] = string(g)
		}
		fmt.Fprintf(b, "\nThis node is a gate: %s wait for it. Report what you learn so they can be unblocked.\n", strings.Join(ids, ", "))
	}
}
