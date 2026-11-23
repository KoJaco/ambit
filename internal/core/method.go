package core

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

// Limits on method files (decision 0013).
const (
	MaxMethodGuidance = 6000
	MaxMethodPreamble = 1500
	MaxMethodStages   = 8

	// FreeformMethod is the default method id.
	FreeformMethod = "freeform"

	methodSourceBuiltin = "builtin"
	methodSourceUser    = "user"
)

//go:embed methods/*.json methods/*.md
var methodFS embed.FS

// builtinOrder is the display order of the embedded methods.
var builtinOrder = []string{"freeform", "design-first", "map-existing"}

// MethodStage is one project stage of a method.
type MethodStage struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Preamble string `json:"preamble,omitempty"`
}

// MethodGates maps a stage id to the node statuses agents may not set while in it.
type MethodGates struct {
	Status map[string][]string `json:"status,omitempty"`
}

// MethodCheck holds check_model defaults.
type MethodCheck struct {
	IncludeDrift bool `json:"include_drift"`
}

// Method is a resolved workflow method: file references are read and `extends` is applied.
type Method struct {
	SchemaVersion int                    `json:"schema_version"`
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description,omitempty"`
	Guidance      string                 `json:"guidance"`
	BriefPreamble string                 `json:"brief_preamble,omitempty"`
	Stages        []MethodStage          `json:"stages,omitempty"`
	InitialStage  string                 `json:"initial_stage,omitempty"`
	Gates         MethodGates            `json:"gates"`
	Lint          map[string]LintSetting `json:"lint"`
	Check         MethodCheck            `json:"check"`
	Extends       string                 `json:"extends,omitempty"`
	// Source is "builtin" or "user". Shadows is true for a user method whose id is a
	// built-in id.
	Source  string `json:"source"`
	Shadows bool   `json:"shadows_builtin,omitempty"`
}

// MethodSummary is one row of ListMethods.
type MethodSummary struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Source      string   `json:"source"`
	Shadows     bool     `json:"shadows_builtin,omitempty"`
	Stages      []string `json:"stages,omitempty"`
	Active      bool     `json:"active"`
}

// StageIndex returns the position of a stage id, or -1.
func (m *Method) StageIndex(id string) int {
	for i, s := range m.Stages {
		if s.ID == id {
			return i
		}
	}
	return -1
}

// textRef is a JSON string or {"file": "name"}.
type textRef struct {
	text string
	file string
}

func (t *textRef) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '"' {
		return json.Unmarshal(data, &t.text)
	}
	var o struct {
		File string `json:"file"`
	}
	if err := json.Unmarshal(data, &o); err != nil || o.File == "" {
		return errors.New(`must be a string or {"file": "name"}`)
	}
	t.file = o.File
	return nil
}

type stageFile struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Preamble *textRef `json:"preamble"`
}

type methodFile struct {
	SchemaVersion *int                    `json:"schema_version"`
	ID            string                  `json:"id"`
	Name          *string                 `json:"name"`
	Description   *string                 `json:"description"`
	Guidance      *textRef                `json:"guidance"`
	BriefPreamble *textRef                `json:"brief_preamble"`
	Stages        *[]stageFile            `json:"stages"`
	InitialStage  *string                 `json:"initial_stage"`
	Gates         *MethodGates            `json:"gates"`
	Lint          *map[string]LintSetting `json:"lint"`
	Check         *struct {
		IncludeDrift *bool `json:"include_drift"`
	} `json:"check"`
	Extends string `json:"extends"`
}

var (
	builtinOnce    sync.Once
	builtinMethods map[string]*Method
	builtinErr     error
)

func loadBuiltins() (map[string]*Method, error) {
	builtinOnce.Do(func() {
		builtinMethods = map[string]*Method{}
		for _, id := range builtinOrder {
			data, err := methodFS.ReadFile("methods/" + id + ".json")
			if err != nil {
				builtinErr = err
				return
			}
			read := func(name string) (string, error) {
				b, err := methodFS.ReadFile("methods/" + name)
				if err != nil {
					return "", err
				}
				s := string(b)
				s = strings.ReplaceAll(s, "{{shallow_model_guidance}}", ShallowModelGuidance)
				s = strings.ReplaceAll(s, "{{authoring_guidance}}", AuthoringGuidance)
				return s, nil
			}
			m, _, err := parseMethod(data, id, nil, read, methodSourceBuiltin)
			if err != nil {
				builtinErr = fmt.Errorf("built-in method %s: %w", id, err)
				return
			}
			builtinMethods[id] = m
		}
	})
	return builtinMethods, builtinErr
}

// BuiltinMethod returns an embedded method by id.
func BuiltinMethod(id string) (*Method, bool) {
	b, err := loadBuiltins()
	if err != nil {
		return nil, false
	}
	m, ok := b[id]
	return m, ok
}

func mustFreeform() *Method {
	m, _ := BuiltinMethod(FreeformMethod)
	if m == nil {
		// Unreachable unless the embedded files are broken (covered by a test).
		return &Method{SchemaVersion: 1, ID: FreeformMethod, Name: "Freeform", Lint: map[string]LintSetting{}, Source: methodSourceBuiltin}
	}
	return m
}

func badMethod(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrBadMethod, fmt.Sprintf(format, a...))
}

// parseMethod decodes and validates a method file. wantID is the id the file must carry
// (its filename stem); base is the extended built-in or nil. The returned warnings are
// non-fatal (for example unknown lint rule ids).
func parseMethod(data []byte, wantID string, base *Method, read func(string) (string, error), source string) (*Method, []string, error) {
	var f methodFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, nil, badMethod("%v", err)
	}
	if f.SchemaVersion == nil || *f.SchemaVersion != 1 {
		return nil, nil, badMethod("schema_version must be 1")
	}
	if !slugPattern.MatchString(f.ID) {
		return nil, nil, badMethod("id %q is not a valid slug", f.ID)
	}
	if f.ID != wantID {
		return nil, nil, badMethod("id %q does not match the file name %q", f.ID, wantID)
	}
	m := &Method{SchemaVersion: 1, ID: f.ID, Name: f.ID, Source: source, Lint: map[string]LintSetting{}, Extends: f.Extends}
	if base != nil {
		c := *base
		m.Name, m.Description, m.Guidance, m.BriefPreamble = c.Name, c.Description, c.Guidance, c.BriefPreamble
		m.Stages = append([]MethodStage(nil), c.Stages...)
		m.InitialStage = c.InitialStage
		m.Gates = MethodGates{Status: map[string][]string{}}
		for k, v := range c.Gates.Status {
			m.Gates.Status[k] = append([]string(nil), v...)
		}
		for k, v := range c.Lint {
			m.Lint[k] = v
		}
		m.Check = c.Check
	}
	text := func(field string, t *textRef) (string, error) {
		if t.file == "" {
			return t.text, nil
		}
		if t.file != filepath.Base(t.file) || strings.ContainsAny(t.file, `/\`) || t.file == ".." || t.file == "." {
			return "", badMethod("%s: file %q must be a plain file name beside the method", field, t.file)
		}
		s, err := read(t.file)
		if err != nil {
			return "", badMethod("%s: cannot read %q: %v", field, t.file, err)
		}
		return s, nil
	}
	var err error
	if f.Name != nil {
		m.Name = strings.TrimSpace(*f.Name)
	}
	if f.Description != nil {
		m.Description = strings.TrimSpace(*f.Description)
	}
	if f.Guidance != nil {
		if m.Guidance, err = text("guidance", f.Guidance); err != nil {
			return nil, nil, err
		}
		m.Guidance = strings.TrimSpace(m.Guidance)
	}
	if f.BriefPreamble != nil {
		if m.BriefPreamble, err = text("brief_preamble", f.BriefPreamble); err != nil {
			return nil, nil, err
		}
		m.BriefPreamble = strings.TrimSpace(m.BriefPreamble)
	}
	stagesGiven := f.Stages != nil
	if stagesGiven {
		m.Stages = nil
		m.InitialStage = ""
		seen := map[string]bool{}
		for _, s := range *f.Stages {
			if !slugPattern.MatchString(s.ID) {
				return nil, nil, badMethod("stage id %q is not a valid slug", s.ID)
			}
			if seen[s.ID] {
				return nil, nil, badMethod("duplicate stage id %q", s.ID)
			}
			seen[s.ID] = true
			st := MethodStage{ID: s.ID, Name: strings.TrimSpace(s.Name)}
			if st.Name == "" {
				st.Name = s.ID
			}
			if s.Preamble != nil {
				p, err := text("stage "+s.ID+" preamble", s.Preamble)
				if err != nil {
					return nil, nil, err
				}
				st.Preamble = strings.TrimSpace(p)
			}
			m.Stages = append(m.Stages, st)
		}
		if f.Gates == nil {
			m.Gates = MethodGates{}
		}
	}
	if f.InitialStage != nil {
		m.InitialStage = strings.TrimSpace(*f.InitialStage)
	} else if stagesGiven && len(m.Stages) > 0 {
		m.InitialStage = m.Stages[0].ID
	}
	if f.Gates != nil {
		m.Gates = *f.Gates
	}
	var warns []string
	if f.Lint != nil {
		for rule, set := range *f.Lint {
			m.Lint[rule] = set
		}
	}
	for rule, set := range m.Lint {
		if err := set.validate(rule); err != nil {
			return nil, nil, err
		}
		if !isLintRule(rule) {
			warns = append(warns, fmt.Sprintf("lint rule %q is unknown and is ignored", rule))
		}
	}
	if f.Check != nil && f.Check.IncludeDrift != nil {
		m.Check.IncludeDrift = *f.Check.IncludeDrift
	}

	// Limits and cross-checks on the resolved method.
	if n := utf8.RuneCountInString(m.Guidance); n > MaxMethodGuidance {
		return nil, nil, badMethod("guidance is %d characters (max %d)", n, MaxMethodGuidance)
	}
	if n := utf8.RuneCountInString(m.BriefPreamble); n > MaxMethodPreamble {
		return nil, nil, badMethod("brief_preamble is %d characters (max %d)", n, MaxMethodPreamble)
	}
	if len(m.Stages) > MaxMethodStages {
		return nil, nil, badMethod("%d stages (max %d)", len(m.Stages), MaxMethodStages)
	}
	for _, s := range m.Stages {
		if n := utf8.RuneCountInString(s.Preamble); n > MaxMethodPreamble {
			return nil, nil, badMethod("stage %s preamble is %d characters (max %d)", s.ID, n, MaxMethodPreamble)
		}
	}
	if len(m.Stages) > 0 && m.StageIndex(m.InitialStage) < 0 {
		return nil, nil, badMethod("initial_stage %q is not one of the stages", m.InitialStage)
	}
	if len(m.Stages) == 0 && m.InitialStage != "" {
		return nil, nil, badMethod("initial_stage %q set but the method has no stages", m.InitialStage)
	}
	for stage, statuses := range m.Gates.Status {
		if m.StageIndex(stage) < 0 {
			return nil, nil, badMethod("gates.status names unknown stage %q", stage)
		}
		for _, s := range statuses {
			if !validStatus(Status(s)) {
				return nil, nil, badMethod("gates.status.%s: invalid status %q", stage, s)
			}
		}
	}
	sort.Strings(warns)
	return m, warns, nil
}

// loadMethods reads .arch/methods and resolves the active method. It never fails: an
// invalid file is skipped with a warning and an unknown or missing active method falls
// back to freeform.
func (idx *Index) loadMethods() {
	idx.userMethods = map[string]*Method{}
	dir := filepath.Join(idx.arch(), "methods")
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || isScratch(name) || !strings.HasSuffix(name, ".json") {
				continue
			}
			path := filepath.Join(dir, name)
			id := strings.TrimSuffix(name, ".json")
			m, warns, err := loadUserMethod(dir, id)
			if err != nil {
				idx.warn(path, fmt.Sprintf("method skipped: %v", err))
				continue
			}
			for _, w := range warns {
				idx.warn(path, w)
			}
			if _, ok := BuiltinMethod(id); ok {
				m.Shadows = true
				idx.warn(path, fmt.Sprintf("method %q shadows the built-in method of the same id", id))
			}
			idx.userMethods[id] = m
		}
	}

	want := idx.MethodID
	if want == "" {
		want = FreeformMethod
	}
	m := idx.lookupMethod(want)
	if m == nil {
		idx.warn(filepath.Join(idx.arch(), "config.json"), fmt.Sprintf("method %q is not defined; using %s", want, FreeformMethod))
		m = idx.lookupMethod(FreeformMethod)
		if m == nil {
			m = mustFreeform()
		}
	}
	idx.activeMethod = m
	if idx.StageID != "" {
		switch {
		case len(m.Stages) == 0:
			idx.warn(filepath.Join(idx.arch(), "config.json"), fmt.Sprintf("stage %q ignored: method %q has no stages", idx.StageID, m.ID))
		case m.StageIndex(idx.StageID) < 0:
			idx.warn(filepath.Join(idx.arch(), "config.json"), fmt.Sprintf("stage %q is not a stage of method %q; using %q", idx.StageID, m.ID, m.InitialStage))
		}
	}
}

// maxMethodFileBytes caps how much of a method JSON or {file} text is read.
const maxMethodFileBytes = 64 << 10

// readMethodFile reads a regular file of at most maxMethodFileBytes. Symlinks, FIFOs,
// devices and other non-regular files are rejected without being opened, so a method
// cannot make ambit read outside .arch or block on a pipe.
func readMethodFile(path string) ([]byte, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file (symlinks and special files are not allowed)", filepath.Base(path))
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxMethodFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxMethodFileBytes {
		return nil, fmt.Errorf("%s is larger than %d KiB", filepath.Base(path), maxMethodFileBytes>>10)
	}
	return data, nil
}

func loadUserMethod(dir, id string) (*Method, []string, error) {
	data, err := readMethodFile(filepath.Join(dir, id+".json"))
	if err != nil {
		return nil, nil, err
	}
	var peek struct {
		Extends string `json:"extends"`
	}
	_ = json.Unmarshal(data, &peek)
	var base *Method
	if peek.Extends != "" {
		b, ok := BuiltinMethod(peek.Extends)
		if !ok {
			return nil, nil, badMethod("extends %q is not a built-in method", peek.Extends)
		}
		base = b
	}
	read := func(name string) (string, error) {
		b, err := readMethodFile(filepath.Join(dir, name))
		return string(b), err
	}
	return parseMethod(data, id, base, read, methodSourceUser)
}

// lookupMethod finds a user method, then a built-in. Caller may hold idx.mu or be loading.
func (idx *Index) lookupMethod(id string) *Method {
	if m, ok := idx.userMethods[id]; ok {
		return m
	}
	if m, ok := BuiltinMethod(id); ok {
		return m
	}
	return nil
}

// ActiveMethod returns the method selected by config.json, or freeform when none is
// selected or the named one is not usable. The result must not be modified.
func (idx *Index) ActiveMethod() *Method {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.activeMethodLocked()
}

func (idx *Index) activeMethodLocked() *Method {
	if idx.activeMethod != nil {
		return idx.activeMethod
	}
	return mustFreeform()
}

// MethodByID returns a user or built-in method (a user method shadows a built-in).
func (idx *Index) MethodByID(id string) (*Method, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if m := idx.lookupMethod(id); m != nil {
		return m, nil
	}
	return nil, fmt.Errorf("%w: %q", ErrUnknownMethod, id)
}

// ListMethods lists the built-in methods (in a fixed order) then user-only methods sorted
// by id. A shadowing user method replaces the built-in row.
func (idx *Index) ListMethods() []MethodSummary {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	active := idx.activeMethodLocked().ID
	row := func(m *Method) MethodSummary {
		s := MethodSummary{ID: m.ID, Name: m.Name, Description: m.Description, Source: m.Source, Shadows: m.Shadows, Active: m.ID == active}
		for _, st := range m.Stages {
			s.Stages = append(s.Stages, st.ID)
		}
		return s
	}
	var out []MethodSummary
	for _, id := range builtinOrder {
		if m := idx.lookupMethod(id); m != nil {
			out = append(out, row(m))
		}
	}
	var extra []string
	for id := range idx.userMethods {
		if _, ok := BuiltinMethod(id); !ok {
			extra = append(extra, id)
		}
	}
	sort.Strings(extra)
	for _, id := range extra {
		out = append(out, row(idx.userMethods[id]))
	}
	return out
}

// EffectiveStage is the stage in force: the configured stage when valid for the active
// method, otherwise its initial stage. Empty for a stage-less method.
func (idx *Index) EffectiveStage() string {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.effectiveStageLocked()
}

func (idx *Index) effectiveStageLocked() string {
	m := idx.activeMethodLocked()
	if len(m.Stages) == 0 {
		return ""
	}
	if m.StageIndex(idx.StageID) >= 0 {
		return idx.StageID
	}
	return m.InitialStage
}

// methodPreambleLocked is the text prepended to a brief: method preamble, then stage
// preamble, each only when non-empty and followed by a blank line.
func (idx *Index) methodPreambleLocked() string {
	m := idx.activeMethodLocked()
	var b strings.Builder
	if m.BriefPreamble != "" {
		b.WriteString(m.BriefPreamble + "\n\n")
	}
	if st := idx.effectiveStageLocked(); st != "" {
		if i := m.StageIndex(st); i >= 0 && m.Stages[i].Preamble != "" {
			b.WriteString(m.Stages[i].Preamble + "\n\n")
		}
	}
	return b.String()
}
