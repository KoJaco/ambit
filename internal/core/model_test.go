package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newModel(t *testing.T) (*Index, string) {
	t.Helper()
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	idx, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return idx, dir
}

func reopen(t *testing.T, dir string) *Index {
	t.Helper()
	idx, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

func TestRoundTripMinimalAndFull(t *testing.T) {
	idx, dir := newModel(t)
	platform, err := idx.Create(CreateInput{Name: "Platform", Type: "boundary", Spec: "# Platform\n"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := idx.Create(CreateInput{
		Name:           "Payments Service",
		Type:           "service",
		ParentID:       platform,
		Status:         StatusAssigned,
		Implementation: []string{"src/payments/**", "migrations/payments/**"},
		Scope:          []string{"src/payments/**", "migrations/payments/**", "src/shared/money.ts"},
		Spec:           "# Payments Service\n\nOwns capture.\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "payments-service" {
		t.Fatalf("id %q", id)
	}
	got := reopen(t, dir)
	n := got.Nodes[id]
	if n.Name != "Payments Service" || n.Type != "service" || n.Status != StatusAssigned || n.ParentID != platform {
		t.Fatalf("node %+v", n)
	}
	if n.Spec != "# Payments Service\n\nOwns capture.\n" {
		t.Fatalf("spec %q", n.Spec)
	}
	if len(n.Implementation) != 2 || len(n.Scope) != 3 {
		t.Fatalf("globs impl=%v scope=%v", n.Implementation, n.Scope)
	}
	if _, ok := got.Children[platform]; !ok || len(got.Children[platform]) != 1 {
		t.Fatalf("children %#v", got.Children)
	}
	min, err := idx.Create(CreateInput{Name: "Orders", Type: "service"})
	if err != nil {
		t.Fatal(err)
	}
	m := reopen(t, dir).Nodes[min]
	if m.Status != StatusDraft || m.ParentID != "" || m.Protected || len(m.Scope) != 0 {
		t.Fatalf("minimal %+v", m)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".arch", "nodes", string(min)+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"x"`) || strings.Contains(string(raw), "position") {
		t.Fatalf("canonical file gained coordinates: %s", raw)
	}
}

func TestUnknownFieldRoundTrip(t *testing.T) {
	idx, dir := newModel(t)
	id, err := idx.Create(CreateInput{Name: "Orders Service", Type: "service", Spec: "spec"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".arch", "nodes", string(id)+".json")
	obj := readObj(t, path)
	obj["observed_implementation"] = []any{"src/orders/**"}
	writeObj(t, path, obj)

	got := reopen(t, dir)
	raw, ok := got.Nodes[id].Unknown["observed_implementation"]
	if !ok || len(raw) == 0 {
		t.Fatal("unknown field dropped on load")
	}
	name := "Orders Renamed"
	if err := got.Update(id, UpdateInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	again := reopen(t, dir)
	if again.Nodes[id].Name != name || again.Nodes[id].ID != id {
		t.Fatalf("rename changed identity: %+v", again.Nodes[id])
	}
	if _, err := os.Stat(filepath.Join(dir, ".arch", "nodes", string(id)+".json")); err != nil {
		t.Fatal(err)
	}
	kept := readObj(t, path)
	obs, ok := kept["observed_implementation"].([]any)
	if !ok || len(obs) != 1 || obs[0] != "src/orders/**" {
		t.Fatalf("unknown field not preserved: %#v", kept["observed_implementation"])
	}

	indexPath := filepath.Join(dir, ".arch", "index.json")
	index := readObj(t, indexPath)
	index["note"] = "keep"
	writeObj(t, indexPath, index)
	loaded := reopen(t, dir)
	if _, err := loaded.Create(CreateInput{Name: "Extra", Type: "service"}); err != nil {
		t.Fatal(err)
	}
	if readObj(t, indexPath)["note"] != "keep" {
		t.Fatal("index unknown field dropped")
	}

	configPath := filepath.Join(dir, ".arch", "config.json")
	writeObj(t, configPath, map[string]any{"ui": "dark"})
	withConfig := reopen(t, dir)
	ui, ok := withConfig.configUnknown["ui"]
	if !ok || string(ui) != `"dark"` {
		t.Fatalf("config unknown %#v", withConfig.configUnknown)
	}
	encoded, err := encodeObject(nil, nil, withConfig.configUnknown)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	if string(reopen(t, dir).configUnknown["ui"]) != `"dark"` {
		t.Fatal("config unknown did not round-trip")
	}
}

func TestSlugCollisions(t *testing.T) {
	idx, _ := newModel(t)
	var ids []NodeID
	for i := 0; i < 3; i++ {
		id, err := idx.Create(CreateInput{Name: "Orders Service", Type: "service"})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if ids[0] != "orders-service" || ids[1] != "orders-service-2" || ids[2] != "orders-service-3" {
		t.Fatalf("ids %v", ids)
	}
	if _, err := idx.Create(CreateInput{Name: "!!!", Type: "service"}); err == nil {
		t.Fatal("expected empty slug to fail")
	}
	id, err := idx.Create(CreateInput{Name: "Data Store", Type: "data store"})
	if err != nil {
		t.Fatal(err)
	}
	if id != "data-store" || idx.Nodes[id].Type != "data store" {
		t.Fatalf("free-form type: %+v", idx.Nodes[id])
	}
}

func TestEmptyScopeIsNotRewritten(t *testing.T) {
	idx, dir := newModel(t)
	id, err := idx.Create(CreateInput{
		Name:           "Payments",
		Type:           "service",
		Implementation: []string{"src/payments/**"},
	})
	if err != nil {
		t.Fatal(err)
	}
	empty := []string{}
	if err := idx.Update(id, UpdateInput{Scope: &empty}); err != nil {
		t.Fatal(err)
	}
	n := idx.Nodes[id]
	if len(n.Scope) != 0 {
		t.Fatalf("scope stored as %#v", n.Scope)
	}
	if strings.Join(EffectiveScope(n), ",") != "src/payments/**" {
		t.Fatalf("effective %#v", EffectiveScope(n))
	}
	raw := string(mustRead(t, filepath.Join(dir, ".arch", "nodes", string(id)+".json")))
	if strings.Contains(raw, `"scope"`) {
		t.Fatalf("empty scope was written: %s", raw)
	}
	obj := readObj(t, filepath.Join(dir, ".arch", "nodes", string(id)+".json"))
	if _, ok := obj["protected"]; ok {
		t.Fatal("default protected was written")
	}
	prot := true
	if err := idx.Update(id, UpdateInput{Protected: &prot}); err != nil {
		t.Fatal(err)
	}
	if readObj(t, filepath.Join(dir, ".arch", "nodes", string(id)+".json"))["protected"] != true {
		t.Fatal("protected true missing")
	}
}

func TestMutations(t *testing.T) {
	idx, dir := newModel(t)
	a, err := idx.Create(CreateInput{Name: "A", Type: "service"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := idx.Create(CreateInput{Name: "B", Type: "service", ParentID: a})
	if err != nil {
		t.Fatal(err)
	}
	c, err := idx.Create(CreateInput{Name: "C", Type: "service", ParentID: b})
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.Update(a, UpdateInput{Parent: &c}); err == nil {
		t.Fatal("expected cycle rejection")
	}
	if idx.Nodes[a].ParentID != "" {
		t.Fatal("cycle update was stored")
	}
	self := a
	if err := idx.Update(a, UpdateInput{Parent: &self}); err == nil {
		t.Fatal("expected self-parent rejection")
	}
	missing := NodeID("nope")
	if err := idx.Update(b, UpdateInput{Parent: &missing}); err == nil {
		t.Fatal("expected missing parent rejection")
	}
	if err := idx.Delete(a); err == nil || !strings.Contains(err.Error(), "b") {
		t.Fatalf("delete parent: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".arch", "nodes", "a.json")); err != nil {
		t.Fatal("delete removed the file anyway")
	}

	if err := idx.SetRelationship(b, c, "calls", KindSync); err != nil {
		t.Fatal(err)
	}
	if err := idx.SetRelationship(b, c, "reads", KindData); err != nil {
		t.Fatal(err)
	}
	if err := idx.SetRelationship(c, b, "replies", KindAsync); err != nil {
		t.Fatal(err)
	}
	if len(idx.ActiveEdges) != 2 {
		t.Fatalf("edges %#v", idx.ActiveEdges)
	}
	var forward Relationship
	for _, rel := range idx.ActiveEdges {
		if rel.From == b && rel.To == c {
			forward = rel
		}
	}
	if forward.Label != "reads" || forward.Kind != KindData {
		t.Fatalf("updated edge %+v", forward)
	}
	if err := idx.SetRelationship(b, "missing", "x", ""); err == nil {
		t.Fatal("expected missing endpoint")
	}
	if err := idx.SetRelationship(b, c, "nope", "pubsub"); err == nil {
		t.Fatal("expected bad kind")
	}

	if err := idx.Delete(c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".arch", "nodes", "c.json")); !os.IsNotExist(err) {
		t.Fatal("node files remain")
	}
	if _, err := os.Stat(filepath.Join(dir, ".arch", "nodes", "c.md")); !os.IsNotExist(err) {
		t.Fatal("markdown remains")
	}
	got := reopen(t, dir)
	for _, id := range got.Order {
		if id == c {
			t.Fatal("membership still lists c")
		}
	}
	if len(got.Relationships) != 0 {
		t.Fatalf("relationships survived delete: %#v", got.Relationships)
	}
}

func TestLoadIntegrity(t *testing.T) {
	idx, dir := newModel(t)
	if _, err := idx.Create(CreateInput{Name: "Platform", Type: "boundary", Spec: "p"}); err != nil {
		t.Fatal(err)
	}
	child, err := idx.Create(CreateInput{Name: "Child", Type: "service", ParentID: "platform", Spec: "c"})
	if err != nil {
		t.Fatal(err)
	}

	orphanPath := filepath.Join(dir, ".arch", "nodes", "rogue.json")
	if err := os.WriteFile(orphanPath, []byte("{\n    \"id\": \"rogue\",\n    \"name\": \"Rogue\",\n    \"type\": \"service\",\n    \"status\": \"draft\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mdOrphan := filepath.Join(dir, ".arch", "nodes", "lonely.md")
	if err := os.WriteFile(mdOrphan, []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := reopen(t, dir)
	if _, ok := got.Nodes["rogue"]; ok {
		t.Fatal("orphan was adopted")
	}
	if !hasDiag(got, SeverityWarning, "rogue.json") || !hasDiag(got, SeverityWarning, "lonely.md") {
		t.Fatalf("diagnostics %#v", got.Diagnostics)
	}
	if _, err := os.Stat(orphanPath); err != nil {
		t.Fatal("orphan was deleted")
	}
	before, err := os.ReadFile(filepath.Join(dir, ".arch", "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	_ = got
	after, err := os.ReadFile(filepath.Join(dir, ".arch", "index.json"))
	if err != nil || string(before) != string(after) {
		t.Fatal("open rewrote index.json")
	}

	nodePath := filepath.Join(dir, ".arch", "nodes", string(child)+".json")
	obj := readObj(t, nodePath)
	obj["parent_id"] = "platfrom"
	writeObj(t, nodePath, obj)
	dangling := reopen(t, dir)
	if !hasDiag(dangling, SeverityWarning, "platfrom") {
		t.Fatalf("diagnostics %#v", dangling.Diagnostics)
	}
	if !containsID(dangling.Roots, child) {
		t.Fatalf("dangling node not at root: %#v", dangling.Roots)
	}
	if len(dangling.Children["platform"]) != 0 {
		t.Fatal("dangling edge was walked")
	}

	// Hand-edited cycle. Files must be left as written.
	aPath := filepath.Join(dir, ".arch", "nodes", "platform.json")
	aObj := readObj(t, aPath)
	aObj["parent_id"] = "child"
	writeObj(t, aPath, aObj)
	cObj := readObj(t, nodePath)
	cObj["parent_id"] = "platform"
	writeObj(t, nodePath, cObj)
	snapA := mustRead(t, aPath)
	snapC := mustRead(t, nodePath)
	cycled := reopen(t, dir)
	if !hasDiag(cycled, SeverityError, "cycle") {
		t.Fatalf("diagnostics %#v", cycled.Diagnostics)
	}
	if len(cycled.Children) != 0 {
		t.Fatalf("cycle was walked: %#v", cycled.Children)
	}
	if string(mustRead(t, aPath)) != string(snapA) || string(mustRead(t, nodePath)) != string(snapC) {
		t.Fatal("open rewrote a cycle")
	}

	bad := readObj(t, aPath)
	bad["status"] = "nope"
	writeObj(t, aPath, bad)
	if _, err := Open(dir); err == nil || !strings.Contains(err.Error(), "platform.json") {
		t.Fatalf("bad status: %v", err)
	}
	bad["status"] = "draft"
	bad["id"] = "other"
	writeObj(t, aPath, bad)
	if _, err := Open(dir); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("id mismatch: %v", err)
	}
	if err := os.WriteFile(aPath, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil || !strings.Contains(err.Error(), "platform.json") {
		t.Fatalf("malformed: %v", err)
	}
}

func TestAssignmentRoundTrip(t *testing.T) {
	idx, dir := newModel(t)
	id, err := idx.Create(CreateInput{Name: "Payments", Type: "service"})
	if err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	if err := idx.SetAssignment(id, when); err != nil {
		t.Fatal(err)
	}
	got := reopen(t, dir)
	if got.Assignment == nil || got.Assignment.NodeID != id || got.Assignment.AssignedAt != "2026-09-27T10:00:00Z" {
		t.Fatalf("assignment %+v", got.Assignment)
	}
	path := filepath.Join(dir, ".arch", "local.json")
	obj := readObj(t, path)
	as := obj["assignment"].(map[string]any)
	as["source"] = "test"
	obj["ui"] = "compact"
	writeObj(t, path, obj)
	withExtra := reopen(t, dir)
	if string(withExtra.localUnknown["ui"]) != `"compact"` {
		t.Fatalf("local unknown %#v", withExtra.localUnknown)
	}
	if err := withExtra.SetAssignment(id, when); err != nil {
		t.Fatal(err)
	}
	kept := readObj(t, path)
	if kept["ui"] != "compact" {
		t.Fatal("top-level local key dropped")
	}
	as = kept["assignment"].(map[string]any)
	if as["source"] != "test" {
		t.Fatalf("assignment unknown dropped: %#v", as)
	}
	if err := withExtra.ClearAssignment(); err != nil {
		t.Fatal(err)
	}
	cleared := reopen(t, dir)
	if cleared.Assignment != nil {
		t.Fatal("assignment remains")
	}
	if string(cleared.localUnknown["ui"]) != `"compact"` {
		t.Fatal("clear removed unrelated local state")
	}
}

func TestInitIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n.arch/.cache/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	text := string(mustRead(t, filepath.Join(dir, ".gitignore")))
	if strings.Count(text, ".arch/local.json") != 1 || strings.Count(text, ".arch/.cache/") != 1 || strings.Count(text, ".arch/.proposals/") != 1 {
		t.Fatalf("gitignore:\n%s", text)
	}
	if !strings.Contains(text, "*.log") {
		t.Fatal("existing ignore line dropped")
	}
	idx := reopen(t, dir)
	if _, err := idx.Create(CreateInput{Name: "Keep", Type: "service", Spec: "s"}); err != nil {
		t.Fatal(err)
	}
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if _, ok := reopen(t, dir).Nodes["keep"]; !ok {
		t.Fatal("second init wiped the model")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "hooks", "pre-commit")); !os.IsNotExist(err) {
		t.Fatal("init installed a hook")
	}
}

func TestRejectsNodeIDThatEscapesNodesDir(t *testing.T) {
	_, dir := newModel(t)
	path := filepath.Join(dir, ".arch", "index.json")
	obj := readObj(t, path)
	obj["nodes"] = []any{"../outside"}
	writeObj(t, path, obj)
	_, err := Open(dir)
	if err == nil || !strings.Contains(err.Error(), "not a valid slug") {
		t.Fatalf("Open err = %v", err)
	}
}

func TestReloadSameLoader(t *testing.T) {
	idx, dir := newModel(t)
	id, err := idx.Create(CreateInput{Name: "Orders", Type: "service", Spec: "s"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".arch", "nodes", string(id)+".json")
	obj := readObj(t, path)
	obj["name"] = "Orders Two"
	writeObj(t, path, obj)
	if err := idx.reload(); err != nil {
		t.Fatal(err)
	}
	if idx.Nodes[id].Name != "Orders Two" {
		t.Fatalf("reload name %q", idx.Nodes[id].Name)
	}
}

func readObj(t *testing.T, path string) map[string]any {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(mustRead(t, path), &obj); err != nil {
		t.Fatal(err)
	}
	return obj
}

func writeObj(t *testing.T, path string, obj map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func hasDiag(idx *Index, sev Severity, fragment string) bool {
	for _, d := range idx.Diagnostics {
		if d.Severity == sev && strings.Contains(d.Message, fragment) {
			return true
		}
	}
	return false
}

func containsID(ids []NodeID, id NodeID) bool {
	for _, existing := range ids {
		if existing == id {
			return true
		}
	}
	return false
}
