package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func (idx *Index) saveIndex() error {
	vals := map[string]jsonRaw{}
	ver, err := rawOf(idx.SchemaVersion)
	if err != nil {
		return err
	}
	vals["schema_version"] = ver
	ids := make([]string, len(idx.Order))
	for i, id := range idx.Order {
		ids[i] = string(id)
	}
	nodes, err := rawOf(ids)
	if err != nil {
		return err
	}
	vals["nodes"] = nodes
	relRaws := make([]jsonRaw, 0, len(idx.Relationships))
	for _, rel := range idx.Relationships {
		raw, err := encodeRelationship(rel)
		if err != nil {
			return err
		}
		relRaws = append(relRaws, raw)
	}
	relArray, err := json.Marshal(relRaws)
	if err != nil {
		return err
	}
	vals["relationships"] = relArray
	data, err := encodeObject([]string{"schema_version", "nodes", "relationships"}, vals, idx.indexUnknown)
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(idx.arch(), "index.json"), data)
}

func (idx *Index) writeNode(n *Node) error {
	data, err := encodeNode(n)
	if err != nil {
		return err
	}
	if err := writeAtomic(idx.nodeJSON(n.ID), data); err != nil {
		return err
	}
	if err := writeAtomic(idx.nodeMD(n.ID), []byte(n.Spec)); err != nil {
		os.Remove(idx.nodeJSON(n.ID))
		return err
	}
	return nil
}

func (idx *Index) removeNodeFiles(id NodeID) error {
	var errs []error
	if err := os.Remove(idx.nodeJSON(id)); err != nil && !os.IsNotExist(err) {
		errs = append(errs, err)
	}
	if err := os.Remove(idx.nodeMD(id)); err != nil && !os.IsNotExist(err) {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (idx *Index) saveLocal() error {
	path := filepath.Join(idx.arch(), "local.json")
	vals := map[string]jsonRaw{}
	if idx.Assignment != nil {
		raw, err := encodeAssignment(idx.Assignment)
		if err != nil {
			return err
		}
		vals["assignment"] = raw
	}
	if idx.Assignment == nil && len(idx.localUnknown) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := encodeObject([]string{"assignment"}, vals, idx.localUnknown)
	if err != nil {
		return err
	}
	return writeAtomic(path, data)
}

func encodeAssignment(as *Assignment) (jsonRaw, error) {
	id, err := rawOf(string(as.NodeID))
	if err != nil {
		return nil, err
	}
	at, err := rawOf(as.AssignedAt)
	if err != nil {
		return nil, err
	}
	vals := map[string]jsonRaw{
		"node_id":     id,
		"assigned_at": at,
	}
	return encodeCompact([]string{"node_id", "assigned_at"}, vals, as.Unknown)
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".ambit-write-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func writeNewIndex(path string) error {
	vals := map[string]jsonRaw{}
	ver, err := rawOf(schemaVersion)
	if err != nil {
		return err
	}
	nodes, err := rawOf([]string{})
	if err != nil {
		return err
	}
	rels, err := rawOf([]struct{}{})
	if err != nil {
		return err
	}
	vals["schema_version"] = ver
	vals["nodes"] = nodes
	vals["relationships"] = rels
	data, err := encodeObject([]string{"schema_version", "nodes", "relationships"}, vals, nil)
	if err != nil {
		return err
	}
	return writeAtomic(path, data)
}
