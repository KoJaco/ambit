package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
)

// jsonRaw is a JSON value preserved byte-for-byte aside from surrounding whitespace.
type jsonRaw = json.RawMessage

func parseObject(data []byte) (map[string]jsonRaw, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, fmt.Errorf("empty JSON")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	var raw map[string]jsonRaw
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, fmt.Errorf("JSON null is not an object")
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return nil, fmt.Errorf("trailing JSON")
	} else if !errors.Is(err, io.EOF) {
		return nil, err
	}
	return raw, nil
}

func take(m map[string]jsonRaw, key string) (jsonRaw, bool) {
	v, ok := m[key]
	if ok {
		delete(m, key)
	}
	return v, ok
}

func decodeString(raw jsonRaw) (string, error) {
	if isNull(raw) {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	return s, nil
}

func decodeStringSlice(raw jsonRaw) ([]string, error) {
	if isNull(raw) {
		return nil, nil
	}
	var s []string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return s, nil
}

func decodeBool(raw jsonRaw) (bool, error) {
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return false, err
	}
	return b, nil
}

func decodeInt(raw jsonRaw) (int, error) {
	var n int
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, err
	}
	return n, nil
}

func isNull(raw jsonRaw) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func rawOf(v any) (jsonRaw, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// encodeObject writes a JSON object with known keys in order, then unknown keys sorted.
// An empty object is "{}\n".
func encodeObject(order []string, vals map[string]jsonRaw, unknown map[string]jsonRaw) ([]byte, error) {
	type pair struct {
		k string
		v jsonRaw
	}
	var pairs []pair
	seen := map[string]bool{}
	for _, k := range order {
		v, ok := vals[k]
		if !ok {
			continue
		}
		pairs = append(pairs, pair{k, v})
		seen[k] = true
	}
	var extra []string
	for k := range unknown {
		if seen[k] {
			continue
		}
		extra = append(extra, k)
	}
	sort.Strings(extra)
	for _, k := range extra {
		pairs = append(pairs, pair{k, unknown[k]})
	}
	if len(pairs) == 0 {
		return []byte("{}\n"), nil
	}
	var buf bytes.Buffer
	buf.WriteString("{\n")
	for i, p := range pairs {
		key, err := json.Marshal(p.k)
		if err != nil {
			return nil, err
		}
		buf.WriteString("    ")
		buf.Write(key)
		buf.WriteString(": ")
		buf.Write(bytes.TrimSpace(p.v))
		if i < len(pairs)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString("}\n")
	return buf.Bytes(), nil
}

func decodeNode(data []byte) (*Node, error) {
	obj, err := parseObject(data)
	if err != nil {
		return nil, err
	}
	n := &Node{}
	idRaw, ok := take(obj, "id")
	if !ok {
		return nil, fmt.Errorf("missing id")
	}
	id, err := decodeString(idRaw)
	if err != nil || id == "" {
		return nil, fmt.Errorf("id must be a non-empty string")
	}
	n.ID = NodeID(id)

	nameRaw, ok := take(obj, "name")
	if !ok {
		return nil, fmt.Errorf("missing name")
	}
	name, err := decodeString(nameRaw)
	if err != nil || name == "" {
		return nil, fmt.Errorf("name must be a non-empty string")
	}
	n.Name = name

	typeRaw, ok := take(obj, "type")
	if !ok {
		return nil, fmt.Errorf("missing type")
	}
	typ, err := decodeString(typeRaw)
	if err != nil || typ == "" {
		return nil, fmt.Errorf("type must be a non-empty string")
	}
	n.Type = typ

	statusRaw, ok := take(obj, "status")
	if !ok {
		return nil, fmt.Errorf("missing status")
	}
	status, err := decodeString(statusRaw)
	if err != nil || status == "" {
		return nil, fmt.Errorf("status must be a non-empty string")
	}
	n.Status = Status(status)

	if raw, ok := take(obj, "parent_id"); ok && !isNull(raw) {
		parent, err := decodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("parent_id: %w", err)
		}
		n.ParentID = NodeID(parent)
	}
	if raw, ok := take(obj, "implementation"); ok {
		impl, err := decodeStringSlice(raw)
		if err != nil {
			return nil, fmt.Errorf("implementation: %w", err)
		}
		n.Implementation = impl
	}
	if raw, ok := take(obj, "scope"); ok {
		scope, err := decodeStringSlice(raw)
		if err != nil {
			return nil, fmt.Errorf("scope: %w", err)
		}
		n.Scope = scope
	}
	if raw, ok := take(obj, "protected"); ok && !isNull(raw) {
		p, err := decodeBool(raw)
		if err != nil {
			return nil, fmt.Errorf("protected: %w", err)
		}
		n.Protected = p
	}
	if len(obj) > 0 {
		n.Unknown = obj
	}
	return n, nil
}

func encodeNode(n *Node) ([]byte, error) {
	vals := map[string]jsonRaw{}
	var err error
	put := func(k string, v any) error {
		raw, err := rawOf(v)
		if err != nil {
			return err
		}
		vals[k] = raw
		return nil
	}
	if err = put("id", string(n.ID)); err != nil {
		return nil, err
	}
	if err = put("name", n.Name); err != nil {
		return nil, err
	}
	if err = put("type", n.Type); err != nil {
		return nil, err
	}
	if n.ParentID != "" {
		if err = put("parent_id", string(n.ParentID)); err != nil {
			return nil, err
		}
	}
	if len(n.Implementation) > 0 {
		if err = put("implementation", n.Implementation); err != nil {
			return nil, err
		}
	}
	if len(n.Scope) > 0 {
		if err = put("scope", n.Scope); err != nil {
			return nil, err
		}
	}
	if n.Protected {
		if err = put("protected", true); err != nil {
			return nil, err
		}
	}
	if err = put("status", string(n.Status)); err != nil {
		return nil, err
	}
	order := []string{"id", "name", "type", "parent_id", "implementation", "scope", "protected", "status"}
	return encodeObject(order, vals, n.Unknown)
}

func decodeRelationship(data []byte) (Relationship, error) {
	obj, err := parseObject(data)
	if err != nil {
		return Relationship{}, err
	}
	rel := Relationship{}
	fromRaw, ok := take(obj, "from")
	if !ok {
		return Relationship{}, fmt.Errorf("relationship missing from")
	}
	from, err := decodeString(fromRaw)
	if err != nil || from == "" {
		return Relationship{}, fmt.Errorf("relationship from must be a non-empty string")
	}
	toRaw, ok := take(obj, "to")
	if !ok {
		return Relationship{}, fmt.Errorf("relationship missing to")
	}
	to, err := decodeString(toRaw)
	if err != nil || to == "" {
		return Relationship{}, fmt.Errorf("relationship to must be a non-empty string")
	}
	rel.From = NodeID(from)
	rel.To = NodeID(to)
	if raw, ok := take(obj, "label"); ok && !isNull(raw) {
		label, err := decodeString(raw)
		if err != nil {
			return Relationship{}, fmt.Errorf("relationship label: %w", err)
		}
		rel.Label = label
	}
	if raw, ok := take(obj, "kind"); ok && !isNull(raw) {
		kind, err := decodeString(raw)
		if err != nil {
			return Relationship{}, fmt.Errorf("relationship kind: %w", err)
		}
		rel.Kind = kind
	}
	if len(obj) > 0 {
		rel.Unknown = obj
	}
	return rel, nil
}

func encodeRelationship(rel Relationship) (jsonRaw, error) {
	vals := map[string]jsonRaw{}
	put := func(k, v string) error {
		raw, err := rawOf(v)
		if err != nil {
			return err
		}
		vals[k] = raw
		return nil
	}
	if err := put("from", string(rel.From)); err != nil {
		return nil, err
	}
	if err := put("to", string(rel.To)); err != nil {
		return nil, err
	}
	if rel.Label != "" {
		if err := put("label", rel.Label); err != nil {
			return nil, err
		}
	}
	if rel.Kind != "" {
		if err := put("kind", rel.Kind); err != nil {
			return nil, err
		}
	}
	return encodeCompact([]string{"from", "to", "label", "kind"}, vals, rel.Unknown)
}

func encodeCompact(order []string, vals map[string]jsonRaw, unknown map[string]jsonRaw) (jsonRaw, error) {
	type pair struct {
		k string
		v jsonRaw
	}
	var pairs []pair
	seen := map[string]bool{}
	for _, k := range order {
		v, ok := vals[k]
		if !ok {
			continue
		}
		pairs = append(pairs, pair{k, v})
		seen[k] = true
	}
	var extra []string
	for k := range unknown {
		if seen[k] {
			continue
		}
		extra = append(extra, k)
	}
	sort.Strings(extra)
	for _, k := range extra {
		pairs = append(pairs, pair{k, unknown[k]})
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, p := range pairs {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(p.k)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(bytes.TrimSpace(p.v))
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
