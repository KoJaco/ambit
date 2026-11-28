package core

import (
	"path/filepath"
	"strings"
	"testing"
)

// lintIdx returns a model whose active method sets every given rule to the severity.
func lintIdx(t *testing.T, sev string, rules ...string) (*Index, string) {
	t.Helper()
	idx, dir := newModel(t)
	var parts []string
	for _, r := range rules {
		parts = append(parts, `"`+r+`":"`+sev+`"`)
	}
	writeMethod(t, dir, "lint.json", `{"schema_version":1,"id":"lint","lint":{`+strings.Join(parts, ",")+`}}`)
	idx = reopen(t, dir)
	if err := idx.SetConfigKeys(map[string]any{"method": "lint"}); err != nil {
		t.Fatal(err)
	}
	return idx, dir
}

func mk(t *testing.T, idx *Index, in CreateInput) NodeID {
	t.Helper()
	id, err := idx.Create(in)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func findingsFor(t *testing.T, idx *Index, opts CheckModelOptions) LintResult {
	t.Helper()
	res, err := CheckModel(idx, opts)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func onlyRule(res LintResult, rule string) []LintFinding {
	var out []LintFinding
	for _, f := range res.Findings {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

func TestLintRegistryHasTwelveRules(t *testing.T) {
	if got := LintRuleIDs(); len(got) != 12 {
		t.Fatalf("%v", got)
	}
}

func TestLintLeafImplementationAndExemptions(t *testing.T) {
	idx, _ := lintIdx(t, "enforced", RuleLeafImplementation, RuleLeafContract)
	mk(t, idx, CreateInput{Name: "Bare", Type: "service"})
	mk(t, idx, CreateInput{Name: "Impl", Type: "service", Implementation: []string{"a/**"}, Contract: &NodeContract{Payloads: "p"}})
	mk(t, idx, CreateInput{Name: "A Note", Type: "note"})
	mk(t, idx, CreateInput{Name: "Stripe", Type: "external"})
	group := mk(t, idx, CreateInput{Name: "Group", Type: "boundary"})
	mk(t, idx, CreateInput{Name: "Child", Type: "service", ParentID: group, Implementation: []string{"c/**"}})
	res := findingsFor(t, idx, CheckModelOptions{})
	li := onlyRule(res, RuleLeafImplementation)
	if len(li) != 1 || li[0].NodeID != "bare" || li[0].Severity != "enforced" {
		t.Fatalf("leaf-implementation: %+v", li)
	}
	lc := onlyRule(res, RuleLeafContract)
	got := map[NodeID]bool{}
	for _, f := range lc {
		got[f.NodeID] = true
	}
	if len(lc) != 2 || !got["bare"] || !got["child"] {
		t.Fatalf("leaf-contract: %+v", lc)
	}
	if res.OK || res.Counts.Enforced != 3 || res.Counts.Total != 3 || res.Method != "lint" {
		t.Fatalf("%+v", res)
	}
}

func TestLintSpecIconEdgeRules(t *testing.T) {
	idx, _ := lintIdx(t, "advisory", RuleSpecPresent, RuleIconSet, RuleEdgeLabel, RuleEdgeKind)
	a := mk(t, idx, CreateInput{Name: "A", Type: "service", Spec: "has spec", Icon: ""})
	b := mk(t, idx, CreateInput{Name: "B", Type: "service"})
	rel, err := idx.SetRelationship(SetRelationshipInput{From: a, To: b})
	if err != nil {
		t.Fatal(err)
	}
	res := findingsFor(t, idx, CheckModelOptions{})
	if f := onlyRule(res, RuleSpecPresent); len(f) != 1 || f[0].NodeID != b {
		t.Fatalf("spec-present: %+v", f)
	}
	if f := onlyRule(res, RuleIconSet); len(f) != 2 {
		t.Fatalf("icon-set: %+v", f)
	}
	if f := onlyRule(res, RuleEdgeLabel); len(f) != 1 || f[0].RelationshipID != rel.ID {
		t.Fatalf("edge-label: %+v", f)
	}
	if f := onlyRule(res, RuleEdgeKind); len(f) != 1 || f[0].RelationshipID != rel.ID {
		t.Fatalf("edge-kind: %+v", f)
	}
	if !res.OK || res.Counts.Advisory != 5 || res.Counts.Enforced != 0 {
		t.Fatalf("advisory only must stay ok: %+v", res.Counts)
	}
}

func TestLintEdgeRulesPassWhenSet(t *testing.T) {
	idx, _ := lintIdx(t, "enforced", RuleEdgeLabel, RuleEdgeKind)
	a := mk(t, idx, CreateInput{Name: "A", Type: "service"})
	b := mk(t, idx, CreateInput{Name: "B", Type: "service"})
	if _, err := idx.SetRelationship(SetRelationshipInput{From: a, To: b, Label: "calls", Kind: KindSync}); err != nil {
		t.Fatal(err)
	}
	if res := findingsFor(t, idx, CheckModelOptions{}); !res.OK || len(res.Findings) != 0 {
		t.Fatalf("%+v", res)
	}
}

func TestLintMaxDepthParam(t *testing.T) {
	idx, dir := newModel(t)
	writeMethod(t, dir, "deep.json", `{"schema_version":1,"id":"deep","lint":{"max-depth":{"severity":"advisory","params":{"max_depth":1}}}}`)
	writeMethod(t, dir, "deepdef.json", `{"schema_version":1,"id":"deepdef","lint":{"max-depth":"advisory"}}`)
	idx = reopen(t, dir)
	l0 := mk(t, idx, CreateInput{Name: "L0", Type: "boundary"})
	l1 := mk(t, idx, CreateInput{Name: "L1", Type: "boundary", ParentID: l0})
	l2 := mk(t, idx, CreateInput{Name: "L2", Type: "boundary", ParentID: l1})
	l3 := mk(t, idx, CreateInput{Name: "L3", Type: "boundary", ParentID: l2})
	mk(t, idx, CreateInput{Name: "L4", Type: "service", ParentID: l3})
	if _, err := idx.SetMethod("deepdef"); err != nil {
		t.Fatal(err)
	}
	f := onlyRule(findingsFor(t, idx, CheckModelOptions{}), RuleMaxDepth)
	if len(f) != 1 || f[0].NodeID != "l4" {
		t.Fatalf("default max_depth 3: %+v", f)
	}
	if _, err := idx.SetMethod("deep"); err != nil {
		t.Fatal(err)
	}
	f = onlyRule(findingsFor(t, idx, CheckModelOptions{}), RuleMaxDepth)
	if len(f) != 3 {
		t.Fatalf("max_depth 1: %+v", f)
	}
}

func TestLintGlobOverlap(t *testing.T) {
	idx, _ := lintIdx(t, "enforced", RuleGlobOverlap)
	mk(t, idx, CreateInput{Name: "Api", Type: "service", Implementation: []string{"src/**"}})
	mk(t, idx, CreateInput{Name: "Web", Type: "service", Implementation: []string{"src/web/**/*.{ts,tsx}"}})
	mk(t, idx, CreateInput{Name: "Docs", Type: "service", Implementation: []string{"docs/**"}})
	res := findingsFor(t, idx, CheckModelOptions{})
	f := onlyRule(res, RuleGlobOverlap)
	if len(f) != 1 || !strings.Contains(f[0].Message, "api, web") {
		t.Fatalf("%+v", f)
	}
}

func TestLintGlobOverlapIgnoresNestedNodes(t *testing.T) {
	idx, _ := lintIdx(t, "enforced", RuleGlobOverlap)
	p := mk(t, idx, CreateInput{Name: "Platform", Type: "boundary", Implementation: []string{"src/**"}})
	mk(t, idx, CreateInput{Name: "Api", Type: "service", ParentID: p, Implementation: []string{"src/api/**"}})
	if f := onlyRule(findingsFor(t, idx, CheckModelOptions{}), RuleGlobOverlap); len(f) != 0 {
		t.Fatalf("%+v", f)
	}
}

func TestLintOrphanNode(t *testing.T) {
	idx, _ := lintIdx(t, "advisory", RuleOrphanNode)
	if f := onlyRule(findingsFor(t, idx, CheckModelOptions{}), RuleOrphanNode); len(f) != 0 {
		t.Fatalf("empty: %+v", f)
	}
	a := mk(t, idx, CreateInput{Name: "A", Type: "service"})
	if f := onlyRule(findingsFor(t, idx, CheckModelOptions{}), RuleOrphanNode); len(f) != 0 {
		t.Fatalf("single node is not an orphan: %+v", f)
	}
	b := mk(t, idx, CreateInput{Name: "B", Type: "service"})
	mk(t, idx, CreateInput{Name: "Lonely", Type: "service"})
	if _, err := idx.SetRelationship(SetRelationshipInput{From: a, To: b}); err != nil {
		t.Fatal(err)
	}
	f := onlyRule(findingsFor(t, idx, CheckModelOptions{}), RuleOrphanNode)
	if len(f) != 1 || f[0].NodeID != "lonely" {
		t.Fatalf("%+v", f)
	}
}

func TestLintDependencyCycle(t *testing.T) {
	idx, dir := lintIdx(t, "enforced", RuleDependencyCycle)
	a := mk(t, idx, CreateInput{Name: "A", Type: "service", Implementation: []string{"a/**"}})
	b := mk(t, idx, CreateInput{Name: "B", Type: "service", Implementation: []string{"b/**"}, DependsOn: []NodeID{a}})
	if res := findingsFor(t, idx, CheckModelOptions{}); !res.OK {
		t.Fatalf("%+v", res)
	}
	// A cycle can only exist on disk (writes reject it): edit the node file.
	path := filepath.Join(dir, ".arch", "nodes", string(a)+".json")
	obj := readObj(t, path)
	obj["depends_on"] = []any{string(b)}
	writeObj(t, path, obj)
	idx2 := reopen(t, dir)
	res := findingsFor(t, idx2, CheckModelOptions{})
	f := onlyRule(res, RuleDependencyCycle)
	if len(f) != 1 || res.OK || !strings.Contains(f[0].Message, "a, b") {
		t.Fatalf("%+v", res)
	}
}

func TestLintSeverityMappingAndAllRules(t *testing.T) {
	idx, _ := lintIdx(t, "off", RuleSpecPresent)
	mk(t, idx, CreateInput{Name: "A", Type: "service"})
	if res := findingsFor(t, idx, CheckModelOptions{}); len(res.Findings) != 0 || !res.OK {
		t.Fatalf("off must run nothing: %+v", res)
	}
	// AllRules runs every rule, all advisory, and never fails OK.
	res := findingsFor(t, idx, CheckModelOptions{AllRules: true})
	seen := map[string]bool{}
	for _, f := range res.Findings {
		if f.Severity != "advisory" {
			t.Fatalf("%+v", f)
		}
		seen[f.Rule] = true
	}
	for _, r := range []string{RuleSpecPresent, RuleIconSet, RuleLeafImplementation, RuleLeafContract} {
		if !seen[r] {
			t.Fatalf("all_rules missed %s: %+v", r, res.Findings)
		}
	}
	if !res.OK || res.Counts.Enforced != 0 {
		t.Fatalf("%+v", res)
	}
	// Unlisted rules default to off under an enforcing method.
	idx3, _ := lintIdx(t, "enforced", RuleSpecPresent)
	mk(t, idx3, CreateInput{Name: "A", Type: "service"})
	res = findingsFor(t, idx3, CheckModelOptions{})
	if len(res.Findings) != 1 || res.Findings[0].Rule != RuleSpecPresent || res.OK {
		t.Fatalf("%+v", res)
	}
}

func TestCheckModelFreeformIsQuiet(t *testing.T) {
	idx, _ := newModel(t)
	mk(t, idx, CreateInput{Name: "A", Type: "service"})
	res := findingsFor(t, idx, CheckModelOptions{})
	if !res.OK || len(res.Findings) != 0 || res.Method != "freeform" || res.Stage != "" || res.Drift != nil {
		t.Fatalf("%+v", res)
	}
}

func TestCheckModelDrift(t *testing.T) {
	idx, dir := newModel(t)
	mk(t, idx, CreateInput{Name: "A", Type: "service", Implementation: []string{"a/**"}})
	// Not a git repo: requested drift is reported softly.
	res := findingsFor(t, idx, CheckModelOptions{IncludeDrift: boolPtr(true)})
	if res.Drift != nil || res.DriftError == "" {
		t.Fatalf("%+v", res)
	}
	gitInitRepo(t, dir)
	writeFile(t, filepath.Join(dir, "stray.txt"), "x")
	gitCmd(t, dir, "add", "stray.txt")
	res = findingsFor(t, idx, CheckModelOptions{IncludeDrift: boolPtr(true)})
	if res.Drift == nil || len(res.Drift.Unowned) != 1 || res.DriftError != "" {
		t.Fatalf("%+v", res)
	}
	// map-existing turns drift on by itself.
	if _, err := idx.SetMethod("map-existing"); err != nil {
		t.Fatal(err)
	}
	if res := findingsFor(t, idx, CheckModelOptions{}); res.Drift == nil {
		t.Fatalf("map-existing should include drift: %+v", res)
	}
}

func TestCheckModelReportsStage(t *testing.T) {
	idx, _ := newModel(t)
	if _, err := idx.SetMethod("design-first"); err != nil {
		t.Fatal(err)
	}
	mk(t, idx, CreateInput{Name: "A", Type: "service"})
	res := findingsFor(t, idx, CheckModelOptions{})
	if res.Stage != "design" || res.OK {
		t.Fatalf("%+v", res)
	}
}

func boolPtr(b bool) *bool { return &b }

func TestCheckModelExplicitDriftFalseOverridesMethod(t *testing.T) {
	idx, _ := newModel(t)
	if _, err := idx.SetMethod("map-existing"); err != nil {
		t.Fatal(err)
	}
	if res := findingsFor(t, idx, CheckModelOptions{}); res.Drift == nil && res.DriftError == "" {
		t.Fatal("nil option should follow the method")
	}
	if res := findingsFor(t, idx, CheckModelOptions{IncludeDrift: boolPtr(false)}); res.Drift != nil || res.DriftError != "" {
		t.Fatalf("explicit false ignored: %+v", res)
	}
}

func TestLintOrphanExemptsNoteAndExternal(t *testing.T) {
	idx, _ := lintIdx(t, "advisory", RuleOrphanNode)
	mk(t, idx, CreateInput{Name: "Memo", Type: "note"})
	mk(t, idx, CreateInput{Name: "Stripe", Type: "external"})
	if f := onlyRule(findingsFor(t, idx, CheckModelOptions{}), RuleOrphanNode); len(f) != 0 {
		t.Fatalf("%+v", f)
	}
}

func TestCheckModelMethodWarning(t *testing.T) {
	idx, dir := newModel(t)
	if res := findingsFor(t, idx, CheckModelOptions{}); res.MethodWarning != "" {
		t.Fatalf("%q", res.MethodWarning)
	}
	writeMethod(t, dir, "mine.json", `{broken`)
	if err := idx.SetConfigKeys(map[string]any{"method": "mine"}); err != nil {
		t.Fatal(err)
	}
	res := findingsFor(t, idx, CheckModelOptions{})
	if res.Method != "freeform" || !strings.Contains(res.MethodWarning, `"mine"`) {
		t.Fatalf("%+v", res)
	}
}
