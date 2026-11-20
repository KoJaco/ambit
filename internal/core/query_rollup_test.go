package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type rollupModel struct {
	idx                                *Index
	a, a1, a2, a1x, a1xx, b, b1, b2, c NodeID
	rels                               map[string]RelID
}

// newRollupModel builds: A{a1{a1x{a1xx}}, a2}, B{b1, b2}, C, with relationships
// that cross, stay inside A, and roll up.
func newRollupModel(t *testing.T) *rollupModel {
	t.Helper()
	idx, _ := newModel(t)
	m := &rollupModel{idx: idx, rels: map[string]RelID{}}
	mk := func(name, typ string, parent NodeID) NodeID {
		id, err := idx.Create(CreateInput{Name: name, Type: typ, ParentID: parent})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	m.a = mk("A", "boundary", "")
	m.a1 = mk("A1", "service", m.a)
	m.a2 = mk("A2", "store", m.a)
	m.a1x = mk("A1X", "worker", m.a1)
	m.a1xx = mk("A1XX", "worker", m.a1x)
	m.b = mk("B", "boundary", "")
	m.b1 = mk("B1", "service", m.b)
	m.b2 = mk("B2", "service", m.b)
	m.c = mk("C", "service", "")
	rel := func(key string, from, to NodeID, kind string) {
		r, err := idx.SetRelationship(SetRelationshipInput{From: from, To: to, Label: key, Kind: kind})
		if err != nil {
			t.Fatal(err)
		}
		m.rels[key] = r.ID
	}
	rel("deep", m.a1x, m.b1, KindSync)
	rel("mid", m.a2, m.b2, KindAsync)
	rel("direct", m.a1, m.b1, KindSync)
	rel("inner", m.a1x, m.a1, KindData)
	rel("c-to-a1", m.c, m.a1, KindSync)
	return m
}

func edgeByPair(lvl Level, from, to NodeID) *EdgeView {
	for i := range lvl.Relationships {
		if lvl.Relationships[i].From == from && lvl.Relationships[i].To == to {
			return &lvl.Relationships[i]
		}
	}
	return nil
}

func childView(lvl Level, id NodeID) *NodeView {
	for i := range lvl.Children {
		if lvl.Children[i].ID == id {
			return &lvl.Children[i]
		}
	}
	return nil
}

func TestRollupGrandchildRelationshipsReachParentLevel(t *testing.T) {
	m := newRollupModel(t)
	lvl, err := m.idx.Level("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !sameIDs(viewIDs(lvl.Children), []NodeID{m.a, m.b, m.c}) {
		t.Fatalf("children %v", viewIDs(lvl.Children))
	}
	ab := edgeByPair(lvl, m.a, m.b)
	if ab == nil || !ab.Aggregated {
		t.Fatalf("A->B %#v", lvl.Relationships)
	}
	wantMembers := []RelID{m.rels["deep"], m.rels["direct"], m.rels["mid"]}
	if !reflect.DeepEqual(ab.Members, wantMembers) {
		t.Fatalf("members %v want %v", ab.Members, wantMembers)
	}
	if ab.Label != "3 relationships" || !reflect.DeepEqual(ab.Kinds, []string{KindAsync, KindSync}) || ab.Kind != "" {
		t.Fatalf("edge %#v", ab)
	}
	if ab.Drillable {
		t.Fatal("multi-member edge must not be drillable")
	}
	ca := edgeByPair(lvl, m.c, m.a)
	if ca == nil || !ca.Aggregated || ca.Label != "1 relationship" || !reflect.DeepEqual(ca.Members, []RelID{m.rels["c-to-a1"]}) || ca.Kind != KindSync {
		t.Fatalf("C->A %#v", ca)
	}
	if len(lvl.Relationships) != 2 || len(lvl.Crossings) != 0 {
		t.Fatalf("edges %#v crossings %#v", lvl.Relationships, lvl.Crossings)
	}
}

func TestRollupSameAnchorIsHiddenAndCounted(t *testing.T) {
	m := newRollupModel(t)
	lvl, err := m.idx.Level("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if a := childView(lvl, m.a); a.InternalRelCount != 1 || a.ChildCount != 2 {
		t.Fatalf("A %#v", a)
	}
	for _, id := range []NodeID{m.b, m.c} {
		if v := childView(lvl, id); v.InternalRelCount != 0 {
			t.Fatalf("%s count %d", id, v.InternalRelCount)
		}
	}
	for _, e := range lvl.Relationships {
		for _, mem := range e.Members {
			if mem == m.rels["inner"] {
				t.Fatalf("internal relationship drawn: %#v", e)
			}
		}
	}
}

func TestRollupDrillableSingleMember(t *testing.T) {
	m := newRollupModel(t)
	if _, err := m.idx.Create(CreateInput{Name: "Step", Type: "step", RelationshipID: m.rels["c-to-a1"]}); err != nil {
		t.Fatal(err)
	}
	lvl, err := m.idx.Level("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if ca := edgeByPair(lvl, m.c, m.a); ca == nil || !ca.Drillable {
		t.Fatalf("single drillable member lost: %#v", ca)
	}
	if ab := edgeByPair(lvl, m.a, m.b); ab.Drillable {
		t.Fatal("A->B must stay not drillable")
	}
}

func TestExpandedGroupExposesMembers(t *testing.T) {
	m := newRollupModel(t)
	lvl, err := m.idx.Level("", []NodeID{m.a})
	if err != nil {
		t.Fatal(err)
	}
	if !sameIDs(viewIDs(lvl.Children), []NodeID{m.a, m.a1, m.a2, m.b, m.c}) {
		t.Fatalf("children %v", viewIDs(lvl.Children))
	}
	if a := childView(lvl, m.a); !a.Expanded || a.GroupID != "" {
		t.Fatalf("A %#v", a)
	}
	for _, id := range []NodeID{m.a1, m.a2} {
		if v := childView(lvl, id); v.GroupID != m.a || v.Expanded {
			t.Fatalf("%s %#v", id, v)
		}
	}
	// Direct edges between visible nodes are not aggregated.
	direct := edgeByPair(lvl, m.a1, m.b)
	if direct == nil || !direct.Aggregated { // a1->b1 rolls up B
		t.Fatalf("a1->B %#v", lvl.Relationships)
	}
	ca1 := edgeByPair(lvl, m.c, m.a1)
	if ca1 == nil || ca1.Aggregated || ca1.ID != m.rels["c-to-a1"] || ca1.Label != "c-to-a1" {
		t.Fatalf("c->a1 %#v", ca1)
	}
	// a1x is hidden inside a1 (collapsed, has children): inner counted there.
	if v := childView(lvl, m.a1); v.InternalRelCount != 1 {
		t.Fatalf("a1 internal %d", v.InternalRelCount)
	}
	if v := childView(lvl, m.a); v.InternalRelCount != 0 {
		t.Fatalf("expanded group counts %d", v.InternalRelCount)
	}
}

func TestExpandNonAggregatedEdgeStaysUnmerged(t *testing.T) {
	m := newRollupModel(t)
	lvl, err := m.idx.Level("", []NodeID{m.a, m.b})
	if err != nil {
		t.Fatal(err)
	}
	// a1->b1 and a2->b2 are plain; a1x->b1 is rolled up (a1x hidden in a1).
	// deep (a1x->b1) rolls up onto a1->b1 as its own aggregate; direct stays plain.
	if e := edgeByPair(lvl, m.a1, m.b1); e == nil || !e.Aggregated || !reflect.DeepEqual(e.Members, []RelID{m.rels["deep"]}) {
		t.Fatalf("a1->b1 aggregate %#v", e)
	}
	plain := 0
	for _, e := range lvl.Relationships {
		if !e.Aggregated {
			plain++
		}
	}
	if plain != 3 { // direct, mid, c-to-a1
		t.Fatalf("plain edges %d in %#v", plain, lvl.Relationships)
	}
}

func TestExpandIgnoresInvalidIDs(t *testing.T) {
	m := newRollupModel(t)
	base, err := m.idx.Level("", nil)
	if err != nil {
		t.Fatal(err)
	}
	// unknown, leaf, not visible (a1 while A collapsed), too deep (a1x is depth 3).
	for _, ids := range [][]NodeID{{"nope"}, {m.c}, {m.a1}, {m.a2}, {"", "x"}} {
		got, err := m.idx.Level("", ids)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, base) {
			t.Fatalf("expand %v changed the level", ids)
		}
	}
	got, err := m.idx.Level("", []NodeID{m.a, m.a1, m.a1x})
	if err != nil {
		t.Fatal(err)
	}
	// a1 is depth 2 and honored; a1x is depth 3 and ignored.
	if !sameIDs(viewIDs(got.Children), []NodeID{m.a, m.a1, m.a1x, m.a2, m.b, m.c}) {
		t.Fatalf("children %v", viewIDs(got.Children))
	}
	if childView(got, m.a1x).Expanded || childView(got, m.a1x).GroupID != m.a1 {
		t.Fatalf("a1x %#v", childView(got, m.a1x))
	}
	if childView(got, m.a1xx) != nil {
		t.Fatal("too-deep group was expanded")
	}
}

func TestExpandOrderAndDuplicatesDoNotMatter(t *testing.T) {
	m := newRollupModel(t)
	one, err := m.idx.Level("", []NodeID{m.a, m.a1, m.b})
	if err != nil {
		t.Fatal(err)
	}
	two, err := m.idx.Level("", []NodeID{m.b, m.a1, m.a, m.a, m.b, "nope"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(one, two) {
		t.Fatalf("order dependent:\n%#v\n%#v", one, two)
	}
}

func TestCrossingsAreAnchoredOnVisibleNodes(t *testing.T) {
	m := newRollupModel(t)
	// Level of A: B and C are outside A's subtree.
	lvl, err := m.idx.Level(m.a, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !sameIDs(viewIDs(lvl.Children), []NodeID{m.a1, m.a2}) {
		t.Fatalf("children %v", viewIDs(lvl.Children))
	}
	got := map[string]bool{}
	for _, c := range lvl.Crossings {
		got[string(c.NodeID)+">"+c.Direction+">"+string(c.OtherID)+">"+string(c.RelID)] = true
	}
	want := []string{
		string(m.a1) + ">out>" + string(m.b1) + ">" + string(m.rels["deep"]),
		string(m.a2) + ">out>" + string(m.b2) + ">" + string(m.rels["mid"]),
		string(m.a1) + ">out>" + string(m.b1) + ">" + string(m.rels["direct"]),
		string(m.a1) + ">in>" + string(m.c) + ">" + string(m.rels["c-to-a1"]),
	}
	if len(got) != len(want) {
		t.Fatalf("crossings %#v", lvl.Crossings)
	}
	for _, k := range want {
		if !got[k] {
			t.Fatalf("missing crossing %s in %#v", k, lvl.Crossings)
		}
	}
	// Expanding a1 moves the deep crossing onto a1x.
	lvl, err = m.idx.Level(m.a, []NodeID{m.a1})
	if err != nil {
		t.Fatal(err)
	}
	var onA1x bool
	for _, c := range lvl.Crossings {
		if c.RelID == m.rels["deep"] && c.NodeID == m.a1x {
			onA1x = true
		}
	}
	if !onA1x {
		t.Fatalf("deep crossing not on a1x: %#v", lvl.Crossings)
	}
	if e := edgeByPair(lvl, m.a1x, m.a1); e == nil || e.Aggregated {
		t.Fatalf("inner edge %#v", lvl.Relationships)
	}
}

func TestTypeMixIsDeterministic(t *testing.T) {
	m := newRollupModel(t)
	lvl, err := m.idx.Level("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := childView(lvl, m.a).TypeMix; !reflect.DeepEqual(got, []TypeCount{{"service", 1}, {"store", 1}}) {
		t.Fatalf("mix %#v", got)
	}
	if childView(lvl, m.c).TypeMix != nil {
		t.Fatal("leaf has a mix")
	}
}

func TestViewDefaultsFromConfig(t *testing.T) {
	idx, dir := newModel(t)
	lvl, err := idx.Level("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if lvl.ViewDefaults != (ViewDefaults{GroupMax: 8, LevelBudget: 24}) {
		t.Fatalf("defaults %#v", lvl.ViewDefaults)
	}

	path := filepath.Join(dir, ".arch", "config.json")
	writeObj(t, path, map[string]any{"group_max": 5, "level_budget": 40, "ui": "dark"})
	reopened := reopen(t, dir)
	lvl, _ = reopened.Level("", nil)
	if lvl.ViewDefaults != (ViewDefaults{GroupMax: 5, LevelBudget: 40}) {
		t.Fatalf("override %#v", lvl.ViewDefaults)
	}
	encoded, err := encodeObject(nil, nil, reopened.configUnknown)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	again := reopen(t, dir)
	if again.GroupMax != 5 || again.LevelBudget != 40 || string(again.configUnknown["ui"]) != `"dark"` {
		t.Fatalf("round trip %d %d %v", again.GroupMax, again.LevelBudget, again.configUnknown)
	}
	if len(again.Warnings()) != 0 {
		t.Fatalf("warnings %v", again.Warnings())
	}

	writeObj(t, path, map[string]any{"group_max": -3, "level_budget": "lots"})
	bad := reopen(t, dir)
	if bad.GroupMax != 8 || bad.LevelBudget != 24 || len(bad.Warnings()) != 2 {
		t.Fatalf("invalid config %d %d %v", bad.GroupMax, bad.LevelBudget, bad.Warnings())
	}
}
