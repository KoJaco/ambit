package mcp

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KoJaco/ambit/internal/core"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func openFixture(t *testing.T) (*core.Index, string) {
	t.Helper()
	dir := t.TempDir()
	if err := core.Init(dir); err != nil {
		t.Fatal(err)
	}
	idx, err := core.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return idx, dir
}

func nodeFiles(dir string) ([]string, error) {
	nodes := filepath.Join(dir, ".arch", "nodes")
	entries, err := os.ReadDir(nodes)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}

func TestCreateNodeStagesProposal(t *testing.T) {
	idx, dir := openFixture(t)
	env := &toolEnv{idx: idx, dir: dir}
	before, _ := nodeFiles(dir)
	_, out, err := env.handleCreateNode(context.Background(), nil, createNodeIn{
		Name: "Refunds Service", Type: "service", Spec: "# Refunds",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Applied || !strings.Contains(strings.ToLower(out.Message), "not") {
		t.Fatalf("response %+v", out)
	}
	after, _ := nodeFiles(dir)
	if strings.Join(before, ",") != strings.Join(after, ",") {
		t.Fatalf("nodes changed")
	}
	list, err := idx.ListProposals()
	if err != nil || len(list) != 1 || list[0].Source != core.OpCreateNode {
		t.Fatalf("proposals %+v err %v", list, err)
	}
}

func TestCreateNodeAcceptsIDAndRejectsCollisions(t *testing.T) {
	idx, dir := openFixture(t)
	env := &toolEnv{idx: idx, dir: dir}
	ctx := context.Background()
	if _, _, err := env.handleCreateNode(ctx, nil, createNodeIn{ID: "fixed", Name: "X", Type: "service"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := env.handleCreateNode(ctx, nil, createNodeIn{ID: "fixed", Name: "Y", Type: "service"}); !errors.Is(err, core.ErrExists) {
		t.Fatalf("collision with pending create: %v", err)
	}
	if _, _, err := env.handleCreateNode(ctx, nil, createNodeIn{ID: "Bad_ID", Name: "Z", Type: "service"}); !errors.Is(err, core.ErrBadOperation) {
		t.Fatalf("bad slug: %v", err)
	}
	list, _ := idx.ListProposals()
	res, err := idx.AcceptAll(list[0].ID)
	if err != nil || len(res.Applied) != 1 || !idx.HasNode("fixed") {
		t.Fatalf("accept %+v %v", res, err)
	}
	if _, _, err := env.handleCreateNode(ctx, nil, createNodeIn{ID: "fixed", Name: "Y", Type: "service"}); !errors.Is(err, core.ErrExists) {
		t.Fatalf("collision with node: %v", err)
	}
}

func TestSeedModelUsesSettableIDsAndRestructures(t *testing.T) {
	idx, dir := openFixture(t)
	env := &toolEnv{idx: idx, dir: dir}
	ctx := context.Background()
	a, _ := idx.Create(core.CreateInput{Name: "A", Type: "service"})
	b, _ := idx.Create(core.CreateInput{Name: "B", Type: "service"})
	rel, err := idx.SetRelationship(core.SetRelationshipInput{From: a, To: b, Label: "calls"})
	if err != nil {
		t.Fatal(err)
	}
	rename := "A2"
	_, out, err := env.handleSeedModel(ctx, nil, seedModelIn{
		Transcript: "restructure",
		Nodes: []seedNodeIn{
			{ID: "zone", Name: "Zone", Type: "boundary"},
			{ID: "worker", Name: "Worker", Type: "service", ParentID: "zone"},
		},
		Relationships:       []seedRelIn{{From: "worker", To: string(a), Label: "uses"}},
		Updates:             []updateNodeIn{{NodeID: string(a), Name: &rename}},
		DeleteRelationships: []string{string(rel.ID)},
		DeleteNodes:         []string{string(b)},
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := idx.AcceptAll(out.ProposalID)
	if err != nil || len(res.Failed) != 0 || len(res.Applied) != 6 {
		t.Fatalf("accept %+v %v", res, err)
	}
	if n, err := idx.Node("worker"); err != nil || n.ParentID != "zone" {
		t.Fatalf("worker %+v %v", n, err)
	}
	if n, _ := idx.Node(a); n.Name != "A2" || idx.HasNode(b) {
		t.Fatalf("restructure not applied")
	}
}

func TestDeleteRelationshipStagesProposal(t *testing.T) {
	idx, dir := openFixture(t)
	env := &toolEnv{idx: idx, dir: dir}
	a, _ := idx.Create(core.CreateInput{Name: "A", Type: "service"})
	b, _ := idx.Create(core.CreateInput{Name: "B", Type: "service"})
	rel, err := idx.SetRelationship(core.SetRelationshipInput{From: a, To: b, Label: "calls"})
	if err != nil {
		t.Fatal(err)
	}
	_, out, err := env.handleDeleteRelationship(context.Background(), nil, deleteRelationshipIn{RelationshipID: string(rel.ID)})
	if err != nil || out.Applied {
		t.Fatalf("out %+v err %v", out, err)
	}
	if len(idx.Relationships) != 1 {
		t.Fatal("staging changed the model")
	}
	list, _ := idx.ListProposals()
	if len(list) != 1 || list[0].Source != core.OpDeleteRelationship {
		t.Fatalf("list %+v", list)
	}
	if _, _, err := env.handleDeleteRelationship(context.Background(), nil, deleteRelationshipIn{RelationshipID: "nope"}); !errors.Is(err, core.ErrRelationshipNotFound) {
		t.Fatalf("err %v", err)
	}
}

func TestUpdateNodeRejectsStatus(t *testing.T) {
	idx, dir := openFixture(t)
	id, err := idx.Create(core.CreateInput{Name: "Payments", Type: "service"})
	if err != nil {
		t.Fatal(err)
	}
	env := &toolEnv{idx: idx, dir: dir}
	done := "done"
	_, _, err = env.handleUpdateNode(context.Background(), nil, updateNodeIn{
		NodeID: string(id), Status: &done,
	})
	if err == nil || !strings.Contains(err.Error(), "update_node_status") {
		t.Fatalf("err %v", err)
	}
}

func TestUpdateNodeStatusWritesCanonical(t *testing.T) {
	idx, dir := openFixture(t)
	id, err := idx.Create(core.CreateInput{Name: "Payments", Type: "service"})
	if err != nil {
		t.Fatal(err)
	}
	env := &toolEnv{idx: idx, dir: dir}
	_, out, err := env.handleUpdateNodeStatus(context.Background(), nil, updateStatusIn{
		NodeID: string(id), Status: "done",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "done" {
		t.Fatalf("out %+v", out)
	}
	n, err := idx.Node(id)
	if err != nil || n.Status != core.StatusDone {
		t.Fatalf("node %+v err %v", n, err)
	}
	list, err := idx.ListProposals()
	if err != nil || len(list) != 0 {
		t.Fatalf("proposals %+v", list)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".arch", "nodes", string(id)+".json"))
	if err != nil || !strings.Contains(string(raw), `"done"`) {
		t.Fatalf("file %s err %v", raw, err)
	}
}

func TestUpdateNodeStatusBadValue(t *testing.T) {
	idx, dir := openFixture(t)
	id, err := idx.Create(core.CreateInput{Name: "Payments", Type: "service"})
	if err != nil {
		t.Fatal(err)
	}
	env := &toolEnv{idx: idx, dir: dir}
	_, _, err = env.handleUpdateNodeStatus(context.Background(), nil, updateStatusIn{
		NodeID: string(id), Status: "finished",
	})
	if err == nil || !strings.Contains(err.Error(), "draft") {
		t.Fatalf("err %v", err)
	}
}

func TestUnknownNodeErrors(t *testing.T) {
	idx, dir := openFixture(t)
	env := &toolEnv{idx: idx, dir: dir}
	_, _, err := env.handleGetContext(context.Background(), nil, getContextIn{NodeID: "nope-id"})
	if err == nil || !strings.Contains(err.Error(), "nope-id") {
		t.Fatalf("get_context err %v", err)
	}
	_, _, err = env.handleCheckScope(context.Background(), nil, checkScopeIn{NodeID: "nope-id", Files: []string{"a.ts"}})
	if err == nil || !strings.Contains(err.Error(), "nope-id") {
		t.Fatalf("check_scope err %v", err)
	}
	_, _, err = env.handleUpdateNodeStatus(context.Background(), nil, updateStatusIn{NodeID: "nope-id", Status: "done"})
	if err == nil || !strings.Contains(err.Error(), "nope-id") {
		t.Fatalf("update_node_status err %v", err)
	}
}

func TestCheckScopeMatchesCore(t *testing.T) {
	idx := &core.Index{
		Nodes: map[core.NodeID]*core.Node{},
		Order: nil,
	}
	pay := &core.Node{ID: "payments-service", Name: "pay", Implementation: []string{"src/payments/**"}}
	ord := &core.Node{ID: "orders-service", Name: "ord", Implementation: []string{"src/orders/**"}}
	bill := &core.Node{ID: "billing", Name: "bill", Implementation: []string{"src/billing/**"}, Protected: true}
	idx.Nodes[pay.ID] = pay
	idx.Nodes[ord.ID] = ord
	idx.Nodes[bill.ID] = bill
	idx.Order = []core.NodeID{pay.ID, ord.ID, bill.ID}

	files := []string{"src/payments/a.ts", "src/billing/fee.ts", "src/unmapped/x.ts"}
	coreRes, err := core.CheckScope("payments-service", files, idx)
	if err != nil {
		t.Fatal(err)
	}
	mcpRes, err := CheckScopeForTool(idx, "payments-service", files)
	if err != nil {
		t.Fatal(err)
	}
	if len(coreRes) != len(mcpRes) {
		t.Fatalf("len %d vs %d", len(coreRes), len(mcpRes))
	}
	for i := range coreRes {
		if coreRes[i].Path != mcpRes[i].Path || coreRes[i].Kind != mcpRes[i].Kind {
			t.Fatalf("%d: %+v vs %+v", i, coreRes[i], mcpRes[i])
		}
	}
}

func TestMCPServerListsTools(t *testing.T) {
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "ambit", Version: "v1"}, nil)
	idx, dir := openFixture(t)
	registerTools(srv, &toolEnv{idx: idx, dir: dir})
	ctx := context.Background()
	t1, t2 := sdkmcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, t1, nil); err != nil {
		t.Fatal(err)
	}
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test", Version: "v1"}, nil)
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	res, err := session.ListTools(ctx, &sdkmcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"seed_model": true, "create_node": true, "update_node": true, "delete_node": true,
		"set_relationship": true, "delete_relationship": true, "get_context": true, "check_scope": true, "update_node_status": true,
		"get_graph": true, "list_proposals": true, "list_icons": true, "check_drift": true,
	}
	for _, tool := range res.Tools {
		delete(want, tool.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing tools %v (got %d)", want, len(res.Tools))
	}
	if len(res.Tools) != 13 {
		t.Fatalf("got %d tools, want 13", len(res.Tools))
	}
}

func TestSeedModelRequiresTranscriptAndNodes(t *testing.T) {
	idx, dir := openFixture(t)
	env := &toolEnv{idx: idx, dir: dir}
	_, _, err := env.handleSeedModel(context.Background(), nil, seedModelIn{})
	if err == nil {
		t.Fatal("expected error")
	}
	_, _, err = env.handleSeedModel(context.Background(), nil, seedModelIn{Transcript: "hi", Nodes: nil})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAuthoringToolsSetSource(t *testing.T) {
	idx, dir := openFixture(t)
	env := &toolEnv{idx: idx, dir: dir}
	id, err := idx.Create(core.CreateInput{Name: "Orders", Type: "service"})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("delete", func(t *testing.T) {
		if _, _, err := env.handleDeleteNode(context.Background(), nil, deleteNodeIn{NodeID: string(id)}); err != nil {
			t.Fatal(err)
		}
		list, err := idx.ListProposals()
		if err != nil || len(list) != 1 || list[0].Source != core.OpDeleteNode {
			t.Fatalf("list %+v err %v", list, err)
		}
	})
	t.Run("relationship", func(t *testing.T) {
		idx2, dir2 := openFixture(t)
		env2 := &toolEnv{idx: idx2, dir: dir2}
		a, _ := idx2.Create(core.CreateInput{Name: "A", Type: "service"})
		b, _ := idx2.Create(core.CreateInput{Name: "B", Type: "service"})
		if _, _, err := env2.handleSetRelationship(context.Background(), nil, setRelationshipIn{
			From: string(a), To: string(b), Label: "calls", Kind: "sync",
		}); err != nil {
			t.Fatal(err)
		}
		list, err := idx2.ListProposals()
		if err != nil || len(list) != 1 || list[0].Source != core.OpSetRelationship {
			t.Fatalf("list %+v err %v", list, err)
		}
	})
}

func TestGetContextObligations(t *testing.T) {
	idx, dir := openFixture(t)
	parent, _ := idx.Create(core.CreateInput{Name: "Platform", Type: "boundary"})
	idx.Create(core.CreateInput{Name: "Pay", Type: "service", ParentID: parent, Implementation: []string{"src/payments/**"}, Scope: []string{"src/payments/api/**"}})
	idx.Create(core.CreateInput{Name: "Bill", Type: "service", ParentID: parent, Implementation: []string{"src/billing/**"}, Protected: true})
	env := &toolEnv{idx: idx, dir: dir}
	_, out, err := env.handleGetContext(context.Background(), nil, getContextIn{NodeID: "pay"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Brief, "src/payments/api/**") || !strings.Contains(out.Brief, "check_scope") {
		t.Fatalf("brief %q", out.Brief)
	}
}

func TestAuthoringDescriptionsCarryShallowGuidance(t *testing.T) {
	for name, d := range map[string]string{
		"seed_model": seedModelDescription, "create_node": createNodeDescription,
		"update_node": updateNodeDescription, "set_relationship": setRelationshipDescription,
	} {
		if !strings.Contains(d, core.ShallowModelGuidance) {
			t.Errorf("%s description lacks guidance", name)
		}
	}
}

func TestReadToolsGraphProposalsIcons(t *testing.T) {
	idx, dir := openFixture(t)
	env := &toolEnv{idx: idx, dir: dir}
	ctx := context.Background()
	if _, err := idx.Create(core.CreateInput{Name: "API", Type: "service"}); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Create(core.CreateInput{Name: "Web", Type: "service"}); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.SetRelationship(core.SetRelationshipInput{From: "web", To: "api", Label: "calls", Kind: core.KindSync}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := env.handleCreateNode(ctx, nil, createNodeIn{Name: "Queue", Type: "queue"}); err != nil {
		t.Fatal(err)
	}
	_, g, err := env.handleGetGraph(ctx, nil, getGraphIn{})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 2 || len(g.Relationships) != 1 || len(g.Pending) != 1 || g.Pending[0].Operations != 1 {
		t.Fatalf("graph %+v", g)
	}
	_, p, err := env.handleListProposals(ctx, nil, listProposalsIn{})
	if err != nil || len(p.Proposals) != 1 {
		t.Fatalf("proposals %+v err %v", p, err)
	}
	_, ic, err := env.handleListIcons(ctx, nil, listIconsIn{})
	if err != nil || len(ic.Icons) != 40 || ic.Icons[0].Label == "" {
		t.Fatalf("icons %d err %v", len(ic.Icons), err)
	}
}

func TestCheckDriftTool(t *testing.T) {
	idx, dir := openFixture(t)
	for _, args := range [][]string{{"init"}, {"config", "user.name", "t"}, {"config", "user.email", "t@e.x"}} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Create(core.CreateInput{Name: "API", Type: "service", Implementation: []string{"nope/**"}}); err != nil {
		t.Fatal(err)
	}
	c := exec.Command("git", "add", "-A")
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	env := &toolEnv{idx: idx, dir: dir}
	_, r, err := env.handleCheckDrift(context.Background(), nil, checkDriftIn{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(r.Unowned, ",") != ".gitignore,main.go" || len(r.Unmatched) != 1 {
		t.Fatalf("report %+v", r)
	}
}
