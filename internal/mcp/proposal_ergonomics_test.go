package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/KoJaco/ambit/internal/core"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSeedModelRelationshipUpsertAndIDs(t *testing.T) {
	idx, dir := openFixture(t)
	env := &toolEnv{idx: idx, dir: dir}
	ctx := context.Background()
	seed := seedModelIn{
		Transcript: "t",
		Nodes: []seedNodeIn{
			{ID: "a", Name: "A", Type: "service"},
			{ID: "b", Name: "B", Type: "service"},
			{ID: "c", Name: "C", Type: "service"},
		},
		Relationships: []seedRelIn{
			{From: "a", To: "b", Label: "calls", Kind: "sync"},
			{From: "a", To: "b", Label: "calls", Kind: "sync"},
		},
	}
	_, out, err := env.handleSeedModel(ctx, nil, seed)
	if err != nil {
		t.Fatal(err)
	}
	if res, err := idx.AcceptAll(out.ProposalID); err != nil || len(res.Failed) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
	if len(idx.Relationships) != 1 {
		t.Fatalf("duplicate edge: %+v", idx.Relationships)
	}
	id := string(idx.Relationships[0].ID)
	// Re-point the edge through relationship_id in seed_model.
	_, out, err = env.handleSeedModel(ctx, nil, seedModelIn{
		Transcript:    "t",
		Updates:       []updateNodeIn{{NodeID: "c", NewID: "gamma"}},
		Relationships: []seedRelIn{{RelationshipID: id, From: "a", To: "c", Label: "calls", Kind: "sync"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res, err := idx.AcceptAll(out.ProposalID); err != nil || len(res.Failed) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
	if len(idx.Relationships) != 1 || idx.Relationships[0].To != "gamma" || idx.HasNode("c") {
		t.Fatalf("relationship_id update / rename: %+v", idx.Relationships)
	}
}

func TestRenameNodeToolAndUpdateNewID(t *testing.T) {
	idx, dir := openFixture(t)
	env := &toolEnv{idx: idx, dir: dir}
	ctx := context.Background()
	for _, id := range []string{"a", "b"} {
		if _, err := idx.Create(core.CreateInput{ID: core.NodeID(id), Name: id, Type: "service"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := env.handleRenameNode(ctx, nil, renameNodeIn{NodeID: "a", NewID: "b"}); !errors.Is(err, core.ErrExists) {
		t.Fatalf("taken: %v", err)
	}
	_, out, err := env.handleRenameNode(ctx, nil, renameNodeIn{NodeID: "a", NewID: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	st, _ := idx.ProposalState(out.ProposalID)
	if st.Source != core.OpRenameNode || st.Operations[0].Op != core.OpRenameNode {
		t.Fatalf("state %+v", st)
	}
	name := "Beta"
	_, out2, err := env.handleUpdateNode(ctx, nil, updateNodeIn{NodeID: "b", NewID: "beta", Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	st, _ = idx.ProposalState(out2.ProposalID)
	if len(st.Operations) != 2 || st.Operations[0].Op != core.OpUpdateNode || st.Operations[1].Op != core.OpRenameNode {
		t.Fatalf("update + new_id ops: %+v", st.Operations)
	}
	// new_id alone is only a rename.
	_, out3, err := env.handleUpdateNode(ctx, nil, updateNodeIn{NodeID: "b", NewID: "bravo"})
	if err != nil {
		t.Fatal(err)
	}
	st, _ = idx.ProposalState(out3.ProposalID)
	if len(st.Operations) != 1 || st.Operations[0].Op != core.OpRenameNode {
		t.Fatalf("new_id only: %+v", st.Operations)
	}
}

func TestUpdateNodeErrorsNameTheRightTool(t *testing.T) {
	idx, dir := openFixture(t)
	env := &toolEnv{idx: idx, dir: dir}
	if _, err := idx.Create(core.CreateInput{ID: "a", Name: "A", Type: "service"}); err != nil {
		t.Fatal(err)
	}
	_, _, err := env.handleUpdateNode(context.Background(), nil, updateNodeIn{NodeID: "a", ID: "z"})
	if err == nil || !strings.Contains(err.Error(), "rename_node") || !errors.Is(err, core.ErrBadOperation) {
		t.Fatalf("id: %v", err)
	}
	rid := "missing"
	_, _, err = env.handleUpdateNode(context.Background(), nil, updateNodeIn{NodeID: "a", RelationshipID: &rid})
	if err == nil || !strings.Contains(err.Error(), "set_relationship") {
		t.Fatalf("relationship_id: %v", err)
	}
}

func TestWithdrawAndSupersedeTools(t *testing.T) {
	idx, dir := openFixture(t)
	mine := &toolEnv{idx: idx, dir: dir}
	theirs := &toolEnv{idx: idx, dir: dir}
	ctx := context.Background()
	node := createNodeIn{ID: "svc", Name: "Svc", Type: "service"}
	_, first, err := mine.handleCreateNode(ctx, nil, node)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := theirs.handleWithdrawProposal(ctx, nil, withdrawProposalIn{ProposalID: first.ProposalID}); !errors.Is(err, core.ErrBadOperation) {
		t.Fatalf("another session must not withdraw: %v", err)
	}
	node.proposalMetaIn = proposalMetaIn{Supersedes: first.ProposalID, ChangeSet: "svc-work"}
	_, second, err := mine.handleCreateNode(ctx, nil, node)
	if err != nil {
		t.Fatalf("supersede: %v", err)
	}
	if st, _ := idx.ProposalState(first.ProposalID); st.Status != core.ProposalWithdrawn {
		t.Fatalf("predecessor %+v", st)
	}
	_, w, err := mine.handleWithdrawProposal(ctx, nil, withdrawProposalIn{ProposalID: second.ProposalID})
	if err != nil || w.Status != core.ProposalWithdrawn {
		t.Fatalf("withdraw: %+v %v", w, err)
	}
	list, _ := idx.ListProposals()
	if len(list) != 0 {
		t.Fatalf("still pending: %+v", list)
	}
}

func TestToolSchemasCarryProposalFieldsAndRenameTools(t *testing.T) {
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "ambit", Version: "v1"}, nil)
	idx, dir := openFixture(t)
	registerTools(srv, &toolEnv{idx: idx, dir: dir})
	ctx := context.Background()
	t1, t2 := sdkmcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, t1, nil); err != nil {
		t.Fatal(err)
	}
	session, err := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test", Version: "v1"}, nil).Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	res, err := session.ListTools(ctx, &sdkmcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		raw, _ := json.Marshal(tool.InputSchema)
		schema := string(raw)
		switch tool.Name {
		case "seed_model", "create_node", "update_node", "delete_node", "set_relationship", "delete_relationship", "set_constraints", "rename_node":
			for _, field := range []string{`"change_set"`, `"supersedes"`, `"depends_on_proposal"`} {
				if !strings.Contains(schema, field) {
					t.Errorf("%s schema lacks %s: %s", tool.Name, field, schema)
				}
			}
		case "check_drift", "check_model":
			if !strings.Contains(schema, `"verbose_globs"`) {
				t.Errorf("%s schema lacks verbose_globs", tool.Name)
			}
		}
		if tool.Name == "update_node" && !strings.Contains(schema, `"new_id"`) {
			t.Errorf("update_node lacks new_id")
		}
	}
}

func TestCheckModelDriftCountsGlobsUnlessVerbose(t *testing.T) {
	idx, dir := openFixture(t)
	gitInit(t, dir)
	if _, err := idx.Create(core.CreateInput{Name: "API", Type: "service", Implementation: []string{"nope/**"}}); err != nil {
		t.Fatal(err)
	}
	env := &toolEnv{idx: idx, dir: dir}
	on := true
	_, res, err := env.handleCheckModel(context.Background(), nil, checkModelIn{IncludeDrift: &on})
	if err != nil || res.Drift == nil || res.Drift.UnmatchedCount != 1 || len(res.Drift.Unmatched) != 0 {
		t.Fatalf("default: %+v %v", res.Drift, err)
	}
	raw, _ := json.Marshal(res)
	if strings.Contains(string(raw), `"unmatched_globs":`) || !strings.Contains(string(raw), `"unmatched_globs_count":1`) {
		t.Fatalf("json %s", raw)
	}
	_, res, err = env.handleCheckModel(context.Background(), nil, checkModelIn{IncludeDrift: &on, VerboseGlobs: true})
	if err != nil || res.Drift == nil || len(res.Drift.Unmatched) != 1 {
		t.Fatalf("verbose: %+v %v", res.Drift, err)
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{{"init"}, {"config", "user.name", "t"}, {"config", "user.email", "t@e.x"}} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
}
