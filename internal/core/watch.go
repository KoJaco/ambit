package core

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const watchDebounce = 150 * time.Millisecond

// Watch rebuilds the index through Open when a node file or index.json changes.
// Editor scratch files and atomic-write temps are ignored. The callback runs
// after each debounced reload, with a nil error on success.
func (idx *Index) Watch(ctx context.Context, onReload func(error)) error {
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
	go idx.watchLoop(ctx, watcher, onReload)
	return nil
}

func (idx *Index) watchLoop(ctx context.Context, watcher *fsnotify.Watcher, onReload func(error)) {
	defer watcher.Close()
	var timer *time.Timer
	var timerC <-chan time.Time
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
			if onReload != nil {
				onReload(err)
			}
		case ev, ok := <-watcher.Events:
			if !ok {
				return
			}
			if ev.Op == fsnotify.Chmod || !idx.relevantWatch(ev.Name) {
				continue
			}
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
			idx.mu.Lock()
			err := idx.reload()
			idx.mu.Unlock()
			if onReload != nil {
				onReload(err)
			}
		}
	}
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
