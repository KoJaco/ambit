package core

import (
	"encoding/json"
	"fmt"
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
	return idx.setConstraintsLocked(in)
}

// normalizeConstraintList validates a replace-all list: count, each entry, unique ids.
func normalizeConstraintList(in []Constraint) ([]Constraint, error) {
	if len(in) > MaxConstraints {
		return nil, fmt.Errorf("%w: %d constraints (max %d)", ErrBadConstraint, len(in), MaxConstraints)
	}
	out := make([]Constraint, 0, len(in))
	seen := map[string]bool{}
	for _, c := range in {
		n, err := normalizeConstraint(c)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBadConstraint, err)
		}
		if seen[n.ID] {
			return nil, fmt.Errorf("%w: duplicate id %q", ErrBadConstraint, n.ID)
		}
		seen[n.ID] = true
		out = append(out, n)
	}
	return out, nil
}

func (idx *Index) setConstraintsLocked(in []Constraint) error {
	out, err := normalizeConstraintList(in)
	if err != nil {
		return err
	}
	// A replace-all list that omits a known id's unknown keys keeps the ones on disk.
	for i := range out {
		if out[i].Unknown != nil {
			continue
		}
		for _, cur := range idx.Constraints {
			if cur.ID == out[i].ID {
				out[i].Unknown = cloneRaw(cur.Unknown)
			}
		}
	}
	if len(out) == 0 {
		return idx.setConfigKeysLocked(map[string]any{"constraints": nil})
	}
	return idx.setConfigKeysLocked(map[string]any{"constraints": out})
}

// ConstraintList returns a copy of the project constraints in file order.
func (idx *Index) ConstraintList() []Constraint {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return append([]Constraint{}, idx.Constraints...)
}
