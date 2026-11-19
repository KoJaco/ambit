package core

import (
	"errors"
	"strings"
	"testing"
)

func groupFixture(t *testing.T) (*Index, string) {
	t.Helper()
	idx, dir := newModel(t)
	for _, c := range []CreateInput{
		{Name: "Platform", Type: "boundary"},
		{Name: "API", Type: "service", ParentID: "platform"},
		{Name: "Worker", Type: "service", ParentID: "platform", Protected: true, Implementation: []string{"worker/**"}},
		{Name: "Handlers", Type: "module", ParentID: "api"},
		{Name: "Db", Type: "datastore"},
	} {
		if _, err := idx.Create(c); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := idx.SetRelationship(SetRelationshipInput{From: "api", To: "worker", Label: "enqueues", Kind: KindAsync}); err != nil {
		t.Fatal(err)
	}
	return idx, dir
}

func TestStageGroupAndAccept(t *testing.T) {
	idx, dir := groupFixture(t)
	pid, err := idx.StageGroup(GroupInput{NodeIDs: []NodeID{"api", "worker"}, Name: "Backend", Color: "blue"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := idx.Nodes["backend"]; ok {
		t.Fatal("staging wrote the container")
	}
	d, err := idx.ProposalDiff(pid)
	if err != nil {
		t.Fatal(err)
	}
	if d.Source != SourceGroupNodes || len(d.Operations) != 3 {
		t.Fatalf("diff %+v", d)
	}
	c := d.Operations[0]
	if c.Op != OpCreateNode || c.NodeID != "backend" || c.Proposed.ParentID != "platform" ||
		c.Proposed.Type != "container" || c.Proposed.Status != string(StatusDraft) || c.Proposed.Renderer != "" {
		t.Fatalf("create %+v", c.Proposed)
	}
	for _, op := range d.Operations[1:] {
		if op.Op != OpUpdateNode || len(op.Fields) != 1 || op.Fields[0] != "parent_id" || op.Proposed.ParentID != "backend" {
			t.Fatalf("update %+v", op)
		}
	}
	res, err := idx.AcceptAll(pid)
	if err != nil || len(res.Applied) != 3 {
		t.Fatalf("accept %+v %v", res, err)
	}
	got := reopen(t, dir)
	if got.Nodes["api"].ParentID != "backend" || got.Nodes["worker"].ParentID != "backend" || got.Nodes["backend"].ParentID != "platform" {
		t.Fatal("tree wrong")
	}
	if got.Nodes["handlers"].ParentID != "api" {
		t.Fatal("grandchild moved")
	}
	if len(got.Relationships) != 1 || got.Relationships[0].From != "api" || got.Relationships[0].To != "worker" {
		t.Fatalf("rels %+v", got.Relationships)
	}
	lvl, err := got.Level("platform", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range lvl.Children {
		if n.ID == "backend" && n.ChildCount != 2 {
			t.Fatalf("child count %d", n.ChildCount)
		}
	}
}

func TestStageGroupRootAndUniqueID(t *testing.T) {
	idx, _ := groupFixture(t)
	if _, err := idx.Create(CreateInput{Name: "Backend", Type: "container"}); err != nil {
		t.Fatal(err)
	}
	pid, err := idx.StageGroup(GroupInput{NodeIDs: []NodeID{"platform", "db"}, Name: "Backend"})
	if err != nil {
		t.Fatal(err)
	}
	d, _ := idx.ProposalDiff(pid)
	if d.Operations[0].NodeID != "backend-2" || d.Operations[0].Proposed.ParentID != "" {
		t.Fatalf("%+v", d.Operations[0])
	}
}

func TestStageGroupValidation(t *testing.T) {
	idx, _ := groupFixture(t)
	cases := []struct {
		name string
		in   GroupInput
		is   error
		msg  string
	}{
		{"different parents", GroupInput{NodeIDs: []NodeID{"api", "db"}, Name: "X"}, ErrBadOperation, "share one parent"},
		{"ancestor", GroupInput{NodeIDs: []NodeID{"platform", "api"}, Name: "X"}, ErrBadOperation, "ancestor"},
		{"one node", GroupInput{NodeIDs: []NodeID{"api"}, Name: "X"}, ErrBadOperation, "at least 2"},
		{"unknown", GroupInput{NodeIDs: []NodeID{"api", "nope"}, Name: "X"}, ErrNotFound, ""},
		{"duplicate", GroupInput{NodeIDs: []NodeID{"api", "api"}, Name: "X"}, ErrBadOperation, "duplicate"},
		{"empty name", GroupInput{NodeIDs: []NodeID{"api", "worker"}, Name: " "}, ErrEmptyName, ""},
		{"bad icon", GroupInput{NodeIDs: []NodeID{"api", "worker"}, Name: "X", Icon: "nope-icon"}, ErrInvalidPresentation, ""},
	}
	for _, c := range cases {
		_, err := idx.StageGroup(c.in)
		if !errors.Is(err, c.is) || (c.msg != "" && !strings.Contains(err.Error(), c.msg)) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	list, _ := idx.ListProposals()
	if len(list) != 0 {
		t.Fatalf("failed validation left %d proposals", len(list))
	}
}

func TestStageUngroupKeepAndDelete(t *testing.T) {
	idx, dir := groupFixture(t)
	// api has child handlers; relationship api->worker is attached to api.
	pid, err := idx.StageUngroup(UngroupInput{NodeID: "api"})
	if err != nil {
		t.Fatal(err)
	}
	if res, err := idx.AcceptAll(pid); err != nil || len(res.Applied) != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	got := reopen(t, dir)
	if got.Nodes["handlers"].ParentID != "platform" {
		t.Fatalf("parent %q", got.Nodes["handlers"].ParentID)
	}
	if _, ok := got.Nodes["api"]; !ok || len(got.Relationships) != 1 {
		t.Fatal("container or relationship lost")
	}

	// Delete variant on a container without relationships.
	if _, err := got.Create(CreateInput{Name: "Box", Type: "container", ParentID: "platform"}); err != nil {
		t.Fatal(err)
	}
	if _, err := got.Create(CreateInput{Name: "Inner", Type: "module", ParentID: "box"}); err != nil {
		t.Fatal(err)
	}
	pid, err = got.StageUngroup(UngroupInput{NodeID: "box", DeleteContainer: true})
	if err != nil {
		t.Fatal(err)
	}
	d, _ := got.ProposalDiff(pid)
	if len(d.Operations) != 2 || d.Operations[1].Op != OpDeleteNode {
		t.Fatalf("%+v", d.Operations)
	}
	if res, err := got.AcceptAll(pid); err != nil || len(res.Applied) != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	if _, ok := got.Nodes["box"]; ok || got.Nodes["inner"].ParentID != "platform" {
		t.Fatal("ungroup delete wrong")
	}
}

func TestStageUngroupRefusals(t *testing.T) {
	idx, _ := groupFixture(t)
	if _, err := idx.StageUngroup(UngroupInput{NodeID: "api", DeleteContainer: true}); !errors.Is(err, ErrBadOperation) {
		t.Fatalf("delete with relationships: %v", err)
	}
	if _, err := idx.StageUngroup(UngroupInput{NodeID: "db"}); !errors.Is(err, ErrBadOperation) {
		t.Fatalf("no children: %v", err)
	}
	if _, err := idx.StageUngroup(UngroupInput{NodeID: "nope"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
}

func TestStageReparent(t *testing.T) {
	idx, dir := groupFixture(t)
	if _, err := idx.StageReparent("handlers", "api"); !errors.Is(err, ErrBadOperation) {
		t.Fatalf("noop: %v", err)
	}
	if _, err := idx.StageReparent("platform", "api"); !errors.Is(err, ErrCycle) {
		t.Fatalf("cycle: %v", err)
	}
	pid, err := idx.StageReparent("handlers", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idx.AcceptAll(pid); err != nil {
		t.Fatal(err)
	}
	if reopen(t, dir).Nodes["handlers"].ParentID != "" {
		t.Fatal("not moved to root")
	}
}

func TestGuidanceInBrief(t *testing.T) {
	idx, _ := groupFixture(t)
	text, err := idx.Brief("api")
	if err != nil || !strings.Contains(text, ShallowModelGuidance) {
		t.Fatalf("brief missing guidance: %v", err)
	}
}
