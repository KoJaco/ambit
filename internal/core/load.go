package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Index is the in-memory model. Hierarchy comes from parent_id. Relationships
// are a separate list and are never stored as parent edges.
//
// Methods serialize with Watch. Do not read exported fields from another
// goroutine while Watch is running on this Index.
type Index struct {
	mu sync.Mutex

	Dir           string
	SchemaVersion int
	Nodes         map[NodeID]*Node
	Order         []NodeID
	Children      map[NodeID][]NodeID
	Roots         []NodeID
	Relationships []Relationship
	ActiveEdges   []Relationship
	Diagnostics   []Diagnostic
	Assignment    *Assignment

	indexUnknown      map[string]jsonRaw
	configUnknown     map[string]jsonRaw
	localUnknown      map[string]jsonRaw
	assignmentUnknown map[string]jsonRaw
}

// Open reads a .arch directory. Malformed JSON, an id that does not match its
// filename, or an invalid status fails the whole load. Integrity problems that
// the contract treats as warnings, and a cycle found on disk, are returned on
// the index.
func Open(dir string) (*Index, error) {
	dir = filepath.Clean(dir)
	arch := filepath.Join(dir, ".arch")
	st, err := os.Stat(arch)
	if err != nil || !st.IsDir() {
		return nil, fmt.Errorf("%s: .arch directory is missing", dir)
	}
	idx := &Index{
		Dir:      dir,
		Nodes:    map[NodeID]*Node{},
		Children: map[NodeID][]NodeID{},
	}
	if err := idx.readIndex(); err != nil {
		return nil, err
	}
	if err := idx.readConfig(); err != nil {
		return nil, err
	}
	if err := idx.readLocal(); err != nil {
		return nil, err
	}
	if err := idx.readMembers(); err != nil {
		return nil, err
	}
	if err := idx.scanOrphans(); err != nil {
		return nil, err
	}
	idx.linkHierarchy()
	idx.linkRelationships()
	idx.warnAssignment()
	return idx, nil
}

func (idx *Index) arch() string { return filepath.Join(idx.Dir, ".arch") }

func (idx *Index) nodeJSON(id NodeID) string {
	return filepath.Join(idx.arch(), "nodes", string(id)+".json")
}

func (idx *Index) nodeMD(id NodeID) string {
	return filepath.Join(idx.arch(), "nodes", string(id)+".md")
}

func (idx *Index) readIndex() error {
	path := filepath.Join(idx.arch(), "index.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	obj, err := parseObject(data)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	verRaw, ok := take(obj, "schema_version")
	if !ok {
		return fmt.Errorf("%s: missing schema_version", path)
	}
	ver, err := decodeInt(verRaw)
	if err != nil {
		return fmt.Errorf("%s: schema_version: %w", path, err)
	}
	if ver != schemaVersion {
		return fmt.Errorf("%s: schema_version %d is not supported", path, ver)
	}
	idx.SchemaVersion = ver

	nodesRaw, ok := take(obj, "nodes")
	if !ok {
		return fmt.Errorf("%s: missing nodes", path)
	}
	if !isNull(nodesRaw) {
		var ids []string
		if err := json.Unmarshal(nodesRaw, &ids); err != nil {
			return fmt.Errorf("%s: nodes: %w", path, err)
		}
		seen := map[NodeID]bool{}
		for _, id := range ids {
			nid := NodeID(id)
			if !slugPattern.MatchString(id) {
				return fmt.Errorf("%s: node id %q is not a valid slug", path, id)
			}
			if seen[nid] {
				return fmt.Errorf("%s: lists %s more than once", path, id)
			}
			seen[nid] = true
			idx.Order = append(idx.Order, nid)
		}
	}

	relRaw, ok := take(obj, "relationships")
	if !ok {
		return fmt.Errorf("%s: missing relationships", path)
	}
	if !isNull(relRaw) {
		var items []jsonRaw
		if err := json.Unmarshal(relRaw, &items); err != nil {
			return fmt.Errorf("%s: relationships: %w", path, err)
		}
		for i, item := range items {
			rel, err := decodeRelationship(item)
			if err != nil {
				return fmt.Errorf("%s: relationships[%d]: %w", path, i, err)
			}
			idx.Relationships = append(idx.Relationships, rel)
		}
	}
	if len(obj) > 0 {
		idx.indexUnknown = obj
	}
	return nil
}

func (idx *Index) readConfig() error {
	path := filepath.Join(idx.arch(), "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	obj, err := parseObject(data)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if len(obj) > 0 {
		idx.configUnknown = obj
	}
	return nil
}

func (idx *Index) readLocal() error {
	path := filepath.Join(idx.arch(), "local.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	obj, err := parseObject(data)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	raw, ok := take(obj, "assignment")
	if ok && !isNull(raw) {
		as, err := decodeAssignment(raw)
		if err != nil {
			return fmt.Errorf("%s: assignment: %w", path, err)
		}
		idx.Assignment = as
		idx.assignmentUnknown = as.Unknown
	}
	if len(obj) > 0 {
		idx.localUnknown = obj
	}
	return nil
}

func decodeAssignment(raw jsonRaw) (*Assignment, error) {
	obj, err := parseObject(raw)
	if err != nil {
		return nil, err
	}
	idRaw, ok := take(obj, "node_id")
	if !ok {
		return nil, fmt.Errorf("missing node_id")
	}
	id, err := decodeString(idRaw)
	if err != nil || id == "" {
		return nil, fmt.Errorf("node_id must be a non-empty string")
	}
	atRaw, ok := take(obj, "assigned_at")
	if !ok {
		return nil, fmt.Errorf("missing assigned_at")
	}
	at, err := decodeString(atRaw)
	if err != nil || at == "" {
		return nil, fmt.Errorf("assigned_at must be a non-empty string")
	}
	if _, err := time.Parse(time.RFC3339, at); err != nil {
		return nil, fmt.Errorf("assigned_at: %w", err)
	}
	as := &Assignment{NodeID: NodeID(id), AssignedAt: at}
	if len(obj) > 0 {
		as.Unknown = obj
	}
	return as, nil
}

func (idx *Index) readMembers() error {
	for _, id := range idx.Order {
		path := idx.nodeJSON(id)
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			idx.warn(path, fmt.Sprintf("index.json lists %s but %s is missing", id, filepath.Base(path)))
			continue
		}
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		node, err := decodeNode(data)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if node.ID != id {
			return fmt.Errorf("%s: id %q does not match filename", path, node.ID)
		}
		if !slugPattern.MatchString(string(node.ID)) {
			return fmt.Errorf("%s: id %q is not a valid slug", path, node.ID)
		}
		if !validStatus(node.Status) {
			return fmt.Errorf("%s: invalid status %q", path, node.Status)
		}
		mdPath := idx.nodeMD(id)
		md, err := os.ReadFile(mdPath)
		if os.IsNotExist(err) {
			idx.warn(mdPath, fmt.Sprintf("node %s is missing its markdown spec", id))
		} else if err != nil {
			return fmt.Errorf("%s: %w", mdPath, err)
		} else {
			node.Spec = string(md)
		}
		idx.Nodes[id] = node
	}
	return nil
}

func (idx *Index) scanOrphans() error {
	dir := filepath.Join(idx.arch(), "nodes")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("%s: %w", dir, err)
	}
	listed := map[NodeID]bool{}
	for _, id := range idx.Order {
		listed[id] = true
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(dir, name)
		switch {
		case strings.HasSuffix(name, ".json"):
			id := NodeID(strings.TrimSuffix(name, ".json"))
			if id == "" || listed[id] {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			if _, err := decodeNode(data); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			idx.warn(path, fmt.Sprintf("%s is not listed in index.json", name))
		case strings.HasSuffix(name, ".md"):
			id := NodeID(strings.TrimSuffix(name, ".md"))
			if id == "" {
				continue
			}
			if _, ok := idx.Nodes[id]; ok {
				continue
			}
			idx.warn(path, fmt.Sprintf("%s has no matching node json", name))
		}
	}
	return nil
}

func (idx *Index) linkHierarchy() {
	idx.Children = map[NodeID][]NodeID{}
	idx.Roots = nil
	dropped := map[NodeID]bool{}
	ids := make([]NodeID, 0, len(idx.Nodes))
	for id := range idx.Nodes {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		n := idx.Nodes[id]
		if n.ParentID == "" {
			continue
		}
		if _, ok := idx.Nodes[n.ParentID]; !ok {
			dropped[id] = true
			idx.warn(idx.nodeJSON(id), fmt.Sprintf("node %s parent_id %q does not exist", id, n.ParentID))
			continue
		}
		if parentEdgeInCycle(idx.Nodes, id) {
			dropped[id] = true
			idx.fail(idx.nodeJSON(id), fmt.Sprintf("hierarchy cycle: %s -> %s", id, n.ParentID))
		}
	}
	for _, id := range ids {
		n := idx.Nodes[id]
		if n.ParentID == "" || dropped[id] {
			idx.Roots = append(idx.Roots, id)
			continue
		}
		idx.Children[n.ParentID] = append(idx.Children[n.ParentID], id)
	}
	for parent, kids := range idx.Children {
		sort.Slice(kids, func(i, j int) bool { return kids[i] < kids[j] })
		idx.Children[parent] = kids
	}
}

func parentEdgeInCycle(nodes map[NodeID]*Node, id NodeID) bool {
	start := nodes[id]
	if start == nil || start.ParentID == "" {
		return false
	}
	if _, ok := nodes[start.ParentID]; !ok {
		return false
	}
	seen := map[NodeID]bool{id: true}
	cur := start.ParentID
	for {
		if cur == id {
			return true
		}
		if seen[cur] {
			return false
		}
		seen[cur] = true
		n := nodes[cur]
		if n == nil || n.ParentID == "" {
			return false
		}
		if _, ok := nodes[n.ParentID]; !ok {
			return false
		}
		cur = n.ParentID
	}
}

func (idx *Index) linkRelationships() {
	idx.ActiveEdges = nil
	path := filepath.Join(idx.arch(), "index.json")
	for _, rel := range idx.Relationships {
		_, fromOK := idx.Nodes[rel.From]
		_, toOK := idx.Nodes[rel.To]
		if !fromOK || !toOK {
			missing := rel.From
			if fromOK {
				missing = rel.To
			}
			idx.warn(path, fmt.Sprintf("relationship %s -> %s references missing node %s", rel.From, rel.To, missing))
			continue
		}
		if rel.Kind != "" && !validKind(rel.Kind) {
			idx.warn(path, fmt.Sprintf("relationship %s -> %s has unknown kind %q", rel.From, rel.To, rel.Kind))
		}
		idx.ActiveEdges = append(idx.ActiveEdges, rel)
	}
}

func (idx *Index) warnAssignment() {
	if idx.Assignment == nil {
		return
	}
	if _, ok := idx.Nodes[idx.Assignment.NodeID]; ok {
		return
	}
	idx.warn(filepath.Join(idx.arch(), "local.json"),
		fmt.Sprintf("assignment node_id %q does not exist", idx.Assignment.NodeID))
}

func (idx *Index) warn(path, msg string) {
	idx.Diagnostics = append(idx.Diagnostics, Diagnostic{Severity: SeverityWarning, Path: path, Message: msg})
}

func (idx *Index) fail(path, msg string) {
	idx.Diagnostics = append(idx.Diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: msg})
}

func (idx *Index) reload() error {
	next, err := Open(idx.Dir)
	if err != nil {
		return err
	}
	idx.SchemaVersion = next.SchemaVersion
	idx.Nodes = next.Nodes
	idx.Order = next.Order
	idx.Children = next.Children
	idx.Roots = next.Roots
	idx.Relationships = next.Relationships
	idx.ActiveEdges = next.ActiveEdges
	idx.Diagnostics = next.Diagnostics
	idx.Assignment = next.Assignment
	idx.indexUnknown = next.indexUnknown
	idx.configUnknown = next.configUnknown
	idx.localUnknown = next.localUnknown
	idx.assignmentUnknown = next.assignmentUnknown
	return nil
}
