package slipbox

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	watchDebounce = 300 * time.Millisecond
	ownWriteGrace = 2 * time.Second
)

// Watch reflects external edits live. Events are debounced, the vault's own
// writes are ignored, and an empty read is never treated as a deletion.
func (v *Vault) Watch() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := w.Add(v.root); err != nil {
		w.Close()
		return err
	}
	v.watchStop = make(chan struct{})
	v.watchDone = make(chan struct{})
	go v.watchLoop(w)
	return nil
}

func (v *Vault) stopWatch() {
	if v.watchStop == nil {
		return
	}
	close(v.watchStop)
	<-v.watchDone
	v.watchStop = nil
}

func (v *Vault) watchLoop(w *fsnotify.Watcher) {
	defer close(v.watchDone)
	defer w.Close()
	pending := map[string]bool{}
	var timer *time.Timer
	var fire <-chan time.Time
	for {
		select {
		case <-v.watchStop:
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			name := filepath.Base(ev.Name)
			if !strings.HasSuffix(name, ".md") || strings.HasPrefix(name, ".") || name == indexFile {
				continue
			}
			pending[ev.Name] = true
			if timer == nil {
				timer = time.NewTimer(watchDebounce)
			} else {
				timer.Reset(watchDebounce)
			}
			fire = timer.C
		case <-fire:
			paths := make([]string, 0, len(pending))
			for p := range pending {
				paths = append(paths, p)
			}
			pending = map[string]bool{}
			fire = nil
			v.handleExternal(paths)
		case _, ok := <-w.Errors:
			if !ok {
				return
			}
		}
	}
}

// handleExternal reparses only the touched files: one changed file, one parse.
func (v *Vault) handleExternal(paths []string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	now := time.Now()
	for path, t := range v.recent { // forget own writes once they are old
		if now.Sub(t) > ownWriteGrace {
			delete(v.recent, path)
		}
	}
	var changed []ID
	var missing []string
	for _, p := range paths {
		if t, own := v.recent[p]; own && now.Sub(t) < ownWriteGrace {
			continue
		}
		delete(v.recent, p)
		rel, err := filepath.Rel(v.root, p)
		if err != nil || strings.Contains(rel, string(os.PathSeparator)) {
			continue
		}
		data, err := os.ReadFile(p)
		if errors.Is(err, os.ErrNotExist) {
			missing = append(missing, rel)
			continue
		}
		if err != nil {
			continue
		}
		if len(data) == 0 {
			if _, exists := v.byPath[rel]; exists {
				continue // a sync client mid-write; keep what we have
			}
		}
		n := ParseNote(rel, data)
		if n.Guest {
			n.ID = guestID(rel)
		}
		if info, err := os.Stat(p); err == nil {
			n.ModTime, n.Size = info.ModTime(), info.Size()
		}
		if old, ok := v.notes[n.ID]; ok && old.Path != rel {
			// Renamed outside the app with the ID intact: keep every placement.
			delete(v.byPath, old.Path)
		}
		v.add(n)
		changed = append(changed, n.ID)
	}
	for _, rel := range missing {
		id, ok := v.byPath[rel]
		if !ok {
			continue
		}
		if v.notes[id].Path != rel {
			continue // already re-pointed by a rename
		}
		v.remove(id)
		changed = append(changed, id)
	}
	if len(changed) == 0 {
		return
	}
	v.reindex()
	v.notify(changed...)
}
