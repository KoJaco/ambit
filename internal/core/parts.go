package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/bmatcuk/doublestar/v4"
)

// Model scope (decision 0016). A whole-model project describes every file in the
// repository. A partial-model project describes only the parts listed in config.json, and
// drift ignores files outside them.
const (
	ScopeWhole   = "whole"
	ScopePartial = "partial"

	MaxParts        = 24
	MaxPartGlobs    = 32
	MaxPartRoots    = 64
	MaxPartName     = 120
	MaxPartGlobLen  = 300
	OpSetPart       = "set_part"
	OpDeletePart    = "delete_part"
	configKeyScope  = "scope"
	configKeyParts  = "parts"
	configKeyPolicy = "proposal_policy"
)

// ErrBadPart is returned for an invalid part, scope or part list.
var ErrBadPart = errors.New("invalid part")

// Part is one slice of the repository a partial model covers.
type Part struct {
	// ID is a lowercase slug, unique among parts.
	ID   string `json:"id"`
	Name string `json:"name"`
	// Include are repo-relative doublestar globs. Unowned files are reported by drift only
	// when they match an include glob of some part.
	Include []string `json:"include"`
	// RootNodeIDs are the top-level nodes of the part. They scope get_graph and the UI.
	RootNodeIDs []NodeID `json:"root_node_ids"`
}

func clonePart(p Part) Part {
	p.Include = append([]string{}, p.Include...)
	p.RootNodeIDs = append([]NodeID{}, p.RootNodeIDs...)
	return p
}

func clonePartList(in []Part) []Part {
	out := make([]Part, len(in))
	for i, p := range in {
		out[i] = clonePart(p)
	}
	return out
}

// validScope reports whether s is a scope value.
func validScope(s string) bool { return s == ScopeWhole || s == ScopePartial }

// normalizePart trims and validates one part. Root ids are checked for shape only; a root
// that does not exist yet is a load warning, not an error, so a part may be staged in the
// same proposal that creates its roots.
func normalizePart(p Part) (Part, error) {
	p.ID = strings.TrimSpace(p.ID)
	p.Name = strings.TrimSpace(p.Name)
	if !slugPattern.MatchString(p.ID) || len(p.ID) > maxExplicitIDLen {
		return p, fmt.Errorf("%w: id %q must be a lowercase slug of at most %d characters", ErrBadPart, p.ID, maxExplicitIDLen)
	}
	if p.Name == "" {
		p.Name = p.ID
	}
	if n := utf8.RuneCountInString(p.Name); n > MaxPartName {
		return p, fmt.Errorf("%w: part %s: name is %d characters (max %d)", ErrBadPart, p.ID, n, MaxPartName)
	}
	if len(p.Include) > MaxPartGlobs {
		return p, fmt.Errorf("%w: part %s: %d include globs (max %d)", ErrBadPart, p.ID, len(p.Include), MaxPartGlobs)
	}
	include := make([]string, 0, len(p.Include))
	seen := map[string]bool{}
	for _, g := range p.Include {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		if len(g) > MaxPartGlobLen {
			return p, fmt.Errorf("%w: part %s: include glob is longer than %d characters", ErrBadPart, p.ID, MaxPartGlobLen)
		}
		if !doublestar.ValidatePattern(toSlash(g)) {
			return p, fmt.Errorf("%w: part %s: invalid glob %q", ErrBadPart, p.ID, g)
		}
		if !seen[g] {
			seen[g] = true
			include = append(include, g)
		}
	}
	p.Include = include
	if len(p.RootNodeIDs) > MaxPartRoots {
		return p, fmt.Errorf("%w: part %s: %d root nodes (max %d)", ErrBadPart, p.ID, len(p.RootNodeIDs), MaxPartRoots)
	}
	roots := make([]NodeID, 0, len(p.RootNodeIDs))
	seenRoot := map[NodeID]bool{}
	for _, r := range p.RootNodeIDs {
		r = NodeID(strings.TrimSpace(string(r)))
		if !slugPattern.MatchString(string(r)) {
			return p, fmt.Errorf("%w: part %s: root node id %q is not a valid slug", ErrBadPart, p.ID, r)
		}
		if !seenRoot[r] {
			seenRoot[r] = true
			roots = append(roots, r)
		}
	}
	p.RootNodeIDs = roots
	return p, nil
}

// parseParts reads config.json "parts". Invalid entries are skipped with a warning, and
// entries past the limit are dropped.
func (idx *Index) parseParts(obj map[string]jsonRaw) []Part {
	raw, ok := obj[configKeyParts]
	if !ok || isNull(raw) {
		return nil
	}
	const file = "config.json"
	var items []jsonRaw
	if err := json.Unmarshal(raw, &items); err != nil {
		idx.warn(file, "parts must be an array; ignoring")
		return nil
	}
	var out []Part
	seen := map[string]bool{}
	for i, item := range items {
		if len(out) >= MaxParts {
			idx.warn(file, fmt.Sprintf("more than %d parts; ignoring parts[%d] and later", MaxParts, i))
			break
		}
		var p Part
		if err := json.Unmarshal(item, &p); err != nil {
			idx.warn(file, fmt.Sprintf("parts[%d] skipped: %v", i, err))
			continue
		}
		p, err := normalizePart(p)
		if err != nil {
			idx.warn(file, fmt.Sprintf("parts[%d] skipped: %v", i, err))
			continue
		}
		if seen[p.ID] {
			idx.warn(file, fmt.Sprintf("parts[%d] skipped: duplicate id %q", i, p.ID))
			continue
		}
		seen[p.ID] = true
		out = append(out, p)
	}
	return out
}

// warnParts reports root node ids that do not exist. It runs after the nodes are loaded.
func (idx *Index) warnParts() {
	for _, p := range idx.Parts {
		for _, r := range p.RootNodeIDs {
			if _, ok := idx.Nodes[r]; !ok {
				idx.warn("config.json", fmt.Sprintf("part %q: root node %q does not exist", p.ID, r))
			}
		}
	}
}

// ScopeInfo is the model scope and its parts.
type ScopeInfo struct {
	Scope string `json:"scope"`
	Parts []Part `json:"parts"`
}

// ScopeInfo returns the effective scope and a copy of the parts.
func (idx *Index) ScopeInfo() ScopeInfo {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return ScopeInfo{Scope: idx.scopeLocked(), Parts: clonePartList(idx.Parts)}
}

func (idx *Index) scopeLocked() string {
	if idx.Scope == ScopePartial {
		return ScopePartial
	}
	return ScopeWhole
}

// PartByID returns a copy of the part with the given id.
func (idx *Index) PartByID(id string) (Part, bool) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.partLocked(id)
}

func (idx *Index) partLocked(id string) (Part, bool) {
	for _, p := range idx.Parts {
		if p.ID == id {
			return clonePart(p), true
		}
	}
	return Part{}, false
}

// SetScope writes the scope to config.json ("whole" removes the key). Human surfaces only.
func (idx *Index) SetScope(scope string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.setScopeLocked(scope)
}

func (idx *Index) setScopeLocked(scope string) error {
	if !validScope(scope) {
		return fmt.Errorf("%w: scope %q must be %s or %s", ErrBadPart, scope, ScopeWhole, ScopePartial)
	}
	if scope == ScopeWhole {
		return idx.setConfigKeysLocked(map[string]any{configKeyScope: nil})
	}
	return idx.setConfigKeysLocked(map[string]any{configKeyScope: ScopePartial})
}

// SetPart adds a part or replaces the one with the same id.
func (idx *Index) SetPart(p Part) (Part, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.setPartLocked(p)
}

func (idx *Index) setPartLocked(p Part) (Part, error) {
	p, err := normalizePart(p)
	if err != nil {
		return p, err
	}
	// Re-read from disk so a part list edited elsewhere is not overwritten.
	current, err := idx.readPartsFromDiskLocked()
	if err != nil {
		return p, err
	}
	replaced := false
	for i := range current {
		if current[i].ID == p.ID {
			current[i], replaced = p, true
		}
	}
	if !replaced {
		if len(current) >= MaxParts {
			return p, fmt.Errorf("%w: %d parts already (max %d)", ErrBadPart, len(current), MaxParts)
		}
		current = append(current, p)
	}
	return p, idx.setConfigKeysLocked(map[string]any{configKeyParts: current})
}

// DeletePart removes a part by id.
func (idx *Index) DeletePart(id string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.deletePartLocked(id)
}

func (idx *Index) deletePartLocked(id string) error {
	current, err := idx.readPartsFromDiskLocked()
	if err != nil {
		return err
	}
	out := current[:0:0]
	found := false
	for _, p := range current {
		if p.ID == id {
			found = true
			continue
		}
		out = append(out, p)
	}
	if !found {
		return fmt.Errorf("%w: no part %q", ErrBadPart, id)
	}
	if len(out) == 0 {
		return idx.setConfigKeysLocked(map[string]any{configKeyParts: nil})
	}
	return idx.setConfigKeysLocked(map[string]any{configKeyParts: out})
}

// readPartsFromDiskLocked parses the parts currently in config.json (invalid entries are
// dropped, as at load). Diagnostics are not touched.
func (idx *Index) readPartsFromDiskLocked() ([]Part, error) {
	data, err := os.ReadFile(filepath.Join(idx.arch(), "config.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	obj, err := parseObject(data)
	if err != nil {
		return nil, err
	}
	scratch := &Index{}
	return scratch.parseParts(obj), nil
}

// partsStateHash fingerprints the scope and parts keys of config.json, so a staged
// set_part or delete_part is stale when another change touched them but not when an
// unrelated config key (the stage, say) changed.
func (idx *Index) partsStateHash() (string, error) {
	data, ok, err := readOptional(filepath.Join(idx.arch(), "config.json"))
	if err != nil {
		return "", err
	}
	h := sha256.New()
	if ok {
		if obj, perr := parseObject(data); perr == nil {
			for _, k := range []string{configKeyScope, configKeyParts} {
				h.Write([]byte(k))
				h.Write([]byte{0})
				h.Write(obj[k])
				h.Write([]byte{0})
			}
		} else {
			h.Write(data)
		}
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// partFileMatch reports whether path matches any include glob of p.
func partFileMatch(p Part, path string) (bool, error) {
	return matchGlobs(p.Include, path)
}

// stageSetPart builds a set_part operation. Config-level: it carries no node files.
func (idx *Index) stageSetPart(in StageOp) (operation, *materialBody, error) {
	if in.Part == nil {
		return operation{}, nil, fmt.Errorf("%w: set_part needs a part", ErrBadOperation)
	}
	p, err := normalizePart(*in.Part)
	if err != nil {
		return operation{}, nil, err
	}
	exists := false
	for _, cur := range idx.Parts {
		if cur.ID == p.ID {
			exists = true
		}
	}
	if !exists && len(idx.Parts) >= MaxParts {
		return operation{}, nil, fmt.Errorf("%w: %d parts already (max %d)", ErrBadPart, len(idx.Parts), MaxParts)
	}
	hash, err := idx.partsStateHash()
	if err != nil {
		return operation{}, nil, err
	}
	return operation{Op: OpSetPart, NodeID: p.ID, BaseHash: &hash, Part: &p, Status: opStatusPending}, nil, nil
}

// stageDeletePart builds a delete_part operation.
func (idx *Index) stageDeletePart(in StageOp) (operation, *materialBody, error) {
	id := ""
	if in.Part != nil {
		id = strings.TrimSpace(in.Part.ID)
	}
	if !slugPattern.MatchString(id) {
		return operation{}, nil, fmt.Errorf("%w: delete_part needs a part id", ErrBadOperation)
	}
	found := false
	for _, cur := range idx.Parts {
		if cur.ID == id {
			found = true
		}
	}
	if !found {
		return operation{}, nil, fmt.Errorf("%w: no part %q to delete", ErrBadPart, id)
	}
	hash, err := idx.partsStateHash()
	if err != nil {
		return operation{}, nil, err
	}
	return operation{Op: OpDeletePart, NodeID: id, BaseHash: &hash, Status: opStatusPending}, nil, nil
}
