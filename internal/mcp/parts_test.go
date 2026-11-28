package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/KoJaco/ambit/internal/core"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// sessionFor connects a client to a server over idx started with role (each call is a new
// MCP session with its own origin).
func sessionFor(t *testing.T, idx *core.Index, dir, role string) *sdkmcp.ClientSession {
	t.Helper()
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "ambit", Version: "v1"}, nil)
	registerTools(srv, &toolEnv{idx: idx, dir: dir, role: role})
	t1, t2 := sdkmcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, t1, nil); err != nil {
		t.Fatal(err)
	}
	session, err := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test", Version: "v1"}, nil).Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func toolNames(t *testing.T, s *sdkmcp.ClientSession) map[string]bool {
	t.Helper()
	res, err := s.ListTools(context.Background(), &sdkmcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, tool := range res.Tools {
		out[tool.Name] = true
	}
	return out
}

func TestAcceptProposalOnlyRegisteredForArchitectRole(t *testing.T) {
	idx, dir := openFixture(t)
	def := toolNames(t, sessionFor(t, idx, dir, ""))
	if def["accept_proposal"] {
		t.Fatal("accept_proposal must not exist by default")
	}
	arch := toolNames(t, sessionFor(t, idx, dir, RoleArchitect))
	if !arch["accept_proposal"] {
		t.Fatal("architect role must register accept_proposal")
	}
	if len(arch) != len(def)+1 {
		t.Fatalf("architect adds only accept_proposal: %d vs %d", len(arch), len(def))
	}
	// No tool, in any role, can change the policy, the stage, the scope or accept via the
	// ordinary path.
	for name := range arch {
		if strings.Contains(name, "policy") || name == "set_stage" || name == "set_scope" || name == "accept_all" {
			t.Fatalf("unexpected tool %s", name)
		}
	}
	// The handler itself refuses outside the architect role (defence in depth).
	env := &toolEnv{idx: idx, dir: dir}
	if _, _, err := env.handleAcceptProposal(context.Background(), nil, acceptProposalIn{ProposalID: "p-20260101-0000-abcd"}); err == nil {
		t.Fatal("handler must refuse without the role")
	}
}

func buildStageFixture(t *testing.T) (*core.Index, string) {
	t.Helper()
	idx, dir := openFixture(t)
	if _, err := idx.Create(core.CreateInput{ID: "svc", Name: "Svc", Type: "service", Spec: "# svc\n", Implementation: []string{"src/svc/**"}}); err != nil {
		t.Fatal(err)
	}
	if err := idx.SetConfigKeys(map[string]any{"method": "design-first", "stage": "build"}); err != nil {
		t.Fatal(err)
	}
	return idx, dir
}

func TestAcceptProposalPolicyGateAndSelfAcceptRefusal(t *testing.T) {
	idx, dir := buildStageFixture(t)
	writer := sessionFor(t, idx, dir, "")
	architect := sessionFor(t, idx, dir, RoleArchitect)

	staged := callTool(t, writer, "update_node", map[string]any{"node_id": "svc", "spec": "# svc v2\n"})
	pid, _ := staged["proposal_id"].(string)
	if pid == "" {
		t.Fatalf("%v", staged)
	}
	callErr := func(s *sdkmcp.ClientSession, id string) string {
		t.Helper()
		res, err := s.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "accept_proposal", Arguments: map[string]any{"proposal_id": id}})
		if err != nil {
			return err.Error()
		}
		if res.IsError {
			var b strings.Builder
			for _, c := range res.Content {
				if tc, ok := c.(*sdkmcp.TextContent); ok {
					b.WriteString(tc.Text)
				}
			}
			return b.String()
		}
		return ""
	}
	// Policy off: refused.
	if msg := callErr(architect, pid); !strings.Contains(msg, "off") {
		t.Fatalf("policy off: %q", msg)
	}
	if _, err := idx.SetProposalPolicy(core.ProposalPolicy{AutoAccept: true}); err != nil {
		t.Fatal(err)
	}
	// An architect session cannot accept its own proposal.
	own := callTool(t, architect, "update_node", map[string]any{"node_id": "svc", "spec": "# mine\n"})
	if msg := callErr(architect, own["proposal_id"].(string)); !strings.Contains(msg, "cannot accept") {
		t.Fatalf("self accept: %q", msg)
	}
	if idx.Nodes["svc"].Spec != "# svc\n" {
		t.Fatal("self-accept changed the model")
	}
	// The writer's proposal is accepted by the architect session.
	out := callTool(t, architect, "accept_proposal", map[string]any{"proposal_id": pid})
	if out["accepted"] != true {
		t.Fatalf("%v", out)
	}
	if idx.Nodes["svc"].Spec != "# svc v2\n" {
		t.Fatalf("spec %q", idx.Nodes["svc"].Spec)
	}
	entries, _ := idx.ReadAudit(core.AuditQuery{Actor: core.ActorArchitectAgent})
	if len(entries) != 1 || entries[0].ProposalID != pid {
		t.Fatalf("%+v", entries)
	}
}

func TestAcceptProposalListsBlockingOps(t *testing.T) {
	idx, dir := buildStageFixture(t)
	if _, err := idx.SetProposalPolicy(core.ProposalPolicy{AutoAccept: true}); err != nil {
		t.Fatal(err)
	}
	writer := sessionFor(t, idx, dir, "")
	architect := sessionFor(t, idx, dir, RoleArchitect)
	staged := callTool(t, writer, "delete_node", map[string]any{"node_id": "svc"})
	out := callTool(t, architect, "accept_proposal", map[string]any{"proposal_id": staged["proposal_id"]})
	blocking, _ := out["blocking"].([]any)
	if out["accepted"] != false || len(blocking) != 1 {
		t.Fatalf("%v", out)
	}
	first := blocking[0].(map[string]any)
	if first["rule"] != core.BlockDelete || first["reason"] == "" {
		t.Fatalf("%v", first)
	}
	if idx.Nodes["svc"] == nil {
		t.Fatal("blocked proposal was applied")
	}
}

func TestSetPartStagesProposalAndGraphPartScopes(t *testing.T) {
	idx, dir := openFixture(t)
	s := sessionFor(t, idx, dir, "")
	if !toolNames(t, s)["set_part"] {
		t.Fatal("set_part missing")
	}
	out := callTool(t, s, "set_part", map[string]any{"id": "billing", "name": "Billing", "include": []string{"src/billing/**"}, "root_node_ids": []string{"billing"}})
	pid := out["proposal_id"].(string)
	if out["applied"] != false || len(idx.Parts) != 0 {
		t.Fatalf("set_part must stage only: %v %+v", out, idx.Parts)
	}
	list, _ := idx.ListProposals()
	if len(list) != 1 || list[0].Operations[0].Op != core.OpSetPart {
		t.Fatalf("%+v", list)
	}
	if res, err := idx.AcceptAll(pid); err != nil || len(res.Applied) != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if len(idx.Parts) != 1 {
		t.Fatal("part not applied")
	}
	del := callTool(t, s, "set_part", map[string]any{"id": "billing", "delete": true})
	if _, err := idx.AcceptAll(del["proposal_id"].(string)); err != nil || len(idx.Parts) != 0 {
		t.Fatalf("%v %+v", err, idx.Parts)
	}
	// A bad part is rejected at staging.
	if res, err := s.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "set_part", Arguments: map[string]any{"id": "Bad Id"}}); err == nil && !res.IsError {
		t.Fatal("invalid id accepted")
	}

	// get_graph part scoping.
	a, _ := idx.Create(core.CreateInput{ID: "a", Name: "A", Type: "service"})
	if _, err := idx.Create(core.CreateInput{ID: "b", Name: "B", Type: "service"}); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.SetPart(core.Part{ID: "pa", RootNodeIDs: []core.NodeID{a}}); err != nil {
		t.Fatal(err)
	}
	g := callTool(t, s, "get_graph", map[string]any{"part": "pa"})
	nodes, _ := g["nodes"].([]any)
	if len(nodes) != 1 || g["part"] != "pa" {
		t.Fatalf("%v", g)
	}
	if res, err := s.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "get_graph", Arguments: map[string]any{"part": "nope"}}); err == nil && !res.IsError {
		t.Fatal("unknown part must fail")
	}
}

func TestAcceptMessageReportsPartialApply(t *testing.T) {
	none := acceptMessage(core.AutoAcceptResult{ProposalID: "p1", Blocking: make([]core.PolicyVerdict, 1)})
	if strings.Contains(none, "already applied") {
		t.Fatal(none)
	}
	some := acceptMessage(core.AutoAcceptResult{ProposalID: "p1", Blocking: make([]core.PolicyVerdict, 1), Applied: []int{0, 1}})
	if !strings.Contains(some, "2 operation(s) were already applied") {
		t.Fatal(some)
	}
}
