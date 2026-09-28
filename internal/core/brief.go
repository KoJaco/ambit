package core

import (
	"fmt"
	"strings"
)

// Brief returns an imperative task description for an assigned agent working on id.
func (idx *Index) Brief(id NodeID) (string, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	n, ok := idx.Nodes[id]
	if !ok || n == nil {
		return "", fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	var b strings.Builder
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
		}
	}

	fmt.Fprintf(&b, "\nCall the check_scope tool with node_id %q periodically while you work, and again before you finish.\n", id)

	if len(n.Spec) > 0 {
		fmt.Fprintf(&b, "\nSpecification for your node:\n%s\n", strings.TrimSpace(n.Spec))
	}
	return b.String(), nil
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
