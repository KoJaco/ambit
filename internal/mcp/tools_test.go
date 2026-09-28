package mcp

import (
	"context"
	"os"
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

func TestCreateNodeRejectsID(t *testing.T) {
	idx, dir := openFixture(t)
	env := &toolEnv{idx: idx, dir: dir}
	_, _, err := env.handleCreateNode(context.Background(), nil, createNodeIn{
		ID: "fixed", Name: "X", Type: "service",
	})
	if err == nil {
		t.Fatal("expected error")
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

func TestMCPServerListsEightTools(t *testing.T) {
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
		"set_relationship": true, "get_context": true, "check_scope": true, "update_node_status": true,
	}
	for _, tool := range res.Tools {
		delete(want, tool.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing tools %v (got %d)", want, len(res.Tools))
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
