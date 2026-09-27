package core

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func slugify(name string) (string, error) {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevHyphen = false
			continue
		}
		if !prevHyphen && b.Len() > 0 {
			b.WriteByte('-')
			prevHyphen = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" || !slugPattern.MatchString(s) {
		return "", fmt.Errorf("%w: %q", errNoSlug, name)
	}
	return s, nil
}

func (idx *Index) allocate(base string) NodeID {
	if !idx.taken(base) {
		return NodeID(base)
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if !idx.taken(candidate) {
			return NodeID(candidate)
		}
	}
}

func (idx *Index) taken(id string) bool {
	if _, ok := idx.Nodes[NodeID(id)]; ok {
		return true
	}
	for _, existing := range idx.Order {
		if string(existing) == id {
			return true
		}
	}
	_, err := os.Stat(idx.nodeJSON(NodeID(id)))
	return err == nil
}
