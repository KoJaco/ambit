package core

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContentHashLengthPrefix(t *testing.T) {
	jsonBytes := []byte("{\"id\":\"a\"}\n")
	mdBytes := []byte("# a\n")
	h := sha256.New()
	var lenb [8]byte
	binary.BigEndian.PutUint64(lenb[:], uint64(len(jsonBytes)))
	h.Write(lenb[:])
	h.Write(jsonBytes)
	binary.BigEndian.PutUint64(lenb[:], uint64(len(mdBytes)))
	h.Write(lenb[:])
	h.Write(mdBytes)
	want := "sha256:" + hex.EncodeToString(h.Sum(nil))
	if got := contentHash(jsonBytes, mdBytes); got != want {
		t.Fatalf("hash %s, want %s", got, want)
	}
}

func TestStageWritesProposalNotNodes(t *testing.T) {
	idx, dir := newModel(t)
	before := nodeFileNames(t, dir)
	id, err := idx.Stage(StageInput{
		Source:  OpCreateNode,
		Summary: "add orders",
		Ops: []StageOp{{
			Op:     OpCreateNode,
			Create: CreateInput{Name: "Payments Service", Type: "service", Spec: "# Payments\n"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !proposalIDPattern.MatchString(id) {
		t.Fatalf("id %q", id)
	}
	if names := nodeFileNames(t, dir); strings.Join(names, ",") != strings.Join(before, ",") {
		t.Fatalf("nodes changed: %v -> %v", before, names)
	}
	manifestPath := filepath.Join(dir, ".arch", ".proposals", id, "manifest.json")
	raw := mustRead(t, manifestPath)
	if !strings.Contains(string(raw), `"manifest_version": 1`) || !strings.Contains(string(raw), `"base_hash": null`) {
		t.Fatalf("manifest %s", raw)
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.ProposalID != id || m.Source != OpCreateNode || m.Summary != "add orders" || m.CreatedAt == "" {
		t.Fatalf("manifest %+v", m)
	}
	if len(m.Operations) != 1 || m.Operations[0].Op != OpCreateNode || m.Operations[0].NodeID != "payments-service" || m.Operations[0].Status != opStatusPending {
		t.Fatalf("op %+v", m.Operations[0])
	}
	propNode := filepath.Join(dir, ".arch", ".proposals", id, "nodes", "payments-service.md")
	if _, err := os.Stat(propNode); err != nil {
		t.Fatal(err)
	}
}

func TestStageRejectsDeleteWithChildrenCycleAndMissingEndpoint(t *testing.T) {
	idx, _ := newModel(t)
	platform, err := idx.Create(CreateInput{Name: "Platform", Type: "boundary", Spec: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Create(CreateInput{Name: "Orders", Type: "service", ParentID: platform, Spec: "o"}); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Create(CreateInput{Name: "Payments", Type: "service", ParentID: platform, Spec: "pay"}); err != nil {
		t.Fatal(err)
	}
	_, err = idx.Stage(StageInput{
		Source: OpDeleteNode,
		Ops:    []StageOp{{Op: OpDeleteNode, NodeID: platform}},
	})
	if !errors.Is(err, ErrHasChildren) {
		t.Fatalf("delete: %v", err)
	}
	if !strings.Contains(err.Error(), "2") || !strings.Contains(err.Error(), "Orders") || !strings.Contains(err.Error(), "Payments") {
		t.Fatalf("children error %v", err)
	}

	orders := NodeID("orders")
	_, err = idx.Stage(StageInput{
		Source: OpUpdateNode,
		Ops: []StageOp{{
			Op:     OpUpdateNode,
			NodeID: platform,
			Update: UpdateInput{Parent: &orders},
		}},
	})
	if !errors.Is(err, ErrCycle) {
		t.Fatalf("cycle: %v", err)
	}

	_, err = idx.Stage(StageInput{
		Source: OpSetRelationship,
		Ops:    []StageOp{{Op: OpSetRelationship, From: orders, To: "missing", Kind: "sync"}},
	})
	if !errors.Is(err, ErrMissingEndpoint) {
		t.Fatalf("endpoint: %v", err)
	}
}

func TestStageSeedModelParentsInsideTheProposal(t *testing.T) {
	idx, dir := newModel(t)
	id, err := idx.Stage(StageInput{
		Source:  SourceSeedModel,
		Summary: "platform and orders",
		Ops: []StageOp{
			{Op: OpCreateNode, Create: CreateInput{Name: "Platform", Type: "boundary", Spec: "p"}},
			{Op: OpCreateNode, Create: CreateInput{Name: "Orders", Type: "service", ParentID: "platform", Spec: "o"}},
			{Op: OpSetRelationship, From: "orders", To: "platform", Label: "runs in", Kind: "sync"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(nodeFileNames(t, dir)) != 0 {
		t.Fatal("seed wrote canonical nodes")
	}
	listed, err := idx.ListProposals()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Summary != "platform and orders" || len(listed[0].Operations) != 3 {
		t.Fatalf("list %+v", listed)
	}
	for _, op := range listed[0].Operations {
		if op.Stale {
			t.Fatalf("fresh op %+v", op)
		}
	}
	diff, err := idx.ProposalDiff(id)
	if err != nil {
		t.Fatal(err)
	}
	if diff.Operations[0].Current != nil || diff.Operations[0].Proposed == nil || diff.Operations[0].Proposed.Name != "Platform" {
		t.Fatalf("create diff %+v", diff.Operations[0])
	}
	if diff.Operations[2].Relationship == nil || diff.Operations[2].Relationship.To != "platform" {
		t.Fatalf("relationship diff %+v", diff.Operations[2])
	}
}

func TestUpdateRecordsDiskHashAndArchitectEditIsStale(t *testing.T) {
	idx, dir := newModel(t)
	id, err := idx.Create(CreateInput{Name: "Payments", Type: "service", Spec: "before"})
	if err != nil {
		t.Fatal(err)
	}
	spec := "staged"
	pid, err := idx.Stage(StageInput{
		Source: OpUpdateNode,
		Ops: []StageOp{{
			Op:     OpUpdateNode,
			NodeID: id,
			Update: UpdateInput{Spec: &spec},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := mustRead(t, filepath.Join(dir, ".arch", ".proposals", pid, "manifest.json"))
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	disk, ok, err := idx.hashOnDisk(id)
	if err != nil || !ok {
		t.Fatal(err, ok)
	}
	if m.Operations[0].BaseHash == nil || *m.Operations[0].BaseHash != disk {
		t.Fatalf("base %v disk %s", m.Operations[0].BaseHash, disk)
	}
	if len(m.Operations[0].Fields) != 1 || m.Operations[0].Fields[0] != "spec" {
		t.Fatalf("fields %v", m.Operations[0].Fields)
	}
	listed, err := idx.ListProposals()
	if err != nil || listed[0].Operations[0].Stale {
		t.Fatalf("fresh list %+v %v", listed, err)
	}
	edited := "architect"
	if err := idx.Update(id, UpdateInput{Spec: &edited}); err != nil {
		t.Fatal(err)
	}
	listed, err = idx.ListProposals()
	if err != nil {
		t.Fatal(err)
	}
	if !listed[0].Operations[0].Stale || listed[0].Operations[0].StaleReason != staleMismatch {
		t.Fatalf("stale %+v", listed[0].Operations[0])
	}
}

func TestVersionMismatchIsUnreadableAndMissingMarkdownIsNotApplied(t *testing.T) {
	idx, dir := newModel(t)
	pid, err := idx.Stage(StageInput{
		Source: OpCreateNode,
		Ops:    []StageOp{{Op: OpCreateNode, Create: CreateInput{Name: "Orders", Type: "service", Spec: "o"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".arch", ".proposals", pid, "manifest.json")
	obj := readObj(t, path)
	obj["manifest_version"] = 2
	writeObj(t, path, obj)
	listed, err := idx.ListProposals()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || !listed[0].Unreadable || !strings.Contains(listed[0].Error, "manifest_version") {
		t.Fatalf("list %+v", listed)
	}
	if _, err := idx.ProposalDiff(pid); !errors.Is(err, ErrUnreadableProposal) {
		t.Fatalf("diff %v", err)
	}
	if err := idx.AcceptOperation(pid, 0, false); !errors.Is(err, ErrUnreadableProposal) {
		t.Fatalf("accept %v", err)
	}
	if len(nodeFileNames(t, dir)) != 0 {
		t.Fatal("unreadable proposal was applied")
	}
	obj["manifest_version"] = 1
	writeObj(t, path, obj)
	if err := os.Remove(filepath.Join(dir, ".arch", ".proposals", pid, "nodes", "orders.md")); err != nil {
		t.Fatal(err)
	}
	diff, err := idx.ProposalDiff(pid)
	if err != nil {
		t.Fatal(err)
	}
	if diff.Operations[0].Proposed != nil || !strings.Contains(diff.Operations[0].Error, ".md") {
		t.Fatalf("diff %+v", diff.Operations[0])
	}
	if err := idx.AcceptOperation(pid, 0, false); !errors.Is(err, ErrUnreadableProposal) {
		t.Fatalf("missing md %v", err)
	}
	if len(nodeFileNames(t, dir)) != 0 {
		t.Fatal("missing markdown was applied")
	}
	if err := idx.DeleteProposal(pid); err != nil {
		t.Fatal(err)
	}
	listed, err = idx.ListProposals()
	if err != nil || len(listed) != 0 {
		t.Fatalf("after delete %+v %v", listed, err)
	}
}

func TestAcceptCreateIsAtomic(t *testing.T) {
	for _, step := range []string{"node-json", "node-md", "index"} {
		t.Run(step, func(t *testing.T) {
			idx, dir := newModel(t)
			t.Cleanup(func() { applyHook = nil })
			pid := stageCreate(t, idx, "Orders", "spec")
			beforeNodes := nodeFileNames(t, dir)
			beforeIndex := mustRead(t, filepath.Join(dir, ".arch", "index.json"))
			applyHook = func(got string) error {
				if got == step {
					return errors.New("injected failure after " + step)
				}
				return nil
			}
			err := idx.AcceptOperation(pid, 0, false)
			if err == nil || !strings.Contains(err.Error(), "injected") {
				t.Fatalf("err %v", err)
			}
			if names := nodeFileNames(t, dir); strings.Join(names, ",") != strings.Join(beforeNodes, ",") {
				t.Fatalf("nodes %v", names)
			}
			if got := mustRead(t, filepath.Join(dir, ".arch", "index.json")); string(got) != string(beforeIndex) {
				t.Fatalf("index changed:\n%s", got)
			}
			listed, err := idx.ListProposals()
			if err != nil || listed[0].Operations[0].Status != opStatusPending {
				t.Fatalf("status %+v %v", listed, err)
			}
		})
	}
}

func TestAcceptUpdateReplacesBothFilesOrNeither(t *testing.T) {
	idx, dir := newModel(t)
	t.Cleanup(func() { applyHook = nil })
	id, err := idx.Create(CreateInput{Name: "Orders", Type: "service", Spec: "original"})
	if err != nil {
		t.Fatal(err)
	}
	jsonBefore := mustRead(t, filepath.Join(dir, ".arch", "nodes", string(id)+".json"))
	mdBefore := mustRead(t, filepath.Join(dir, ".arch", "nodes", string(id)+".md"))
	spec := "staged"
	pid, err := idx.Stage(StageInput{
		Source: OpUpdateNode,
		Ops:    []StageOp{{Op: OpUpdateNode, NodeID: id, Update: UpdateInput{Spec: &spec}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	applyHook = func(step string) error {
		if step == "node-json" {
			return errors.New("injected")
		}
		return nil
	}
	if err := idx.AcceptOperation(pid, 0, false); err == nil {
		t.Fatal("expected injected failure")
	}
	if string(mustRead(t, filepath.Join(dir, ".arch", "nodes", string(id)+".json"))) != string(jsonBefore) {
		t.Fatal("json changed")
	}
	if string(mustRead(t, filepath.Join(dir, ".arch", "nodes", string(id)+".md"))) != string(mdBefore) {
		t.Fatal("markdown changed")
	}
	applyHook = nil
	if err := idx.AcceptOperation(pid, 0, false); err != nil {
		t.Fatal(err)
	}
	if got := string(mustRead(t, filepath.Join(dir, ".arch", "nodes", string(id)+".md"))); got != spec {
		t.Fatalf("md %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, ".arch", ".proposals", pid)); !os.IsNotExist(err) {
		t.Fatal("resolved proposal directory remains")
	}
}

func TestStaleAcceptRequiresConfirmationAndCycleStaysPending(t *testing.T) {
	idx, dir := newModel(t)
	a, err := idx.Create(CreateInput{Name: "A", Type: "service", Spec: "a"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := idx.Create(CreateInput{Name: "B", Type: "service", Spec: "b"})
	if err != nil {
		t.Fatal(err)
	}
	pid, err := idx.Stage(StageInput{
		Source: OpUpdateNode,
		Ops:    []StageOp{{Op: OpUpdateNode, NodeID: a, Update: UpdateInput{Parent: &b}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	edited := "architect touched a"
	if err := idx.Update(a, UpdateInput{Spec: &edited}); err != nil {
		t.Fatal(err)
	}
	if err := idx.Update(b, UpdateInput{Parent: &a}); err != nil {
		t.Fatal(err)
	}
	jsonBefore := mustRead(t, filepath.Join(dir, ".arch", "nodes", string(a)+".json"))
	if err := idx.AcceptOperation(pid, 0, false); !errors.Is(err, ErrStale) || !strings.Contains(err.Error(), "confirm_stale") || !strings.Contains(err.Error(), string(a)) {
		t.Fatalf("stale accept %v", err)
	}
	if string(mustRead(t, filepath.Join(dir, ".arch", "nodes", string(a)+".json"))) != string(jsonBefore) {
		t.Fatal("stale accept wrote")
	}
	if err := idx.AcceptOperation(pid, 0, true); !errors.Is(err, ErrCycle) {
		t.Fatalf("confirm %v", err)
	}
	if string(mustRead(t, filepath.Join(dir, ".arch", "nodes", string(a)+".json"))) != string(jsonBefore) {
		t.Fatal("cycle accept wrote")
	}
	listed, err := idx.ListProposals()
	if err != nil || listed[0].Operations[0].Status != opStatusPending {
		t.Fatalf("status %+v %v", listed, err)
	}
}

func TestMissingNodeCannotBeApplied(t *testing.T) {
	idx, _ := newModel(t)
	id, err := idx.Create(CreateInput{Name: "Orders", Type: "service", Spec: "o"})
	if err != nil {
		t.Fatal(err)
	}
	spec := "next"
	pid, err := idx.Stage(StageInput{
		Source: OpUpdateNode,
		Ops:    []StageOp{{Op: OpUpdateNode, NodeID: id, Update: UpdateInput{Spec: &spec}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.Delete(id); err != nil {
		t.Fatal(err)
	}
	err = idx.AcceptOperation(pid, 0, true)
	if !errors.Is(err, ErrStale) || !strings.Contains(err.Error(), "missing") || !strings.Contains(err.Error(), string(id)) {
		t.Fatalf("missing %v", err)
	}
}

func TestTwoProposalsTheSecondBecomesStale(t *testing.T) {
	idx, _ := newModel(t)
	id, err := idx.Create(CreateInput{Name: "Payments", Type: "service", Spec: "v0"})
	if err != nil {
		t.Fatal(err)
	}
	firstSpec, secondSpec := "v1", "v2"
	first, err := idx.Stage(StageInput{
		Source: OpUpdateNode,
		Ops:    []StageOp{{Op: OpUpdateNode, NodeID: id, Update: UpdateInput{Spec: &firstSpec}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := idx.Stage(StageInput{
		Source: OpUpdateNode,
		Ops:    []StageOp{{Op: OpUpdateNode, NodeID: id, Update: UpdateInput{Spec: &secondSpec}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	listed, err := idx.ListProposals()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[0].Operations[0].Stale || listed[1].Operations[0].Stale {
		t.Fatalf("both should be fresh: %+v", listed)
	}
	if err := idx.AcceptOperation(first, 0, false); err != nil {
		t.Fatal(err)
	}
	listed, err = idx.ListProposals()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != second || !listed[0].Operations[0].Stale {
		t.Fatalf("second proposal %+v", listed)
	}
}

func TestAcceptAllSkipsStaleAndRejectKeepsAPendingSibling(t *testing.T) {
	idx, dir := newModel(t)
	payments, err := idx.Create(CreateInput{Name: "Payments", Type: "service", Spec: "pay"})
	if err != nil {
		t.Fatal(err)
	}
	spec := "staged pay"
	pid, err := idx.Stage(StageInput{
		Source:  SourceSeedModel,
		Summary: "orders plus a payments edit",
		Ops: []StageOp{
			{Op: OpCreateNode, Create: CreateInput{Name: "Orders", Type: "service", Spec: "orders"}},
			{Op: OpUpdateNode, NodeID: payments, Update: UpdateInput{Spec: &spec}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	edited := "architect pay"
	if err := idx.Update(payments, UpdateInput{Spec: &edited}); err != nil {
		t.Fatal(err)
	}
	result, err := idx.AcceptAll(pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Applied) != 1 || result.Applied[0] != 0 || len(result.Skipped) != 1 || result.Skipped[0].NodeID != string(payments) {
		t.Fatalf("result %+v", result)
	}
	if _, ok := reopen(t, dir).Nodes["orders"]; !ok {
		t.Fatal("orders was not created")
	}
	if got := reopen(t, dir).Nodes[payments].Spec; got != edited {
		t.Fatalf("payments spec %q", got)
	}
	listed, err := idx.ListProposals()
	if err != nil || len(listed) != 1 || listed[0].Operations[1].Status != opStatusPending {
		t.Fatalf("pending sibling %+v %v", listed, err)
	}
	if err := idx.RejectOperation(pid, 0); err == nil {
		t.Fatal("rejected an operation that was already accepted")
	}
	if err := idx.RejectOperation(pid, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".arch", ".proposals", pid)); !os.IsNotExist(err) {
		t.Fatal("directory remained after the last operation resolved")
	}
}

func TestPartialReviewSurvivesReload(t *testing.T) {
	idx, dir := newModel(t)
	pid, err := idx.Stage(StageInput{
		Source: SourceSeedModel,
		Ops: []StageOp{
			{Op: OpCreateNode, Create: CreateInput{Name: "Billing", Type: "service", Spec: "b"}},
			{Op: OpCreateNode, Create: CreateInput{Name: "Notify", Type: "service", Spec: "n"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.AcceptOperation(pid, 0, false); err != nil {
		t.Fatal(err)
	}
	again := reopen(t, dir)
	listed, err := again.ListProposals()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Operations[0].Status != opStatusAccepted || listed[0].Operations[1].Status != opStatusPending {
		t.Fatalf("reloaded %+v", listed)
	}
	if err := again.RejectAll(pid); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".arch", ".proposals", pid)); !os.IsNotExist(err) {
		t.Fatal("reject-all left the directory")
	}
}

func TestAcceptAllAppliesSeedModel(t *testing.T) {
	idx, dir := newModel(t)
	pid, err := idx.Stage(StageInput{
		Source:  SourceSeedModel,
		Summary: "platform and orders",
		Ops: []StageOp{
			{Op: OpCreateNode, Create: CreateInput{Name: "Platform", Type: "boundary", Spec: "p"}},
			{Op: OpCreateNode, Create: CreateInput{Name: "Orders", Type: "service", ParentID: "platform", Spec: "o"}},
			{Op: OpSetRelationship, From: "orders", To: "platform", Label: "runs in", Kind: "sync"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := idx.AcceptAll(pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Applied) != 3 || len(result.Skipped) != 0 {
		t.Fatalf("result %+v", result)
	}
	got := reopen(t, dir)
	if got.Nodes["orders"].ParentID != "platform" {
		t.Fatalf("parent %s", got.Nodes["orders"].ParentID)
	}
	if len(got.Relationships) != 1 || got.Relationships[0].From != "orders" || got.Relationships[0].Label != "runs in" {
		t.Fatalf("rels %+v", got.Relationships)
	}
	if _, err := os.Stat(filepath.Join(dir, ".arch", ".proposals", pid)); !os.IsNotExist(err) {
		t.Fatal("proposal directory remains")
	}
}

func TestAcceptUsesLayoutInvalidationRules(t *testing.T) {
	idx, _ := newModel(t)
	if err := idx.PutLayout(RootLayoutKey, Layout{Positions: []Position{{ID: "x", X: 1, Y: 2}}}); err != nil {
		t.Fatal(err)
	}
	pid := stageCreate(t, idx, "Orders", "spec")
	if err := idx.AcceptOperation(pid, 0, false); err != nil {
		t.Fatal(err)
	}
	got, err := idx.Layout(RootLayoutKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Positions) != 0 {
		t.Fatalf("create left layout %+v", got)
	}

	if err := idx.PutLayout(RootLayoutKey, Layout{Positions: []Position{{ID: "orders", X: 3, Y: 4}}}); err != nil {
		t.Fatal(err)
	}
	spec := "renamed prose"
	pid, err = idx.Stage(StageInput{
		Source: OpUpdateNode,
		Ops:    []StageOp{{Op: OpUpdateNode, NodeID: "orders", Update: UpdateInput{Spec: &spec}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.AcceptOperation(pid, 0, false); err != nil {
		t.Fatal(err)
	}
	got, err = idx.Layout(RootLayoutKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Positions) != 1 {
		t.Fatalf("spec edit cleared layout %+v", got)
	}
}

func TestStageThirtyOperations(t *testing.T) {
	idx, _ := newModel(t)
	ops := make([]StageOp, 30)
	for i := range ops {
		ops[i] = StageOp{Op: OpCreateNode, Create: CreateInput{
			Name: "Service " + strings.Repeat("x", i+1),
			Type: "service",
			Spec: "spec",
		}}
	}
	pid, err := idx.Stage(StageInput{Source: SourceSeedModel, Summary: "thirty services", Ops: ops})
	if err != nil {
		t.Fatal(err)
	}
	diff, err := idx.ProposalDiff(pid)
	if err != nil {
		t.Fatal(err)
	}
	if diff.Summary != "thirty services" || len(diff.Operations) != 30 {
		t.Fatalf("diff %s ops %d", diff.Summary, len(diff.Operations))
	}
}

func TestManualStage(t *testing.T) {
	dir := os.Getenv("AMBIT_STAGE_DIR")
	if dir == "" {
		t.Skip("set AMBIT_STAGE_DIR to stage into a scratch model")
	}
	idx, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	spec := "staged from the test helper"
	id, err := idx.Create(CreateInput{Name: "Scratch", Type: "service", Spec: "original"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Stage(StageInput{
		Source:  OpUpdateNode,
		Summary: "scratch review",
		Ops:     []StageOp{{Op: OpUpdateNode, NodeID: id, Update: UpdateInput{Spec: &spec}}},
	}); err != nil {
		t.Fatal(err)
	}
}

func stageCreate(t *testing.T, idx *Index, name, spec string) string {
	t.Helper()
	id, err := idx.Stage(StageInput{
		Source: OpCreateNode,
		Ops:    []StageOp{{Op: OpCreateNode, Create: CreateInput{Name: name, Type: "service", Spec: spec}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func nodeFileNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, ".arch", "nodes"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
