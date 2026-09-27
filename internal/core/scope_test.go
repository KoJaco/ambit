package core

import (
	"errors"
	"strings"
	"testing"
)

func scopeIndex(nodes ...*Node) *Index {
	idx := &Index{
		Nodes: map[NodeID]*Node{},
		Order: make([]NodeID, 0, len(nodes)),
	}
	for _, n := range nodes {
		idx.Nodes[n.ID] = n
		idx.Order = append(idx.Order, n.ID)
	}
	return idx
}

func node(id, impl string, scope []string, protected bool) *Node {
	n := &Node{ID: NodeID(id), Name: id, Implementation: []string{impl}, Protected: protected}
	if scope != nil {
		n.Scope = append([]string{}, scope...)
	}
	return n
}

func checkOne(t *testing.T, assignment string, idx *Index, path string) FileResult {
	t.Helper()
	got, err := CheckScope(NodeID(assignment), []string{path}, idx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("results %d", len(got))
	}
	return got[0]
}

func TestCheckScopeEmptyScopeAllowsImplementation(t *testing.T) {
	n := node("payments-service", "src/payments/**", nil, false)
	idx := scopeIndex(n)
	got := checkOne(t, "payments-service", idx, "src/payments/authorise.ts")
	if got.Kind != KindAllowed || len(got.Hits) != 0 {
		t.Fatalf("verdict %+v", got)
	}
	if n.Scope != nil {
		t.Fatalf("scope rewritten: %#v", n.Scope)
	}
}

func TestCheckScopeOutsideDefault(t *testing.T) {
	idx := scopeIndex(
		node("payments-service", "src/payments/**", nil, false),
		node("orders-service", "src/orders/**", nil, false),
	)
	got := checkOne(t, "payments-service", idx, "src/orders/create.ts")
	if got.Kind != KindViolation || len(got.Hits) != 1 {
		t.Fatalf("verdict %+v", got)
	}
	h := got.Hits[0]
	if h.Rule != RuleOutsideScope || h.Node != "orders-service" {
		t.Fatalf("hit %+v", h)
	}
	if h.Detail != "belongs to orders-service; not in the scope declared for payments-service" {
		t.Fatalf("detail %q", h.Detail)
	}
}

func TestCheckScopeNarrowerScopeForbidsGap(t *testing.T) {
	idx := scopeIndex(node("payments-service", "src/payments/**", []string{"src/payments/api/**"}, false))
	got := checkOne(t, "payments-service", idx, "src/payments/internal/store.ts")
	if got.Kind != KindViolation || len(got.Hits) != 1 {
		t.Fatalf("verdict %+v", got)
	}
	if got.Hits[0].Rule != RuleOutsideScope || got.Hits[0].Node != "payments-service" {
		t.Fatalf("hit %+v", got.Hits[0])
	}
}

func TestCheckScopeWiderScopeAllowsExtraGlob(t *testing.T) {
	idx := scopeIndex(node("payments-service", "src/payments/**", []string{"src/payments/**", "src/shared/**"}, false))
	got := checkOne(t, "payments-service", idx, "src/shared/money.ts")
	if got.Kind != KindAllowed || len(got.Hits) != 0 {
		t.Fatalf("verdict %+v", got)
	}
}

func TestCheckScopeProtectedOverridesAssignment(t *testing.T) {
	n := node("billing", "src/billing/**", nil, true)
	idx := scopeIndex(n)
	got := checkOne(t, "billing", idx, "src/billing/fee.ts")
	if got.Kind != KindViolation || len(got.Hits) != 1 {
		t.Fatalf("verdict %+v", got)
	}
	if got.Hits[0].Rule != RuleProtected || got.Hits[0].Node != "billing" {
		t.Fatalf("hit %+v", got.Hits[0])
	}
}

func TestCheckScopeProtectedWithOtherAssignment(t *testing.T) {
	idx := scopeIndex(
		node("payments-service", "src/payments/**", nil, false),
		node("billing", "src/billing/**", nil, true),
	)
	got := checkOne(t, "payments-service", idx, "src/billing/fee.ts")
	if got.Kind != KindViolation || len(got.Hits) != 2 {
		t.Fatalf("verdict %+v", got)
	}
	if got.Hits[0].Rule != RuleProtected || got.Hits[0].Node != "billing" {
		t.Fatalf("protected hit %+v", got.Hits[0])
	}
	if got.Hits[1].Rule != RuleOutsideScope || got.Hits[1].Node != "payments-service" {
		t.Fatalf("outside hit %+v", got.Hits[1])
	}
}

func TestCheckScopeTwoFailingMatches(t *testing.T) {
	idx := scopeIndex(
		node("payments-service", "src/payments/**", nil, false),
		node("orders-service", "src/shared/**", nil, false),
		node("ledger", "src/shared/**", nil, false),
	)
	got := checkOne(t, "payments-service", idx, "src/shared/money.ts")
	if got.Kind != KindViolation || len(got.Hits) != 2 {
		t.Fatalf("verdict %+v", got)
	}
	if got.Hits[0].Rule != RuleOutsideScope || got.Hits[0].Node != "orders-service" {
		t.Fatalf("hit0 %+v", got.Hits[0])
	}
	if got.Hits[1].Rule != RuleOutsideScope || got.Hits[1].Node != "ledger" {
		t.Fatalf("hit1 %+v", got.Hits[1])
	}
}

func TestCheckScopeUnmappedWithAssignment(t *testing.T) {
	idx := scopeIndex(node("payments-service", "src/payments/**", nil, false))
	got := checkOne(t, "payments-service", idx, "docs/notes.md")
	if got.Kind != KindViolation || len(got.Hits) != 1 {
		t.Fatalf("verdict %+v", got)
	}
	h := got.Hits[0]
	if h.Rule != RuleOutsideScope || h.Node != "payments-service" {
		t.Fatalf("hit %+v", h)
	}
	if h.Detail != "matches no node; not in the scope declared for payments-service" {
		t.Fatalf("detail %q", h.Detail)
	}
}

func TestCheckScopeUnmappedWithoutAssignment(t *testing.T) {
	idx := scopeIndex(node("payments-service", "src/payments/**", nil, false))
	got := checkOne(t, "", idx, "docs/notes.md")
	if got.Kind != KindInformational || len(got.Hits) != 0 {
		t.Fatalf("verdict %+v", got)
	}
}

func TestCheckScopeProtectedWithoutAssignment(t *testing.T) {
	idx := scopeIndex(node("billing", "src/billing/**", nil, true))
	got := checkOne(t, "", idx, "src/billing/fee.ts")
	if got.Kind != KindViolation || len(got.Hits) != 1 || got.Hits[0].Rule != RuleProtected {
		t.Fatalf("verdict %+v", got)
	}
}

func TestCheckScopeInScopeIgnoresOtherOwner(t *testing.T) {
	idx := scopeIndex(
		node("payments-service", "src/payments/**", []string{"src/payments/**", "src/shared/**"}, false),
		node("ledger", "src/shared/**", nil, false),
	)
	got := checkOne(t, "payments-service", idx, "src/shared/money.ts")
	if got.Kind != KindAllowed {
		t.Fatalf("verdict %+v", got)
	}
}

func TestCheckScopeDanglingAssignment(t *testing.T) {
	idx := scopeIndex(node("payments-service", "src/payments/**", nil, false))
	_, err := CheckScope("missing", []string{"docs/notes.md"}, idx)
	var dangling *DanglingAssignmentError
	if !errors.As(err, &dangling) || dangling.ID != "missing" {
		t.Fatalf("err %v", err)
	}
}

func TestCheckScopeInvalidGlob(t *testing.T) {
	idx := scopeIndex(node("payments-service", "[", nil, false))
	_, err := CheckScope("payments-service", []string{"src/a.ts"}, idx)
	if err == nil || !strings.Contains(err.Error(), "payments-service") || !strings.Contains(err.Error(), "[") {
		t.Fatalf("err %v", err)
	}
}
