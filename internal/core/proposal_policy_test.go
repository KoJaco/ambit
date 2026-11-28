package core

import (
	"strings"
	"testing"
)

func pnode(id string, mod func(*Node)) *Node {
	n := &Node{ID: NodeID(id), Name: id, Type: "service", Status: StatusDraft}
	if mod != nil {
		mod(n)
	}
	return n
}

func clone(n *Node, mod func(*Node)) *Node {
	c := cloneNode(n)
	mod(c)
	return c
}

func policyModel(nodes ...*Node) PolicyModel {
	m := PolicyModel{Nodes: map[NodeID]*Node{}}
	for _, n := range nodes {
		m.Nodes[n.ID] = n
	}
	return m
}

func TestClassifyOpsTable(t *testing.T) {
	svc := pnode("svc", func(n *Node) { n.Deliverables = []string{"src/svc/main.go"}; n.Implementation = []string{"src/svc/**"} })
	dep := pnode("dep", nil)
	prot := pnode("prot", func(n *Node) { n.Protected = true; n.Implementation = []string{"src/secure/**"} })
	ext := pnode("ext", func(n *Node) { n.Type = "external" })
	extFlag := pnode("extflag", func(n *Node) { n.External = true })
	box := pnode("box", func(n *Node) { n.Type = "boundary" })
	protBox := pnode("protbox", func(n *Node) { n.Protected = true; n.Type = "boundary" })
	chain := pnode("chain", func(n *Node) { n.DependsOn = []NodeID{"svc"} })
	model := policyModel(svc, dep, prot, ext, extFlag, box, protBox, chain)

	upd := func(cur *Node, fields []string, mod func(*Node)) PolicyOp {
		return PolicyOp{Op: OpUpdateNode, NodeID: string(cur.ID), Fields: fields, Current: cur, Proposed: clone(cur, mod)}
	}
	create := func(mod func(*Node)) PolicyOp {
		n := pnode("fresh", func(n *Node) { n.ParentID = "box"; n.Type = "module" })
		mod(n)
		return PolicyOp{Op: OpCreateNode, NodeID: "fresh", Proposed: n}
	}
	edge := func(from, to, kind string, cur *Relationship) PolicyOp {
		return PolicyOp{Op: OpSetRelationship, NodeID: from, From: NodeID(from), To: NodeID(to), Label: "calls", Kind: kind, CurrentRel: cur}
	}

	cases := []struct {
		name string
		op   PolicyOp
		auto bool
		rule string // allowing rule, or blocking rule
	}{
		// AUTO
		{"spec text", upd(svc, []string{"spec"}, func(n *Node) { n.Spec = "new" }), true, RuleSpecText},
		{"contract text", upd(svc, []string{"contract"}, func(n *Node) { n.Contract = &NodeContract{Invariants: []string{"x"}} }), true, RuleSpecText},
		{"spec and contract", upd(svc, []string{"spec", "contract"}, func(n *Node) { n.Spec = "n" }), true, RuleSpecText},
		{"remove dependency", upd(chain, []string{"depends_on"}, func(n *Node) { n.DependsOn = nil }), true, RuleDependencyLoosen},
		{"add acyclic dependency", upd(svc, []string{"depends_on"}, func(n *Node) { n.DependsOn = []NodeID{"dep"} }), true, RuleDependencyAdd},
		{"scope matching deliverable", upd(svc, []string{"scope"}, func(n *Node) { n.Scope = []string{"src/svc/main.go"} }), true, RuleScopeDeliverable},
		{"implementation matching deliverable", upd(svc, []string{"implementation"}, func(n *Node) { n.Implementation = []string{"src/svc/**", "src/svc/*.go"} }), true, RuleScopeDeliverable},
		{"create under container", create(func(n *Node) {}), true, RuleCreateContainer},
		{"create with safe dep", create(func(n *Node) { n.DependsOn = []NodeID{"dep"} }), true, RuleCreateContainer},
		{"new sync edge", edge("svc", "dep", KindSync, nil), true, RuleEdgeExisting},
		{"new edge, no kind", edge("svc", "dep", "", nil), true, RuleEdgeExisting},
		{"existing edge same kind", edge("svc", "dep", KindAsync, &Relationship{From: "svc", To: "dep", Kind: KindAsync}), true, RuleEdgeExisting},

		// ALWAYS HUMAN: protected
		{"update protected node", upd(prot, []string{"spec"}, func(n *Node) { n.Spec = "x" }), false, BlockProtected},
		{"un-protect via update", upd(svc, []string{"protected"}, func(n *Node) { n.Protected = true }), false, BlockProtected},
		{"depends_on adds protected", upd(svc, []string{"depends_on"}, func(n *Node) { n.DependsOn = []NodeID{"prot"} }), false, BlockProtected},
		{"create under protected parent", create(func(n *Node) { n.ParentID = "protbox" }), false, BlockProtected},
		{"create overlapping protected glob", create(func(n *Node) { n.Implementation = []string{"src/secure/**"} }), false, BlockProtected},
		{"create depends on protected", create(func(n *Node) { n.DependsOn = []NodeID{"prot"} }), false, BlockProtected},
		{"edge to protected", edge("svc", "prot", KindSync, nil), false, BlockProtected},
		// ALWAYS HUMAN: external
		{"create type external", create(func(n *Node) { n.Type = "external" }), false, BlockExternal},
		{"create external flag", create(func(n *Node) { n.External = true }), false, BlockExternal},
		{"create depends on external", create(func(n *Node) { n.DependsOn = []NodeID{"ext"} }), false, BlockExternal},
		{"edge to external", edge("svc", "ext", KindSync, nil), false, BlockExternal},
		{"edge to external flag", edge("extflag", "dep", KindSync, nil), false, BlockExternal},
		// ALWAYS HUMAN: sensitive fields
		{"stage-like field risk", upd(svc, []string{"risk"}, func(n *Node) { n.Risk = "high" }), false, BlockSensitive},
		{"review", upd(svc, []string{"review"}, func(n *Node) { n.Review = "required" }), false, BlockSensitive},
		{"human_gates", upd(svc, []string{"human_gates"}, func(n *Node) { n.HumanGates = []HumanGate{{Kind: GateDecision}} }), false, BlockSensitive},
		{"shared", upd(svc, []string{"shared"}, func(n *Node) { n.Shared = true }), false, BlockSensitive},
		{"protected field", upd(dep, []string{"protected"}, func(n *Node) { n.Protected = true }), false, BlockProtected},
		{"external field", upd(dep, []string{"external"}, func(n *Node) { n.External = true }), false, BlockSensitive},
		{"mixed spec + risk", upd(svc, []string{"spec", "risk"}, func(n *Node) { n.Risk = "low" }), false, BlockSensitive},
		{"create with risk", create(func(n *Node) { n.Risk = "low" }), false, BlockSensitive},
		{"create with review", create(func(n *Node) { n.Review = "required" }), false, BlockSensitive},
		{"create protected", create(func(n *Node) { n.Protected = true }), false, BlockSensitive},
		{"create shared", create(func(n *Node) { n.Shared = true }), false, BlockSensitive},
		{"create human gates", create(func(n *Node) { n.HumanGates = []HumanGate{{Kind: GateCredential}} }), false, BlockSensitive},
		{"create gates others", create(func(n *Node) { n.Gates = []NodeID{"dep"} }), false, BlockSensitive},
		// ALWAYS HUMAN: structural ops
		{"delete node", PolicyOp{Op: OpDeleteNode, NodeID: "dep", Current: dep}, false, BlockDelete},
		{"delete relationship", PolicyOp{Op: OpDeleteRelationship, NodeID: "svc"}, false, BlockDelete},
		{"rename", PolicyOp{Op: OpRenameNode, NodeID: "dep", NewID: "dep2", Current: dep}, false, BlockRename},
		{"set constraints", PolicyOp{Op: OpSetConstraints}, false, BlockConstraints},
		{"set part", PolicyOp{Op: OpSetPart, NodeID: "p"}, false, BlockPart},
		{"delete part", PolicyOp{Op: OpDeletePart, NodeID: "p"}, false, BlockPart},
		{"unknown op", PolicyOp{Op: "teleport"}, false, BlockUnknown},
		// Not auto-eligible
		{"rename via name field", upd(svc, []string{"name"}, func(n *Node) { n.Name = "other" }), false, BlockNotAuto},
		{"reparent", upd(svc, []string{"parent_id"}, func(n *Node) { n.ParentID = "box" }), false, BlockNotAuto},
		{"deliverables change", upd(svc, []string{"deliverables"}, func(n *Node) { n.Deliverables = []string{"x"} }), false, BlockNotAuto},
		{"scope not a deliverable", upd(svc, []string{"scope"}, func(n *Node) { n.Scope = []string{"src/other/**"} }), false, BlockNotAuto},
		{"scope removal", upd(svc, []string{"implementation"}, func(n *Node) { n.Implementation = nil }), false, BlockNotAuto},
		{"dependency cycle", upd(svc, []string{"depends_on"}, func(n *Node) { n.DependsOn = []NodeID{"chain"} }), false, BlockNotAuto},
		{"self dependency", upd(svc, []string{"depends_on"}, func(n *Node) { n.DependsOn = []NodeID{"svc"} }), false, BlockNotAuto},
		{"update missing node", PolicyOp{Op: OpUpdateNode, NodeID: "gone", Fields: []string{"spec"}}, false, BlockNotAuto},
		{"create top level", create(func(n *Node) { n.ParentID = "" }), false, BlockNotAuto},
		{"create inside edge", create(func(n *Node) { n.ParentID = ""; n.RelationshipID = "r" }), false, BlockNotAuto},
		{"create under missing parent", create(func(n *Node) { n.ParentID = "nowhere" }), false, BlockNotAuto},
		{"create already in progress", create(func(n *Node) { n.Status = StatusInProgress }), false, BlockNotAuto},
		{"new async edge", edge("svc", "dep", KindAsync, nil), false, BlockNotAuto},
		{"new data edge", edge("svc", "dep", KindData, nil), false, BlockNotAuto},
		{"edge kind change", edge("svc", "dep", KindAsync, &Relationship{From: "svc", To: "dep", Kind: KindSync}), false, BlockNotAuto},
		{"edge endpoint move", edge("svc", "dep", KindSync, &Relationship{From: "svc", To: "box", Kind: KindSync}), false, BlockNotAuto},
		{"edge missing endpoint", edge("svc", "ghost", KindSync, nil), false, BlockNotAuto},
		{"stale op", PolicyOp{Op: OpUpdateNode, NodeID: "svc", Fields: []string{"spec"}, Current: svc, Proposed: svc, Stale: true, StaleReason: "changed"}, false, BlockStale},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifyOps(model, []PolicyOp{c.op})
			if len(got) != 1 {
				t.Fatalf("%d verdicts", len(got))
			}
			v := got[0]
			if v.Auto != c.auto || v.Rule != c.rule {
				t.Fatalf("auto=%v rule=%q reason=%q, want auto=%v rule=%q", v.Auto, v.Rule, v.Reason, c.auto, c.rule)
			}
			if !c.auto && v.Reason == "" {
				t.Fatal("blocking verdict needs a reason")
			}
			if v.Summary == "" {
				t.Fatal("verdict needs a diff summary")
			}
		})
	}
}

func TestClassifyMixedRulesAndAllAuto(t *testing.T) {
	svc := pnode("svc", func(n *Node) { n.DependsOn = []NodeID{"dep"} })
	dep := pnode("dep", nil)
	other := pnode("other", nil)
	model := policyModel(svc, dep, other)
	op := PolicyOp{Op: OpUpdateNode, NodeID: "svc", Fields: []string{"spec", "depends_on"}, Current: svc,
		Proposed: clone(svc, func(n *Node) { n.Spec = "x"; n.DependsOn = []NodeID{"other"} })}
	vs := ClassifyOps(model, []PolicyOp{op})
	if !vs[0].Auto || vs[0].Rule != RuleDependencyAdd+","+RuleDependencyLoosen+","+RuleSpecText {
		t.Fatalf("%+v", vs[0])
	}
	if !strings.Contains(vs[0].Summary, "depends_on +other -dep") {
		t.Fatalf("summary %q", vs[0].Summary)
	}
	if AllAuto(nil) || !AllAuto(vs) {
		t.Fatal("AllAuto")
	}
	mixed := ClassifyOps(model, []PolicyOp{op, {Op: OpDeleteNode, NodeID: "dep", Current: dep}})
	if AllAuto(mixed) || len(Blocking(mixed)) != 1 || Blocking(mixed)[0].Index != 0 {
		t.Fatalf("%+v", mixed)
	}
}

func TestClassifyCreateChainUnderCreatedContainer(t *testing.T) {
	box := pnode("box", func(n *Node) { n.Type = "boundary" })
	model := policyModel(box)
	a := pnode("a", func(n *Node) { n.ParentID = "box"; n.Type = "boundary" })
	b := pnode("b", func(n *Node) { n.ParentID = "a"; n.Type = "module" })
	vs := ClassifyOps(model, []PolicyOp{
		{Index: 0, Op: OpCreateNode, NodeID: "a", Proposed: a},
		{Index: 1, Op: OpCreateNode, NodeID: "b", Proposed: b},
	})
	if !AllAuto(vs) {
		t.Fatalf("%+v", vs)
	}
	// A child of a create that is itself blocked is not auto.
	extA := pnode("a", func(n *Node) { n.ParentID = "box"; n.Type = "external" })
	vs = ClassifyOps(model, []PolicyOp{
		{Index: 0, Op: OpCreateNode, NodeID: "a", Proposed: extA},
		{Index: 1, Op: OpCreateNode, NodeID: "b", Proposed: b},
	})
	if vs[0].Auto || vs[1].Auto {
		t.Fatalf("%+v", vs)
	}
}

func TestProposalPolicyConfig(t *testing.T) {
	idx, dir := newModel(t)
	if info := idx.ProposalPolicyInfo(); info.AutoAccept || info.Effective {
		t.Fatalf("%+v", info)
	}
	info, err := idx.SetProposalPolicy(ProposalPolicy{AutoAccept: true})
	if err != nil || !info.AutoAccept || info.Effective {
		t.Fatalf("freeform has no build stage: %+v %v", info, err)
	}
	if err := idx.SetConfigKeys(map[string]any{"method": "design-first", "stage": "build"}); err != nil {
		t.Fatal(err)
	}
	idx = reopen(t, dir)
	if info := idx.ProposalPolicyInfo(); !info.Effective || info.Stage != BuildStage {
		t.Fatalf("%+v", info)
	}
	if err := idx.SetConfigKeys(map[string]any{"stage": "design"}); err != nil {
		t.Fatal(err)
	}
	if info := idx.ProposalPolicyInfo(); info.Effective || !strings.Contains(info.Reason, "build") {
		t.Fatalf("%+v", info)
	}
	if info, _ := idx.SetProposalPolicy(ProposalPolicy{}); info.AutoAccept {
		t.Fatal("off")
	}
	writeConfig(t, dir, `{"proposal_policy": "yes"}`)
	idx = reopen(t, dir)
	if idx.ProposalPolicy.AutoAccept || !strings.Contains(warningsText(idx), "proposal_policy") {
		t.Fatal("bad policy value must be ignored with a warning")
	}
}
