package core

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Node review metadata (decision 0015). All of it is optional.
const (
	RiskLow    = "low"
	RiskMedium = "medium"
	RiskHigh   = "high"

	ReviewNone     = "none"
	ReviewRequired = "required"

	MaxVerifyCommands = 8
	MaxVerifyLen      = 300
)

// normalizeNodeMeta trims and validates risk, review and verify commands.
func normalizeNodeMeta(risk, review string, verify []string) (string, string, []string, error) {
	risk, review = strings.ToLower(strings.TrimSpace(risk)), strings.ToLower(strings.TrimSpace(review))
	switch risk {
	case "", RiskLow, RiskMedium, RiskHigh:
	default:
		return "", "", nil, fmt.Errorf("%w: node risk %q (valid: low, medium, high)", ErrBadNodeMeta, risk)
	}
	switch review {
	case "", ReviewNone, ReviewRequired:
	default:
		return "", "", nil, fmt.Errorf("%w: node review %q (valid: none, required)", ErrBadNodeMeta, review)
	}
	if len(verify) > MaxVerifyCommands {
		return "", "", nil, fmt.Errorf("%w: at most %d verify commands", ErrBadNodeMeta, MaxVerifyCommands)
	}
	var out []string
	for _, c := range verify {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if utf8.RuneCountInString(c) > MaxVerifyLen {
			return "", "", nil, fmt.Errorf("%w: a verify command exceeds %d characters", ErrBadNodeMeta, MaxVerifyLen)
		}
		out = append(out, c)
	}
	return risk, review, out, nil
}

// reviewRequiredFor reports whether an agent finishing n must stop at needs_review.
// A node's own review setting wins over the method's completion policy.
func reviewRequiredFor(m *Method, n *Node) bool {
	switch n.Review {
	case ReviewRequired:
		return true
	case ReviewNone:
		return false
	}
	return m.Completion.ReviewRequired
}
