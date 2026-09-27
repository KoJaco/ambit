package core

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const watchDebounce = 150 * time.Millisecond

// WatchNotice is one debounced reload. ModelChanged means a node file or
// index.json changed. NodeIDs are the nodes those paths touched. IntegrityChanged
// is true only when the warning set changed. Proposal events are not represented.
type WatchNotice struct {
	Err              error
	ModelChanged     bool
	NodeIDs          []NodeID
	IntegrityChanged bool
}

// Watch rebuilds the index through Open when a node file or index.json changes.
// Editor scratch files and atomic-write temps are ignored. The callback runs
// after each debounced reload, with a nil error on success.
func (idx *Index) Watch(ctx context.Context, onReload func(error)) error {
	return idx.WatchNotices(ctx, func(n WatchNotice) {
		if onReload != nil {
			onReload(n.Err)
		}
	})
}

// WatchNotices is Watch with the node ids and whether the warning set changed.
func (idx *Index) WatchNotices(ctx context.Context, onNotice func(WatchNotice)) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	arch := idx.arch()
	nodes := filepath.Join(arch, "nodes")
	if err := watcher.Add(nodes); err != nil {
		watcher.Close()
		return err
	}
	// Watch the directory rather than index.json itself. saveIndex replaces the
	// file by rename, which drops a watch on the inode.
	if err := watcher.Add(arch); err != nil {
		watcher.Close()
		return err
	}
	go idx.watchLoop(ctx, watcher, onNotice)
	return nil
}

func (idx *Index) watchLoop(ctx context.Context, watcher *fsnotify.Watcher, onNotice func(WatchNotice)) {
	defer watcher.Close()
	var timer *time.Timer
	var timerC <-chan time.Time
	pending := map[string]struct{}{}
	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			if onNotice != nil {
				onNotice(WatchNotice{Err: err})
			}
		case ev, ok := <-watcher.Events:
			if !ok {
				return
			}
			if ev.Op == fsnotify.Chmod || !idx.relevantWatch(ev.Name) {
				continue
			}
			pending[ev.Name] = struct{}{}
			if timer == nil {
				timer = time.NewTimer(watchDebounce)
				timerC = timer.C
				continue
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(watchDebounce)
		case <-timerC:
			paths := pending
			pending = map[string]struct{}{}
			notice := idx.reloadWatched(paths)
			if onNotice != nil {
				onNotice(notice)
			}
		}
	}
}

func (idx *Index) reloadWatched(paths map[string]struct{}) WatchNotice {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	beforeWarn := warningKey(idx.Diagnostics)
	beforeOrder := append([]NodeID{}, idx.Order...)
	beforeRels := append([]Relationship{}, idx.Relationships...)
	err := idx.reload()
	if err != nil {
		return WatchNotice{Err: err}
	}
	ids := map[NodeID]struct{}{}
	indexChanged := false
	for path := range paths {
		base := filepath.Base(path)
		if base == "index.json" {
			indexChanged = true
			continue
		}
		name := strings.TrimSuffix(base, filepath.Ext(base))
		if name != "" {
			ids[NodeID(name)] = struct{}{}
		}
	}
	if indexChanged {
		for _, id := range affectedByIndex(beforeOrder, idx.Order, beforeRels, idx.Relationships) {
			ids[id] = struct{}{}
		}
	}
	return WatchNotice{
		ModelChanged:     true,
		NodeIDs:          sortedIDs(ids),
		IntegrityChanged: warningKey(idx.Diagnostics) != beforeWarn,
	}
}

func affectedByIndex(beforeOrder, afterOrder []NodeID, beforeRels, afterRels []Relationship) []NodeID {
	ids := map[NodeID]struct{}{}
	before := map[NodeID]struct{}{}
	for _, id := range beforeOrder {
		before[id] = struct{}{}
	}
	after := map[NodeID]struct{}{}
	for _, id := range afterOrder {
		after[id] = struct{}{}
		if _, ok := before[id]; !ok {
			ids[id] = struct{}{}
		}
	}
	for id := range before {
		if _, ok := after[id]; !ok {
			ids[id] = struct{}{}
		}
	}
	beforeRel := map[string]Relationship{}
	for _, rel := range beforeRels {
		beforeRel[relKey(rel)] = rel
	}
	afterRel := map[string]Relationship{}
	for _, rel := range afterRels {
		afterRel[relKey(rel)] = rel
		if _, ok := beforeRel[relKey(rel)]; !ok {
			ids[rel.From] = struct{}{}
			ids[rel.To] = struct{}{}
		}
	}
	for key, rel := range beforeRel {
		if _, ok := afterRel[key]; !ok {
			ids[rel.From] = struct{}{}
			ids[rel.To] = struct{}{}
		}
	}
	return sortedIDs(ids)
}

func relKey(rel Relationship) string {
	return string(rel.From) + "\t" + string(rel.To) + "\t" + rel.Label + "\t" + rel.Kind
}

func warningKey(ds []Diagnostic) string {
	parts := make([]string, len(ds))
	for i, d := range ds {
		parts[i] = d.String()
	}
	sort.Strings(parts)
	return strings.Join(parts, "\n")
}

func sortedIDs(set map[NodeID]struct{}) []NodeID {
	out := make([]NodeID, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (idx *Index) relevantWatch(path string) bool {
	path = filepath.Clean(path)
	base := filepath.Base(path)
	if isScratch(base) {
		return false
	}
	arch := filepath.Clean(idx.arch())
	if base == "index.json" && filepath.Clean(filepath.Dir(path)) == arch {
		return true
	}
	nodes := filepath.Join(arch, "nodes")
	if filepath.Clean(filepath.Dir(path)) == nodes && (strings.HasSuffix(base, ".json") || strings.HasSuffix(base, ".md")) {
		return true
	}
	return false
}

func isScratch(base string) bool {
	if strings.HasPrefix(base, ".") {
		return true
	}
	if strings.HasSuffix(base, "~") || strings.HasSuffix(base, ".swp") || strings.HasSuffix(base, ".swx") || strings.HasSuffix(base, ".tmp") {
		return true
	}
	return false
}
