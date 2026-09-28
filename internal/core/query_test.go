package core

import "testing"

func TestLevelIsOneDrillDown(t *testing.T) {
	idx, _ := newModel(t)
	platform, err := idx.Create(CreateInput{Name: "Platform", Type: "boundary", Spec: "platform spec"})
	if err != nil {
		t.Fatal(err)
	}
	orders, err := idx.Create(CreateInput{Name: "Orders Service", Type: "service", ParentID: platform, Spec: "orders spec"})
	if err != nil {
		t.Fatal(err)
	}
	payments, err := idx.Create(CreateInput{
		Name:           "Payments Service",
		Type:           "service",
		ParentID:       platform,
		Status:         StatusDone,
		Implementation: []string{"src/payments/**"},
		Spec:           "payments spec",
	})
	if err != nil {
		t.Fatal(err)
	}
	capture, err := idx.Create(CreateInput{
		Name:           "Capture Worker",
		Type:           "worker",
		ParentID:       payments,
		Implementation: []string{"secret/capture-only/**"},
		Spec:           "OUTSIDE-LEVEL-capture",
	})
	if err != nil {
		t.Fatal(err)
	}
	billing, err := idx.Create(CreateInput{Name: "Billing Ledger", Type: "service", Spec: "OUTSIDE-LEVEL-billing"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idx.SetRelationship(SetRelationshipInput{From: orders, To: payments, Label: "requests authorisation", Kind: KindSync}); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.SetRelationship(SetRelationshipInput{From: orders, To: billing, Label: "settles", Kind: KindAsync}); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.SetRelationship(SetRelationshipInput{From: payments, To: capture, Label: "enqueues", Kind: KindAsync}); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.SetRelationship(SetRelationshipInput{From: billing, To: orders, Label: "notifies", Kind: KindData}); err != nil {
		t.Fatal(err)
	}

	root, err := idx.Level("")
	if err != nil {
		t.Fatal(err)
	}
	if root.Node != nil {
		t.Fatalf("root node %#v", root.Node)
	}
	if ids := viewIDs(root.Children); !sameIDs(ids, []NodeID{platform, billing}) {
		t.Fatalf("root children %v", ids)
	}
	if len(root.Relationships) != 0 {
		t.Fatalf("root relationships %#v", root.Relationships)
	}
	for _, c := range root.Children {
		if c.Spec != "" && (c.Spec == "OUTSIDE-LEVEL-capture" || c.ID == capture) {
			t.Fatalf("root included %s", c.ID)
		}
	}

	lvl, err := idx.Level(platform)
	if err != nil {
		t.Fatal(err)
	}
	if lvl.Node == nil || lvl.Node.ID != platform || lvl.Node.Name != "Platform" {
		t.Fatalf("node %#v", lvl.Node)
	}
	if ids := viewIDs(lvl.Children); !sameIDs(ids, []NodeID{orders, payments}) {
		t.Fatalf("children %v", ids)
	}
	if len(lvl.Relationships) != 1 || lvl.Relationships[0].From != orders || lvl.Relationships[0].To != payments {
		t.Fatalf("relationships %#v", lvl.Relationships)
	}
	if lvl.Relationships[0].Label != "requests authorisation" || lvl.Relationships[0].Kind != KindSync {
		t.Fatalf("edge %#v", lvl.Relationships[0])
	}
	for _, rel := range lvl.Relationships {
		if rel.From == billing || rel.To == billing || rel.From == capture || rel.To == capture {
			t.Fatalf("outside edge included %#v", rel)
		}
	}
	wantCross := map[string]bool{
		string(orders) + ">out>" + string(billing):   false,
		string(payments) + ">out>" + string(capture): false,
		string(orders) + ">in>" + string(billing):    false,
	}
	for _, c := range lvl.Crossings {
		key := string(c.NodeID) + ">" + c.Direction + ">" + string(c.OtherID)
		if _, ok := wantCross[key]; !ok {
			t.Fatalf("unexpected crossing %#v", c)
		}
		wantCross[key] = true
	}
	for key, seen := range wantCross {
		if !seen {
			t.Fatalf("missing crossing %s in %#v", key, lvl.Crossings)
		}
	}

	nested, err := idx.Level(payments)
	if err != nil {
		t.Fatal(err)
	}
	if ids := viewIDs(nested.Children); !sameIDs(ids, []NodeID{capture}) {
		t.Fatalf("nested children %v", ids)
	}
	if len(nested.Relationships) != 0 {
		t.Fatalf("nested relationships %#v", nested.Relationships)
	}

	if _, err := idx.Level("missing-node"); err == nil {
		t.Fatal("expected not found")
	}
}

func viewIDs(nodes []NodeView) []NodeID {
	out := make([]NodeID, len(nodes))
	for i, n := range nodes {
		out[i] = n.ID
	}
	return out
}

func sameIDs(got, want []NodeID) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[NodeID]int{}
	for _, id := range got {
		seen[id]++
	}
	for _, id := range want {
		seen[id]--
		if seen[id] < 0 {
			return false
		}
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}
