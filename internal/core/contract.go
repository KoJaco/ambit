package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"
)

// Limits on contract and constraint content (decision 0012). New values over them are
// rejected on write; values already on disk load as-is.
const (
	MaxContractPayloads = 8000
	MaxContractItems    = 32
	MaxContractItemLen  = 300
	MaxEdgeContractText = 4000
	MaxEdgeContractRef  = 300
	MaxConstraints      = 24
	MaxConstraintText   = 300
)

// Edge contract formats. Unknown values on disk load as-is; new unknown values are rejected.
const (
	ContractFormatOpenAPI    = "openapi"
	ContractFormatJSONSchema = "jsonschema"
	ContractFormatText       = "text"
)

// Constraint severities.
const (
	ConstraintMust   = "must"
	ConstraintShould = "should"
)

var (
	ErrBadContract     = errors.New("invalid contract")
	ErrBadConstraint   = errors.New("invalid constraint")
	ErrBadDependency   = errors.New("invalid depends_on")
	ErrDependencyCycle = errors.New("dependency cycle")
	ErrHasDependents   = errors.New("node is depended on")
)

// NodeContract is the structured interface description of a node. Nil on a Node means none.
type NodeContract struct {
	Payloads   string   `json:"payloads,omitempty"`
	Invariants []string `json:"invariants,omitempty"`
	Errors     []string `json:"errors,omitempty"`
	OwnsData   []string `json:"owns_data,omitempty"`
	// Unknown holds keys this version does not know, preserved on rewrite.
	Unknown map[string]jsonRaw `json:"-"`
}

// EdgeContract describes what is on the wire for a relationship. Stored as opaque text.
type EdgeContract struct {
	Format string `json:"format,omitempty"`
	Ref    string `json:"ref,omitempty"`
	Text   string `json:"text,omitempty"`
	// Unknown holds keys this version does not know, preserved on rewrite.
	Unknown map[string]jsonRaw `json:"-"`
}

// Constraint is one project-wide rule from config.json.
type Constraint struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Severity string `json:"severity"`
	// Unknown holds keys this version does not know, preserved on rewrite.
	Unknown map[string]jsonRaw `json:"-"`
}

// IsEmpty reports whether the contract carries no content (nil is empty).
func (c *NodeContract) IsEmpty() bool {
	return c == nil || (c.Payloads == "" && len(c.Invariants) == 0 && len(c.Errors) == 0 && len(c.OwnsData) == 0 && len(c.Unknown) == 0)
}

// IsEmpty reports whether the contract carries no content (nil is empty).
func (c *EdgeContract) IsEmpty() bool {
	return c == nil || (c.Format == "" && c.Ref == "" && c.Text == "" && len(c.Unknown) == 0)
}

// normalizeNodeContract returns a deep copy, or nil when empty.
func normalizeNodeContract(c *NodeContract) *NodeContract {
	if c.IsEmpty() {
		return nil
	}
	out := &NodeContract{Payloads: c.Payloads, Unknown: cloneRaw(c.Unknown)}
	if len(c.Invariants) > 0 {
		out.Invariants = cloneStrings(c.Invariants)
	}
	if len(c.Errors) > 0 {
		out.Errors = cloneStrings(c.Errors)
	}
	if len(c.OwnsData) > 0 {
		out.OwnsData = cloneStrings(c.OwnsData)
	}
	return out
}

func normalizeEdgeContract(c *EdgeContract) *EdgeContract {
	if c.IsEmpty() {
		return nil
	}
	cp := *c
	cp.Unknown = cloneRaw(c.Unknown)
	return &cp
}

func validateContractList(name string, items []string) error {
	if len(items) > MaxContractItems {
		return fmt.Errorf("%w: %s has %d items (max %d)", ErrBadContract, name, len(items), MaxContractItems)
	}
	for i, it := range items {
		if utf8.RuneCountInString(it) > MaxContractItemLen {
			return fmt.Errorf("%w: %s item %d exceeds %d characters", ErrBadContract, name, i+1, MaxContractItemLen)
		}
	}
	return nil
}

func validateNodeContract(c *NodeContract) error {
	if c.IsEmpty() {
		return nil
	}
	if utf8.RuneCountInString(c.Payloads) > MaxContractPayloads {
		return fmt.Errorf("%w: payloads exceeds %d characters", ErrBadContract, MaxContractPayloads)
	}
	for _, l := range []struct {
		name  string
		items []string
	}{{"invariants", c.Invariants}, {"errors", c.Errors}, {"owns_data", c.OwnsData}} {
		if err := validateContractList(l.name, l.items); err != nil {
			return err
		}
	}
	return nil
}

func validEdgeFormat(f string) bool {
	switch f {
	case ContractFormatOpenAPI, ContractFormatJSONSchema, ContractFormatText:
		return true
	}
	return false
}

// refEscapes reports whether a repo path contains a ".." segment.
func refEscapes(ref string) bool {
	for _, seg := range strings.FieldsFunc(ref, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return true
		}
	}
	return false
}

func validateEdgeContract(c *EdgeContract) error {
	if c.IsEmpty() {
		return nil
	}
	if c.Format != "" && !validEdgeFormat(c.Format) {
		return fmt.Errorf("%w: unknown format %q (valid: openapi, jsonschema, text)", ErrBadContract, c.Format)
	}
	if utf8.RuneCountInString(c.Text) > MaxEdgeContractText {
		return fmt.Errorf("%w: text exceeds %d characters", ErrBadContract, MaxEdgeContractText)
	}
	if utf8.RuneCountInString(c.Ref) > MaxEdgeContractRef {
		return fmt.Errorf("%w: ref exceeds %d characters", ErrBadContract, MaxEdgeContractRef)
	}
	if refEscapes(c.Ref) {
		return fmt.Errorf("%w: ref %q must not contain '..'", ErrBadContract, c.Ref)
	}
	return nil
}

// patchNodeContract returns the contract to store for a write. It validates only when the
// value differs from current, so an unchanged over-limit value from disk can be re-sent.
func patchNodeContract(in, current *NodeContract) (*NodeContract, error) {
	next := normalizeNodeContract(in)
	if next != nil && next.Unknown == nil && current != nil {
		next.Unknown = cloneRaw(current.Unknown)
	}
	if !reflect.DeepEqual(next, current) {
		if err := validateNodeContract(next); err != nil {
			return nil, err
		}
	}
	return next, nil
}

func patchEdgeContract(in, current *EdgeContract) (*EdgeContract, error) {
	next := normalizeEdgeContract(in)
	if next != nil && next.Unknown == nil && current != nil {
		next.Unknown = cloneRaw(current.Unknown)
	}
	if !reflect.DeepEqual(next, current) {
		if err := validateEdgeContract(next); err != nil {
			return nil, err
		}
	}
	return next, nil
}

func decodeNodeContract(raw jsonRaw) (*NodeContract, error) {
	var c NodeContract
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	return normalizeNodeContract(&c), nil
}

func decodeEdgeContract(raw jsonRaw) (*EdgeContract, error) {
	var c EdgeContract
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	return normalizeEdgeContract(&c), nil
}

var (
	nodeContractKeys = []string{"payloads", "invariants", "errors", "owns_data"}
	edgeContractKeys = []string{"format", "ref", "text"}
	constraintKeys   = []string{"id", "text", "severity"}
)

// marshalWithUnknown appends unknown keys (sorted) to an encoded object of known keys.
func marshalWithUnknown(known any, unknown map[string]jsonRaw) ([]byte, error) {
	base, err := marshalNoEscape(known)
	if err != nil {
		return nil, err
	}
	if len(unknown) == 0 {
		return base, nil
	}
	keys := make([]string, 0, len(unknown))
	for k := range unknown {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	buf.Write(base[:len(base)-1])
	for i, k := range keys {
		if i > 0 || len(base) > 2 {
			buf.WriteByte(',')
		}
		kb, err := marshalNoEscape(k)
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(bytes.TrimSpace(unknown[k]))
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// unknownKeys returns the keys of data not in known, or nil.
func unknownKeys(data []byte, known []string) (map[string]jsonRaw, error) {
	var obj map[string]jsonRaw
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	for _, k := range known {
		delete(obj, k)
	}
	if len(obj) == 0 {
		return nil, nil
	}
	return obj, nil
}

// MarshalJSON writes known keys then unknown keys sorted, without HTML escaping.
func (c NodeContract) MarshalJSON() ([]byte, error) {
	type plain NodeContract
	return marshalWithUnknown(plain(c), c.Unknown)
}

// UnmarshalJSON reads known keys and keeps the rest in Unknown.
func (c *NodeContract) UnmarshalJSON(data []byte) error {
	type plain NodeContract
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	u, err := unknownKeys(data, nodeContractKeys)
	if err != nil {
		return err
	}
	p.Unknown = u
	*c = NodeContract(p)
	return nil
}

func (c EdgeContract) MarshalJSON() ([]byte, error) {
	type plain EdgeContract
	return marshalWithUnknown(plain(c), c.Unknown)
}

func (c *EdgeContract) UnmarshalJSON(data []byte) error {
	type plain EdgeContract
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	u, err := unknownKeys(data, edgeContractKeys)
	if err != nil {
		return err
	}
	p.Unknown = u
	*c = EdgeContract(p)
	return nil
}

func (c Constraint) MarshalJSON() ([]byte, error) {
	type plain Constraint
	return marshalWithUnknown(plain(c), c.Unknown)
}

func (c *Constraint) UnmarshalJSON(data []byte) error {
	type plain Constraint
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	u, err := unknownKeys(data, constraintKeys)
	if err != nil {
		return err
	}
	p.Unknown = u
	*c = Constraint(p)
	return nil
}
