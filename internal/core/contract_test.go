package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mkNode(t *testing.T, idx *Index, id string, deps ...NodeID) {
	t.Helper()
	if _, err := idx.Create(CreateInput{ID: NodeID(id), Name: strings.ToUpper(id), Type: "service", DependsOn: deps}); err != nil {
		t.Fatalf("create %s: %v", id, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestNodeContractAndDependsOnRoundTrip(t *testing.T) {
	idx, dir := newModel(t)
	mkNode(t, idx, "db")
	mkNode(t, idx, "cache")
	c := &NodeContract{
		Payloads:   "POST /x {id}",
		Invariants: []string{"scoped by user_id"},
		Errors:     []string{"404 when missing"},
		OwnsData:   []string{"orders"},
	}
	if _, err := idx.Create(CreateInput{ID: "api", Name: "API", Type: "service", DependsOn: []NodeID{"db", "cache"}, Contract: c}); err != nil {
		t.Fatal(err)
	}
	got := reopen(t, dir).Nodes["api"]
	if got.Contract == nil || got.Contract.Payloads != c.Payloads || got.Contract.OwnsData[0] != "orders" || got.Contract.Errors[0] != "404 when missing" {
		t.Fatalf("contract %+v", got.Contract)
	}
	if len(got.DependsOn) != 2 || got.DependsOn[0] != "cache" || got.DependsOn[1] != "db" {
		t.Fatalf("depends_on not sorted: %v", got.DependsOn)
	}
	// Without the fields: nothing is written.
	raw := readFile(t, filepath.Join(dir, ".arch", "nodes", "db.json"))
	if strings.Contains(raw, "contract") || strings.Contains(raw, "depends_on") {
		t.Fatalf("empty fields written: %s", raw)
	}
	if n := reopen(t, dir).Nodes["db"]; n.Contract != nil || n.DependsOn != nil {
		t.Fatalf("db %+v", n)
	}
}

func TestNodeEncodeGoldenOrder(t *testing.T) {
	n := &Node{
		ID: "a", Name: "A", Type: "service", Status: StatusDraft, ParentID: "p",
		Schema:    []SchemaColumn{{Title: "id", Type: "int"}},
		DependsOn: []NodeID{"z", "b"},
		Contract:  &NodeContract{Payloads: "p", Invariants: []string{"i"}, Errors: []string{"e"}, OwnsData: []string{"o"}},
		Unknown:   map[string]jsonRaw{"zzz": jsonRaw(`1`)},
	}
	data, err := encodeNode(n)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n" +
		"    \"id\": \"a\",\n" +
		"    \"name\": \"A\",\n" +
		"    \"type\": \"service\",\n" +
		"    \"parent_id\": \"p\",\n" +
		"    \"schema\": [{\"title\":\"id\",\"type\":\"int\"}],\n" +
		"    \"depends_on\": [\"b\",\"z\"],\n" +
		"    \"contract\": {\"payloads\":\"p\",\"invariants\":[\"i\"],\"errors\":[\"e\"],\"owns_data\":[\"o\"]},\n" +
		"    \"status\": \"draft\",\n" +
		"    \"zzz\": 1\n" +
		"}\n"
	if string(data) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", data, want)
	}
	rel, err := encodeRelationship(Relationship{ID: "r", From: "a", To: "b", Label: "l", Kind: "sync",
		Contract: &EdgeContract{Format: "text", Ref: "docs/x.md", Text: "t"}})
	if err != nil {
		t.Fatal(err)
	}
	wantRel := `{"id":"r","from":"a","to":"b","label":"l","kind":"sync","contract":{"format":"text","ref":"docs/x.md","text":"t"}}`
	if string(rel) != wantRel {
		t.Fatalf("got %s", rel)
	}
}

func TestUnknownKeysPreservedBothDirections(t *testing.T) {
	// An unknown top-level key survives alongside the new fields.
	data := []byte(`{"id":"a","name":"A","type":"service","status":"draft","depends_on":["b"],"contract":{"payloads":"p"},"future":{"x":1}}`)
	n, err := decodeNode(data)
	if err != nil {
		t.Fatal(err)
	}
	if n.Contract == nil || n.DependsOn[0] != "b" || len(n.Unknown) != 1 {
		t.Fatalf("%+v", n)
	}
	out, err := encodeNode(n)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"future": {"x":1}`) || !strings.Contains(string(out), `"depends_on"`) {
		t.Fatalf("lost keys: %s", out)
	}
	// The new keys are not left in Unknown (so they are not written twice).
	if _, ok := n.Unknown["contract"]; ok {
		t.Fatal("contract left in Unknown")
	}
	// Relationship: unknown key kept next to contract.
	rel, err := decodeRelationship([]byte(`{"from":"a","to":"b","contract":{"format":"openapi","ref":"api.yaml"},"extra":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if rel.Contract == nil || rel.Contract.Format != "openapi" || len(rel.Unknown) != 1 {
		t.Fatalf("%+v", rel)
	}
	enc, err := encodeRelationship(rel)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(enc), `"extra":true`) || !strings.Contains(string(enc), `"contract"`) {
		t.Fatalf("lost: %s", enc)
	}
}

func TestDecodeWrongTypesIsHardErrorNamingFile(t *testing.T) {
	idx, dir := newModel(t)
	mkNode(t, idx, "a")
	path := filepath.Join(dir, ".arch", "nodes", "a.json")
	for name, body := range map[string]string{
		"depends_on string": `{"id":"a","name":"A","type":"service","status":"draft","depends_on":"b"}`,
		"contract string":   `{"id":"a","name":"A","type":"service","status":"draft","contract":"x"}`,
		"payloads number":   `{"id":"a","name":"A","type":"service","status":"draft","contract":{"payloads":3}}`,
		"errors object":     `{"id":"a","name":"A","type":"service","status":"draft","contract":{"errors":{}}}`,
	} {
		writeFile(t, path, body)
		_, err := Open(dir)
		if err == nil || !strings.Contains(err.Error(), "a.json") {
			t.Fatalf("%s: err %v", name, err)
		}
	}
	idx2, dir2 := newModel(t)
	mkNode(t, idx2, "a")
	mkNode(t, idx2, "b")
	if _, err := idx2.SetRelationship(SetRelationshipInput{From: "a", To: "b"}); err != nil {
		t.Fatal(err)
	}
	ip := filepath.Join(dir2, ".arch", "index.json")
	writeFile(t, ip, `{"schema_version":1,"nodes":["a","b"],"relationships":[{"id":"r","from":"a","to":"b","contract":{"ref":5}}]}`)
	if _, err := Open(dir2); err == nil || !strings.Contains(err.Error(), "index.json") {
		t.Fatalf("err %v", err)
	}
}

func TestContractLimitsRejectedOnWriteToleratedOnLoad(t *testing.T) {
	idx, dir := newModel(t)
	long := strings.Repeat("x", 301)
	items33 := make([]string, 33)
	for i := range items33 {
		items33[i] = "i"
	}
	bad := []*NodeContract{
		{Payloads: strings.Repeat("p", MaxContractPayloads+1)},
		{Invariants: items33},
		{Errors: []string{long}},
		{OwnsData: items33},
	}
	for i, c := range bad {
		_, err := idx.Create(CreateInput{ID: "n", Name: "N", Type: "service", Contract: c})
		if !errors.Is(err, ErrBadContract) {
			t.Fatalf("case %d: %v", i, err)
		}
	}
	// Exactly at the limits is accepted.
	ok := &NodeContract{Payloads: strings.Repeat("p", MaxContractPayloads), Invariants: items33[:32], Errors: []string{strings.Repeat("x", 300)}}
	mkNodeWithContract := func() {
		if _, err := idx.Create(CreateInput{ID: "n", Name: "N", Type: "service", Contract: ok}); err != nil {
			t.Fatal(err)
		}
	}
	mkNodeWithContract()
	// Over-limit text on disk loads; re-sending it unchanged is tolerated, changing it is not.
	overPayload := strings.Repeat("q", MaxContractPayloads+10)
	writeFile(t, filepath.Join(dir, ".arch", "nodes", "n.json"),
		`{"id":"n","name":"N","type":"service","status":"draft","contract":{"payloads":"`+overPayload+`"}}`)
	idx = reopen(t, dir)
	same := &NodeContract{Payloads: overPayload}
	if err := idx.Update("n", UpdateInput{Contract: same}); err != nil {
		t.Fatalf("unchanged over-limit: %v", err)
	}
	if err := idx.Update("n", UpdateInput{Contract: &NodeContract{Payloads: overPayload + "z"}}); !errors.Is(err, ErrBadContract) {
		t.Fatalf("got %v", err)
	}
	// Empty clears.
	if err := idx.Update("n", UpdateInput{Contract: &NodeContract{}}); err != nil {
		t.Fatal(err)
	}
	if reopen(t, dir).Nodes["n"].Contract != nil {
		t.Fatal("contract not cleared")
	}
	// Nil leaves it unchanged.
	if err := idx.Update("n", UpdateInput{Contract: &NodeContract{Payloads: "keep"}}); err != nil {
		t.Fatal(err)
	}
	name := "N2"
	if err := idx.Update("n", UpdateInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	if c := reopen(t, dir).Nodes["n"].Contract; c == nil || c.Payloads != "keep" {
		t.Fatalf("contract %+v", c)
	}
}

func TestEdgeContractValidationAndUpdate(t *testing.T) {
	idx, dir := newModel(t)
	mkNode(t, idx, "a")
	mkNode(t, idx, "b")
	cases := []*EdgeContract{
		{Format: "protobuf", Text: "x"},
		{Format: "text", Text: strings.Repeat("t", MaxEdgeContractText+1)},
		{Format: "openapi", Ref: strings.Repeat("r", MaxEdgeContractRef+1)},
		{Format: "openapi", Ref: "../secrets.yaml"},
		{Format: "openapi", Ref: "api/../../x.yaml"},
		{Format: "openapi", Ref: `api\..\x.yaml`},
	}
	for i, c := range cases {
		if _, err := idx.SetRelationship(SetRelationshipInput{From: "a", To: "b", Contract: c}); !errors.Is(err, ErrBadContract) {
			t.Fatalf("case %d: %v", i, err)
		}
	}
	rel, err := idx.SetRelationship(SetRelationshipInput{From: "a", To: "b", Label: "calls", Kind: "sync",
		Contract: &EdgeContract{Format: "openapi", Ref: "api/openapi.yaml", Text: "see spec"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := reopen(t, dir).Relationships[0].Contract; got == nil || got.Ref != "api/openapi.yaml" {
		t.Fatalf("%+v", got)
	}
	// nil contract on update leaves it; empty clears.
	if _, err := idx.SetRelationship(SetRelationshipInput{ID: rel.ID, From: "a", To: "b", Label: "calls2", Kind: "sync"}); err != nil {
		t.Fatal(err)
	}
	if got := reopen(t, dir).Relationships[0]; got.Contract == nil || got.Label != "calls2" {
		t.Fatalf("%+v", got)
	}
	if _, err := idx.SetRelationship(SetRelationshipInput{ID: rel.ID, From: "a", To: "b", Contract: &EdgeContract{}}); err != nil {
		t.Fatal(err)
	}
	if got := reopen(t, dir).Relationships[0]; got.Contract != nil {
		t.Fatalf("%+v", got)
	}
}

func TestDependsOnValidationOnMutation(t *testing.T) {
	idx, _ := newModel(t)
	mkNode(t, idx, "a")
	mkNode(t, idx, "b", "a")
	mkNode(t, idx, "c", "b")

	if _, err := idx.Create(CreateInput{ID: "x", Name: "X", Type: "service", DependsOn: []NodeID{"nope"}}); !errors.Is(err, ErrBadDependency) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := idx.Create(CreateInput{ID: "x", Name: "X", Type: "service", DependsOn: []NodeID{"a", "a"}}); !errors.Is(err, ErrBadDependency) {
		t.Fatalf("dup: %v", err)
	}
	if _, err := idx.Create(CreateInput{ID: "x", Name: "X", Type: "service", DependsOn: []NodeID{"x"}}); !errors.Is(err, ErrBadDependency) {
		t.Fatalf("self on create: %v", err)
	}
	set := func(id NodeID, deps ...NodeID) error {
		return idx.Update(id, UpdateInput{DependsOn: &deps})
	}
	if err := set("a", "a"); !errors.Is(err, ErrBadDependency) {
		t.Fatalf("self: %v", err)
	}
	if err := set("a", "b", "b"); !errors.Is(err, ErrBadDependency) {
		t.Fatalf("dup: %v", err)
	}
	if err := set("a", "ghost"); !errors.Is(err, ErrBadDependency) {
		t.Fatalf("missing: %v", err)
	}
	err := set("a", "c")
	if !errors.Is(err, ErrDependencyCycle) || !strings.Contains(err.Error(), "a -> c -> b -> a") {
		t.Fatalf("cycle: %v", err)
	}
	if err := set("a", "b"); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("2-cycle: %v", err)
	}
	if len(idx.Nodes["a"].DependsOn) != 0 {
		t.Fatalf("rejected write leaked: %v", idx.Nodes["a"].DependsOn)
	}
	// Clearing works.
	if err := set("c"); err != nil {
		t.Fatal(err)
	}
	if len(idx.Nodes["c"].DependsOn) != 0 {
		t.Fatal("not cleared")
	}
}

func TestDeleteGuardNamesDependents(t *testing.T) {
	idx, dir := newModel(t)
	mkNode(t, idx, "db")
	mkNode(t, idx, "api", "db")
	mkNode(t, idx, "worker", "db")
	err := idx.Delete("db")
	if !errors.Is(err, ErrHasDependents) || !strings.Contains(err.Error(), "API (api)") || !strings.Contains(err.Error(), "WORKER (worker)") {
		t.Fatalf("got %v", err)
	}
	if !idx.HasNode("db") {
		t.Fatal("db deleted")
	}
	empty := []NodeID{}
	for _, id := range []NodeID{"api", "worker"} {
		if err := idx.Update(id, UpdateInput{DependsOn: &empty}); err != nil {
			t.Fatal(err)
		}
	}
	if err := idx.Delete("db"); err != nil {
		t.Fatal(err)
	}
	if reopen(t, dir).HasNode("db") {
		t.Fatal("db still present")
	}
}

func TestLoadDependencyDiagnostics(t *testing.T) {
	idx, dir := newModel(t)
	mkNode(t, idx, "a")
	mkNode(t, idx, "b")
	mkNode(t, idx, "c")
	nodes := filepath.Join(dir, ".arch", "nodes")
	put := func(id, deps string) {
		writeFile(t, filepath.Join(nodes, id+".json"),
			`{"id":"`+id+`","name":"`+id+`","type":"service","status":"draft","depends_on":[`+deps+`]}`)
	}
	put("a", `"ghost"`)
	put("b", `"c"`)
	put("c", `"b"`)
	got := reopen(t, dir)
	var warn, fail []string
	for _, d := range got.Diagnostics {
		switch {
		case d.Severity == SeverityWarning && strings.Contains(d.Message, "depends_on"):
			warn = append(warn, d.Message)
		case d.Severity == SeverityError && strings.Contains(d.Message, "dependency cycle"):
			fail = append(fail, d.Message)
		}
	}
	if len(warn) != 1 || !strings.Contains(warn[0], "ghost") {
		t.Fatalf("warnings %v", warn)
	}
	if len(fail) != 1 || !strings.Contains(fail[0], "b, c") {
		t.Fatalf("errors %v", fail)
	}
	// Self reference on load is a cycle error, and the model still loads.
	put("a", `"a"`)
	put("b", ``)
	put("c", ``)
	got = reopen(t, dir)
	found := false
	for _, d := range got.Diagnostics {
		if d.Severity == SeverityError && strings.Contains(d.Message, "depends on itself") {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics %v", got.Diagnostics)
	}
}
