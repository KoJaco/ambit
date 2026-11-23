package core

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

// SetConfigKeys sets top-level config.json keys. A nil value removes the key. All other
// keys, including ones this version does not interpret, are preserved. The file is
// rewritten atomically with keys sorted and values not HTML-escaped.
func (idx *Index) SetConfigKeys(kv map[string]any) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.setConfigKeysLocked(kv)
}

func (idx *Index) setConfigKeysLocked(kv map[string]any) error {
	obj := cloneRaw(idx.configUnknown)
	if obj == nil {
		obj = map[string]jsonRaw{}
	}
	for k, v := range kv {
		if v == nil {
			delete(obj, k)
			continue
		}
		raw, err := rawOf(v)
		if err != nil {
			return fmt.Errorf("config key %q: %w", k, err)
		}
		obj[k] = raw
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

// configString reads an optional string config value. A non-string value is ignored with
// a warning.
func (idx *Index) configString(obj map[string]jsonRaw, key string) string {
	raw, ok := obj[key]
	if !ok || isNull(raw) {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		idx.warn("config.json", fmt.Sprintf("%s must be a string; ignoring", key))
		return ""
	}
	return s
}
