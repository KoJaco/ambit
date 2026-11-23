package core

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// normalizeConstraint trims, defaults severity to must, and validates one entry.
func normalizeConstraint(c Constraint) (Constraint, error) {
	c.ID = strings.TrimSpace(c.ID)
	c.Text = strings.TrimSpace(c.Text)
	c.Severity = strings.TrimSpace(c.Severity)
	if c.Severity == "" {
		c.Severity = ConstraintMust
	}
	if !slugPattern.MatchString(c.ID) {
		return c, fmt.Errorf("id %q is not a valid slug", c.ID)
	}
	if n := utf8.RuneCountInString(c.Text); n < 1 || n > MaxConstraintText {
		return c, fmt.Errorf("constraint %s: text must be 1..%d characters", c.ID, MaxConstraintText)
	}
	if c.Severity != ConstraintMust && c.Severity != ConstraintShould {
		return c, fmt.Errorf("constraint %s: severity %q must be must or should", c.ID, c.Severity)
	}
	return c, nil
}

// parseConstraints reads config.json "constraints". Invalid entries are skipped with a
// warning; a non-array value is ignored with a warning. Count over the limit is tolerated.
func (idx *Index) parseConstraints(obj map[string]jsonRaw) []Constraint {
	raw, ok := obj["constraints"]
	if !ok || isNull(raw) {
		return nil
	}
	const file = "config.json"
	var items []jsonRaw
	if err := json.Unmarshal(raw, &items); err != nil {
		idx.warn(file, "constraints must be an array; ignoring")
		return nil
	}
	var out []Constraint
	seen := map[string]bool{}
	for i, item := range items {
		var c Constraint
		if err := json.Unmarshal(item, &c); err != nil {
			idx.warn(file, fmt.Sprintf("constraints[%d] skipped: %v", i, err))
			continue
		}
		c, err := normalizeConstraint(c)
		if err != nil {
			idx.warn(file, fmt.Sprintf("constraints[%d] skipped: %v", i, err))
			continue
		}
		if seen[c.ID] {
			idx.warn(file, fmt.Sprintf("constraints[%d] skipped: duplicate id %q", i, c.ID))
			continue
		}
		seen[c.ID] = true
		out = append(out, c)
	}
	if len(out) > MaxConstraints {
		idx.warn(file, fmt.Sprintf("%d constraints exceed the limit of %d", len(out), MaxConstraints))
	}
	return out
}

// SetConstraints replaces the project constraints in config.json (replace-all). Other
// config keys, including unknown ones, are preserved. An empty list removes the key.
func (idx *Index) SetConstraints(in []Constraint) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if len(in) > MaxConstraints {
		return fmt.Errorf("%w: %d constraints (max %d)", ErrBadConstraint, len(in), MaxConstraints)
	}
	out := make([]Constraint, 0, len(in))
	seen := map[string]bool{}
	for _, c := range in {
		n, err := normalizeConstraint(c)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrBadConstraint, err)
		}
		if seen[n.ID] {
			return fmt.Errorf("%w: duplicate id %q", ErrBadConstraint, n.ID)
		}
		seen[n.ID] = true
		out = append(out, n)
	}
	obj := cloneRaw(idx.configUnknown)
	if obj == nil {
		obj = map[string]jsonRaw{}
	}
	if len(out) == 0 {
		delete(obj, "constraints")
	} else {
		raw, err := rawOf(out)
		if err != nil {
			return err
		}
		obj["constraints"] = raw
	}
	data, err := encodeObject(nil, nil, obj)
	if err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(idx.arch(), "config.json"), data); err != nil {
		return err
	}
	return idx.reload()
}
