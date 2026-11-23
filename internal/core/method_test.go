package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeMethod(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, ".arch", "methods", name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuiltinMethodsLoad(t *testing.T) {
	for _, id := range builtinOrder {
		m, ok := BuiltinMethod(id)
		if !ok {
			t.Fatalf("built-in %s missing (%v)", id, builtinErr)
		}
		if m.Guidance == "" || strings.Contains(m.Guidance, "{{") {
			t.Fatalf("%s guidance not assembled: %q", id, m.Guidance)
		}
		for rule := range m.Lint {
			if !isLintRule(rule) {
				t.Fatalf("%s names unknown rule %s", id, rule)
			}
		}
	}
	df, _ := BuiltinMethod("design-first")
	if !strings.Contains(df.Guidance, ShallowModelGuidance) || !strings.Contains(df.Guidance, AuthoringGuidance) {
		t.Fatal("design-first guidance must embed the shared constants")
	}
	if df.InitialStage != "design" || len(df.Stages) != 2 || len(df.Gates.Status["design"]) != 3 {
		t.Fatalf("design-first: %+v", df)
	}
	me, _ := BuiltinMethod("map-existing")
	if !me.Check.IncludeDrift || len(me.Stages) != 0 {
		t.Fatalf("map-existing: %+v", me)
	}
	ff, _ := BuiltinMethod("freeform")
	if len(ff.Lint) != 0 || ff.BriefPreamble != "" || len(ff.Stages) != 0 || strings.Count(ff.Guidance, "\n\n") != 0 {
		t.Fatalf("freeform: %+v", ff)
	}
}

func TestDefaultMethodIsFreeform(t *testing.T) {
	idx, _ := newModel(t)
	if idx.ActiveMethod().ID != "freeform" || idx.EffectiveStage() != "" || len(idx.Diagnostics) != 0 {
		t.Fatalf("%v %v", idx.ActiveMethod().ID, idx.Diagnostics)
	}
	list := idx.ListMethods()
	if len(list) != 3 || !list[0].Active {
		t.Fatalf("%+v", list)
	}
	if _, err := idx.MethodByID("nope"); !errors.Is(err, ErrUnknownMethod) {
		t.Fatalf("%v", err)
	}
}

func TestUserMethodExtendsAndFiles(t *testing.T) {
	idx, dir := newModel(t)
	writeMethod(t, dir, "team.json", `{
		"schema_version": 1, "id": "team", "name": "Team", "extends": "design-first",
		"guidance": {"file": "team.md"},
		"lint": {"icon-set": "enforced", "max-depth": {"severity": "enforced", "params": {"max_depth": 1}}, "bogus-rule": "off"}
	}`)
	writeMethod(t, dir, "team.md", "Team guidance.\n")
	idx = reopen(t, dir)
	if _, err := idx.SetMethod("team"); err != nil {
		t.Fatal(err)
	}
	m := idx.ActiveMethod()
	if m.ID != "team" || m.Guidance != "Team guidance." || m.Source != "user" {
		t.Fatalf("%+v", m)
	}
	if len(m.Stages) != 2 || m.InitialStage != "design" || len(m.Gates.Status["design"]) != 3 {
		t.Fatalf("inherited stages/gates missing: %+v", m)
	}
	if m.Lint["spec-present"].Severity != "enforced" || m.Lint["icon-set"].Severity != "enforced" || m.Lint["max-depth"].Params["max_depth"] != float64(1) {
		t.Fatalf("lint merge: %+v", m.Lint)
	}
	if !hasDiag(idx, SeverityWarning, `lint rule "bogus-rule" is unknown`) {
		t.Fatalf("no unknown-rule warning: %v", idx.Diagnostics)
	}
	// Round trip through JSON keeps compact severities.
	b, _ := json.Marshal(m.Lint["icon-set"])
	if string(b) != `"enforced"` {
		t.Fatal(string(b))
	}
}

func TestBadUserMethodFallsBackToFreeform(t *testing.T) {
	cases := map[string]string{
		"mismatch.json": `{"schema_version":1,"id":"other"}`,
		"badver.json":   `{"schema_version":2,"id":"badver"}`,
		"broken.json":   `{`,
		"toolong.json":  `{"schema_version":1,"id":"toolong","guidance":"` + strings.Repeat("x", 6001) + `"}`,
		"badinit.json":  `{"schema_version":1,"id":"badinit","stages":[{"id":"a"}],"initial_stage":"z"}`,
		"badgate.json":  `{"schema_version":1,"id":"badgate","stages":[{"id":"a"}],"gates":{"status":{"a":["nope"]}}}`,
		"badlint.json":  `{"schema_version":1,"id":"badlint","lint":{"icon-set":"loud"}}`,
		"badext.json":   `{"schema_version":1,"id":"badext","extends":"someone-else"}`,
		"badfile.json":  `{"schema_version":1,"id":"badfile","guidance":{"file":"../x.md"}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			idx, dir := newModel(t)
			writeMethod(t, dir, name, body)
			id := strings.TrimSuffix(name, ".json")
			if err := idx.SetConfigKeys(map[string]any{"method": id}); err != nil {
				t.Fatal(err)
			}
			if idx.ActiveMethod().ID != "freeform" {
				t.Fatalf("active %s", idx.ActiveMethod().ID)
			}
			if !hasDiag(idx, SeverityWarning, "method skipped") || !hasDiag(idx, SeverityWarning, "not defined") {
				t.Fatalf("diagnostics: %v", idx.Diagnostics)
			}
			if _, err := idx.MethodByID(id); !errors.Is(err, ErrUnknownMethod) {
				t.Fatalf("%v", err)
			}
		})
	}
}

func TestParseMethodErrorsAreErrBadMethod(t *testing.T) {
	_, _, err := parseMethod([]byte(`{"schema_version":1,"id":"x","lint":{"icon-set":"loud"}}`), "x", nil, nil, "user")
	if !errors.Is(err, ErrBadMethod) || !errors.Is(err, ErrBadLintRule) {
		t.Fatalf("%v", err)
	}
	_, _, err = parseMethod([]byte(`{"schema_version":1,"id":"x","stages":[{"id":"a"},{"id":"a"}]}`), "x", nil, nil, "user")
	if !errors.Is(err, ErrBadMethod) {
		t.Fatalf("%v", err)
	}
	var stages []string
	for i := 0; i < 9; i++ {
		stages = append(stages, `{"id":"s`+string(rune('a'+i))+`"}`)
	}
	_, _, err = parseMethod([]byte(`{"schema_version":1,"id":"x","stages":[`+strings.Join(stages, ",")+`]}`), "x", nil, nil, "user")
	if !errors.Is(err, ErrBadMethod) {
		t.Fatalf("9 stages: %v", err)
	}
	long := strings.Repeat("y", 1501)
	_, _, err = parseMethod([]byte(`{"schema_version":1,"id":"x","stages":[{"id":"a","preamble":"`+long+`"}]}`), "x", nil, nil, "user")
	if !errors.Is(err, ErrBadMethod) {
		t.Fatalf("long preamble: %v", err)
	}
}

func TestUserMethodShadowsBuiltin(t *testing.T) {
	idx, dir := newModel(t)
	writeMethod(t, dir, "design-first.json", `{"schema_version":1,"id":"design-first","extends":"design-first","guidance":"Mine."}`)
	if err := idx.SetConfigKeys(map[string]any{"method": "design-first"}); err != nil {
		t.Fatal(err)
	}
	m := idx.ActiveMethod()
	if m.Source != "user" || !m.Shadows || m.Guidance != "Mine." || len(m.Stages) != 2 {
		t.Fatalf("%+v", m)
	}
	if !hasDiag(idx, SeverityWarning, "shadows the built-in") {
		t.Fatalf("%v", idx.Diagnostics)
	}
	if row := idx.ListMethods()[1]; row.ID != "design-first" || row.Source != "user" {
		t.Fatalf("%+v", row)
	}
}

func TestUnknownMethodAndBadConfigValuesWarnNeverFail(t *testing.T) {
	idx, dir := newModel(t)
	if err := os.WriteFile(filepath.Join(dir, ".arch", "config.json"), []byte(`{"method": 5, "stage": ["x"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	idx = reopen(t, dir)
	if idx.ActiveMethod().ID != "freeform" || !hasDiag(idx, SeverityWarning, "method must be a string") || !hasDiag(idx, SeverityWarning, "stage must be a string") {
		t.Fatalf("%v", idx.Diagnostics)
	}
	if err := os.WriteFile(filepath.Join(dir, ".arch", "config.json"), []byte(`{"method": "ghost", "stage": "x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	idx = reopen(t, dir)
	if idx.ActiveMethod().ID != "freeform" || !hasDiag(idx, SeverityWarning, `method "ghost" is not defined`) {
		t.Fatalf("%v", idx.Diagnostics)
	}
}

func readConfigObj(t *testing.T, dir string) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".arch", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSetConfigKeysPreservesUnknownAndSorts(t *testing.T) {
	idx, dir := newModel(t)
	if err := os.WriteFile(filepath.Join(dir, ".arch", "config.json"), []byte(`{"zeta": {"a": [1,2]}, "alpha": true, "group_max": 5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	idx = reopen(t, dir)
	if err := idx.SetConfigKeys(map[string]any{"method": "design-first", "note": "List<Order> & more"}); err != nil {
		t.Fatal(err)
	}
	if err := idx.SetConstraints([]Constraint{{ID: "r", Text: "a<b", Severity: "must"}}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".arch", "config.json"))
	s := string(data)
	if !strings.Contains(s, "List<Order> & more") || !strings.Contains(s, "a<b") {
		t.Fatalf("escaped: %s", s)
	}
	order := []string{`"alpha"`, `"constraints"`, `"group_max"`, `"method"`, `"note"`, `"zeta"`}
	last := -1
	for _, k := range order {
		i := strings.Index(s, k)
		if i < 0 || i < last {
			t.Fatalf("keys not sorted or missing %s:\n%s", k, s)
		}
		last = i
	}
	if !strings.Contains(s, `"zeta": {"a":[1,2]}`) && !strings.Contains(s, `{"a": [1,2]}`) && !strings.Contains(s, `"a":[1,2]`) && !strings.Contains(s, `[`) {
		t.Fatalf("zeta lost: %s", s)
	}
	// Removing a key with nil keeps the rest; clearing constraints keeps method.
	if err := idx.SetConstraints(nil); err != nil {
		t.Fatal(err)
	}
	obj := readConfigObj(t, dir)
	if _, ok := obj["constraints"]; ok {
		t.Fatal("constraints not removed")
	}
	for _, k := range []string{"method", "note", "zeta", "alpha", "group_max"} {
		if _, ok := obj[k]; !ok {
			t.Fatalf("lost %s", k)
		}
	}
	if err := idx.SetConfigKeys(map[string]any{"note": nil}); err != nil {
		t.Fatal(err)
	}
	if _, ok := readConfigObj(t, dir)["note"]; ok {
		t.Fatal("note not removed")
	}
	if idx.MethodID != "design-first" {
		t.Fatalf("MethodID %q", idx.MethodID)
	}
}

func TestSetStageAndSetMethod(t *testing.T) {
	idx, dir := newModel(t)
	if err := idx.SetStage("design"); !errors.Is(err, ErrStageUnsupported) {
		t.Fatalf("freeform: %v", err)
	}
	res, err := idx.SetMethod("design-first")
	if err != nil || res.Stage != "design" || res.StageReset {
		t.Fatalf("%+v %v", res, err)
	}
	if err := idx.SetStage("nope"); !errors.Is(err, ErrUnknownStage) {
		t.Fatalf("%v", err)
	}
	if err := idx.SetStage("build"); err != nil {
		t.Fatal(err)
	}
	if got := idx.EffectiveStage(); got != "build" {
		t.Fatal(got)
	}
	if string(readConfigObj(t, dir)["stage"]) != `"build"` {
		t.Fatal("stage not persisted")
	}
	// Same-stage method keeps the stage.
	writeMethod(t, dir, "two.json", `{"schema_version":1,"id":"two","stages":[{"id":"design"},{"id":"build"},{"id":"ship"}]}`)
	idx = reopen(t, dir)
	res, err = idx.SetMethod("two")
	if err != nil || res.Stage != "build" || res.StageReset {
		t.Fatalf("keep: %+v %v", res, err)
	}
	// Stage invalid in the new method resets to initial and says so.
	writeMethod(t, dir, "other.json", `{"schema_version":1,"id":"other","stages":[{"id":"plan"},{"id":"go"}]}`)
	idx = reopen(t, dir)
	res, err = idx.SetMethod("other")
	if err != nil || res.Stage != "plan" || !res.StageReset || res.PreviousStage != "build" {
		t.Fatalf("reset: %+v %v", res, err)
	}
	// Stage-less target clears the stage; freeform removes the method key.
	res, err = idx.SetMethod("freeform")
	if err != nil || res.Stage != "" || !res.StageReset {
		t.Fatalf("freeform: %+v %v", res, err)
	}
	obj := readConfigObj(t, dir)
	if _, ok := obj["method"]; ok {
		t.Fatal("method key should be removed for freeform")
	}
	if _, ok := obj["stage"]; ok {
		t.Fatal("stage key should be removed")
	}
	if _, err := idx.SetMethod("ghost"); !errors.Is(err, ErrUnknownMethod) {
		t.Fatalf("%v", err)
	}
}

func TestStageGate(t *testing.T) {
	idx, _ := newModel(t)
	for _, s := range []Status{StatusAssigned, StatusDone} {
		if err := idx.StageGate(s); err != nil {
			t.Fatalf("freeform gated %s: %v", s, err)
		}
	}
	if _, err := idx.SetMethod("design-first"); err != nil {
		t.Fatal(err)
	}
	for _, s := range []Status{StatusAssigned, StatusInProgress, StatusDone} {
		err := idx.StageGate(s)
		if !errors.Is(err, ErrStageGate) || !strings.Contains(err.Error(), `"build"`) {
			t.Fatalf("%s: %v", s, err)
		}
	}
	for _, s := range []Status{StatusDraft, StatusSpecified, StatusBlocked} {
		if err := idx.StageGate(s); err != nil {
			t.Fatalf("%s gated: %v", s, err)
		}
	}
	if err := idx.SetStage("build"); err != nil {
		t.Fatal(err)
	}
	if err := idx.StageGate(StatusDone); err != nil {
		t.Fatal(err)
	}
}

func TestGraphViewCarriesMethodAndStage(t *testing.T) {
	idx, _ := newModel(t)
	if v := idx.GraphView(); v.Method != "freeform" || v.Stage != "" {
		t.Fatalf("%+v", v)
	}
	if _, err := idx.SetMethod("design-first"); err != nil {
		t.Fatal(err)
	}
	if v := idx.GraphView(); v.Method != "design-first" || v.Stage != "design" {
		t.Fatalf("%+v", v)
	}
}

func TestWatchReloadsMethodChanges(t *testing.T) {
	idx, dir := newModel(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notices := make(chan WatchNotice, 16)
	if err := idx.WatchNotices(ctx, func(n WatchNotice) { notices <- n }); err != nil {
		t.Fatal(err)
	}
	wait := func(cond func() bool) {
		t.Helper()
		deadline := time.After(4 * time.Second)
		for {
			select {
			case n := <-notices:
				if n.Err != nil {
					t.Fatal(n.Err)
				}
				idx.mu.Lock()
				ok := cond()
				idx.mu.Unlock()
				if ok {
					return
				}
			case <-deadline:
				t.Fatal("timed out waiting for reload")
			}
		}
	}
	// config.json change selects a method.
	if err := os.WriteFile(filepath.Join(dir, ".arch", "config.json"), []byte(`{"method":"design-first"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	wait(func() bool { return idx.activeMethodLocked().ID == "design-first" })

	// The methods directory did not exist at start; creating a method in it still reloads.
	writeMethod(t, dir, "mine.json", `{"schema_version":1,"id":"mine","guidance":"v1"}`)
	if err := os.WriteFile(filepath.Join(dir, ".arch", "config.json"), []byte(`{"method":"mine"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	wait(func() bool { return idx.activeMethodLocked().ID == "mine" })
	writeMethod(t, dir, "mine.json", `{"schema_version":1,"id":"mine","guidance":"v2"}`)
	wait(func() bool { return idx.activeMethodLocked().Guidance == "v2" })
	// A sibling .md edit reloads too.
	writeMethod(t, dir, "mine.json", `{"schema_version":1,"id":"mine","guidance":{"file":"mine.md"}}`)
	writeMethod(t, dir, "mine.md", "from file")
	wait(func() bool { return idx.activeMethodLocked().Guidance == "from file" })
	writeMethod(t, dir, "mine.md", "edited")
	wait(func() bool { return idx.activeMethodLocked().Guidance == "edited" })
}
