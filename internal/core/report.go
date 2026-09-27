package core

import (
	"fmt"
	"strings"
)

// FormatReport prints violations and informational drift. Allowed paths are omitted.
// When any path is informational, the text says that no assignment is active.
func FormatReport(results []FileResult) string {
	informational := false
	for _, r := range results {
		if r.Kind == KindInformational {
			informational = true
			break
		}
	}
	var b strings.Builder
	if informational {
		b.WriteString("no assignment is active\n")
	}
	for _, r := range results {
		switch r.Kind {
		case KindViolation:
			for _, h := range r.Hits {
				fmt.Fprintf(&b, "%s %s %s\n", r.Path, h.Rule, h.Node)
			}
		case KindInformational:
			fmt.Fprintf(&b, "%s informational\n", r.Path)
		}
	}
	return b.String()
}
