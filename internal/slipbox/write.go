package slipbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (v *Vault) abs(rel string) string { return filepath.Join(v.root, rel) }

// writeNote persists a note atomically and records the write so the watcher
// ignores the echo.
func (v *Vault) writeNote(n *Note) error {
	data, err := n.Marshal()
	if err != nil {
		return err
	}
	v.recent[v.abs(n.Path)] = time.Now()
	if err := WriteFileAtomic(v.root, v.abs(n.Path), data, 0o644); err != nil {
		return err
	}
	if info, err := os.Stat(v.abs(n.Path)); err == nil {
		n.ModTime, n.Size = info.ModTime(), info.Size()
	}
	return nil
}

// normaliseBody makes the in-memory body identical to what Marshal writes.
func normaliseBody(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return body
}

// NewNote creates a card. With a parent it is filed behind that card at once,
// the way a card written from another card already knows its place. With no
// parent it is unfiled: a fresh card on the desk, waiting to be filed.
func (v *Vault) NewNote(title, body string, parent ID) (*Note, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if parent != "" {
		p, ok := v.notes[parent]
		if !ok {
			return nil, ErrNotFound
		}
		if p.Guest || p.Damaged {
			return nil, ErrGuestNote
		}
		if _, filed := v.state.Tree[parent]; !filed {
			return nil, ErrParentUnfiled
		}
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Untitled"
	}
	id := NewID(time.Now())
	for {
		if _, taken := v.notes[id]; !taken {
			break
		}
		id = id.Next()
	}
	created, _ := id.Time()
	n := &Note{ID: id, Slug: Slugify(title), Title: title, Created: created, Body: normaliseBody(body), Extra: map[string]any{}}
	n.Path = Filename(id, title)
	n.Links = ExtractLinks(n.Body)
	if err := v.writeNote(n); err != nil {
		return nil, err
	}
	v.add(n)
	if parent != "" {
		siblings := append(cloneSlice(v.tree.children[parent]), id)
		v.renumber(parent, "", siblings)
	}
	v.reindex()
	v.markDirty()
	v.notify(id)
	return n, nil
}

// UpdateNote rewrites title and body. A title change renames the file, keeping
// the ID so every placement survives. If the file changed on disk since the
// engine read it, nothing is written and a ConflictError carries the disk
// version.
func (v *Vault) UpdateNote(id ID, title, body string) error {
	return v.update(id, title, body, time.Time{}, false)
}

// UpdateNoteSeen is UpdateNote for an editor that opened the note at a known
// version: seen is the ModTime it loaded. If the note has moved on since,
// whether through the watcher or on disk, the edit conflicts.
func (v *Vault) UpdateNoteSeen(id ID, title, body string, seen time.Time) error {
	return v.update(id, title, body, seen, false)
}

// Overwrite is UpdateNote without the conflict check, for when the user has
// seen both versions and chosen theirs.
func (v *Vault) Overwrite(id ID, title, body string) error {
	return v.update(id, title, body, time.Time{}, true)
}

func (v *Vault) update(id ID, title, body string, seen time.Time, force bool) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	n, ok := v.notes[id]
	if !ok {
		return ErrNotFound
	}
	if n.Guest {
		return ErrGuestNote
	}
	if n.Damaged {
		return ErrDamagedNote
	}
	if !force {
		if info, err := os.Stat(v.abs(n.Path)); err == nil {
			changedOnDisk := !info.ModTime().Equal(n.ModTime) || info.Size() != n.Size
			movedOn := !seen.IsZero() && !info.ModTime().Equal(seen)
			if changedOnDisk || movedOn {
				if data, rerr := os.ReadFile(v.abs(n.Path)); rerr == nil {
					disk := ParseNote(n.Path, data)
					disk.ID = n.ID
					disk.ModTime, disk.Size = info.ModTime(), info.Size()
					return &ConflictError{OnDisk: disk}
				}
			}
		}
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Untitled"
	}
	oldPath := n.Path
	n.Title = title
	n.Slug = Slugify(title)
	n.Body = normaliseBody(body)
	n.Links = ExtractLinks(n.Body)
	n.Path = Filename(id, title)
	if err := v.writeNote(n); err != nil {
		n.Path = oldPath
		return err
	}
	if n.Path != oldPath {
		v.recent[v.abs(oldPath)] = time.Now()
		os.Remove(v.abs(oldPath))
		delete(v.byPath, oldPath)
		v.byPath[n.Path] = id
	}
	v.reindex()
	v.notify(id)
	return nil
}

// Adopt gives a guest file an ID: it is renamed to ID--slug.md, keeping its
// content and frontmatter untouched, and becomes an unfiled card on the
// desk. Links that reached it by slug or title keep resolving.
func (v *Vault) Adopt(id ID) (*Note, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	n, ok := v.notes[id]
	if !ok {
		return nil, ErrNotFound
	}
	if !n.Guest {
		return nil, errors.New("only a guest file can be adopted")
	}
	if n.Damaged {
		return nil, ErrDamagedNote
	}
	newID := NewID(time.Now())
	for {
		if _, taken := v.notes[newID]; !taken {
			break
		}
		newID = newID.Next()
	}
	oldPath := n.Path
	newPath := Filename(newID, strings.TrimSuffix(filepath.Base(oldPath), ".md"))
	if !insideRoot(v.root, v.abs(newPath)) {
		return nil, ErrOutsideVault
	}
	v.recent[v.abs(oldPath)] = time.Now()
	v.recent[v.abs(newPath)] = time.Now()
	if err := os.Rename(v.abs(oldPath), v.abs(newPath)); err != nil {
		return nil, fmt.Errorf("adopt: %w", err)
	}
	data, err := os.ReadFile(v.abs(newPath))
	if err != nil {
		return nil, err
	}
	v.remove(id)
	adopted := ParseNote(newPath, data)
	if info, err := os.Stat(v.abs(newPath)); err == nil {
		adopted.ModTime, adopted.Size = info.ModTime(), info.Size()
	}
	v.add(adopted)
	v.reindex()
	v.markDirty()
	v.notify(id, adopted.ID)
	return adopted, nil
}

// DeleteNote moves the file to the trash, drops its placements and register
// entries, and slides its children into the slot it occupied among its
// siblings so the sequence reads the same with one card gone.
func (v *Vault) DeleteNote(id ID) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	n, ok := v.notes[id]
	if !ok {
		return ErrNotFound
	}
	trash := filepath.Join(v.root, sidecarDir, trashDir)
	if err := os.MkdirAll(trash, 0o755); err != nil {
		return err
	}
	base := filepath.Base(n.Path)
	dest := filepath.Join(trash, base)
	for i := 1; ; i++ {
		if _, err := os.Stat(dest); errors.Is(err, os.ErrNotExist) {
			break
		}
		dest = filepath.Join(trash, fmt.Sprintf("%s-%d.md", strings.TrimSuffix(base, ".md"), i))
	}
	v.recent[v.abs(n.Path)] = time.Now()
	if err := os.Rename(v.abs(n.Path), dest); err != nil {
		return fmt.Errorf("trash note: %w", err)
	}

	if entry, filed := v.state.Tree[id]; filed {
		box := v.tree.boxOf[id]
		var order []ID
		found := false
		for _, s := range v.siblingsOf(entry.Parent, box) {
			if s == id {
				found = true
				order = append(order, v.tree.children[id]...)
			} else {
				order = append(order, s)
			}
		}
		if !found { // orphaned card: its children go at the end
			order = append(order, v.tree.children[id]...)
		}
		delete(v.state.Tree, id)
		v.renumber(entry.Parent, box, order)
	}
	v.state.Desk = dropPlacement(v.state.Desk, id)
	for i := range v.state.Boards {
		v.state.Boards[i].Cards = dropPlacement(v.state.Boards[i].Cards, id)
	}
	for i := range v.state.Register {
		e := &v.state.Register[i]
		if j := findID(e.Targets, id); j >= 0 {
			e.Targets = append(e.Targets[:j], e.Targets[j+1:]...)
		}
	}
	v.remove(id)
	v.reindex()
	v.markDirty()
	v.notify(id)
	return nil
}

// ---------------------------------------------------------------- filing

// siblingsOf lists the cards under parent in order, or the roots of box when
// parent is empty; caller holds the lock.
func (v *Vault) siblingsOf(parent ID, box string) []ID {
	if parent == "" {
		return v.tree.roots[box]
	}
	return v.tree.children[parent]
}

// renumber writes clean 1..n orders for one sibling list; caller holds the
// lock. Roots carry the box, children do not.
func (v *Vault) renumber(parent ID, box string, ids []ID) {
	for i, id := range ids {
		if parent == "" {
			v.state.Tree[id] = TreeEntry{Parent: "", Order: i + 1, Box: box}
		} else {
			v.state.Tree[id] = TreeEntry{Parent: parent, Order: i + 1}
		}
	}
}

// File puts a card in the box behind parent, placed after the sibling given,
// at the end if after is "", or first if after is First. Filing an already
// filed card moves it, and its whole branch with it.
func (v *Vault) File(id, parent, after ID) error {
	if parent == "" {
		return v.FileAsRoot(id, "", after)
	}
	return v.file(id, parent, "", after)
}

// FileAsRoot starts a new branch in a box ("" for the main box).
func (v *Vault) FileAsRoot(id ID, box string, after ID) error {
	return v.file(id, "", box, after)
}

func (v *Vault) file(id, parent ID, box string, after ID) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	n, ok := v.notes[id]
	if !ok {
		return ErrNotFound
	}
	if n.Guest || n.Damaged {
		return ErrGuestNote
	}
	if parent != "" {
		p, ok := v.notes[parent]
		if !ok {
			return ErrNotFound
		}
		if p.Guest || p.Damaged {
			return ErrGuestNote
		}
		if _, filed := v.state.Tree[parent]; !filed {
			return ErrParentUnfiled
		}
		if parent == id || v.descendants(id)[parent] {
			return ErrCycle
		}
		box = v.tree.boxOf[parent]
	} else {
		b, ok := v.box(box)
		if !ok {
			return ErrNoSuchBox
		}
		box = b.ID
	}

	// Remember the old sibling list so it can be renumbered without a gap.
	oldEntry, wasFiled := v.state.Tree[id]
	oldBox := v.tree.boxOf[id]
	var oldSiblings []ID
	moved := wasFiled && (oldEntry.Parent != parent || (parent == "" && oldBox != box))
	if moved {
		for _, s := range v.siblingsOf(oldEntry.Parent, oldBox) {
			if s != id {
				oldSiblings = append(oldSiblings, s)
			}
		}
	}
	var siblings []ID
	for _, s := range v.siblingsOf(parent, box) {
		if s != id {
			siblings = append(siblings, s)
		}
	}
	var ordered []ID
	switch after {
	case "":
		ordered = append(siblings, id)
	case First:
		ordered = append([]ID{id}, siblings...)
	default:
		found := false
		for _, s := range siblings {
			ordered = append(ordered, s)
			if s == after {
				ordered = append(ordered, id)
				found = true
			}
		}
		if !found {
			return ErrNotSibling
		}
	}
	v.renumber(parent, box, ordered)
	if moved {
		v.renumber(oldEntry.Parent, oldBox, oldSiblings)
	}
	v.state.Desk = dropPlacement(v.state.Desk, id)
	v.reindex()
	v.markDirty()
	v.notify(id)
	return nil
}
