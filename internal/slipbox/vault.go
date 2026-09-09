package slipbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Sentinel errors the UI must be able to distinguish.
var (
	ErrNotFound        = errors.New("note not found")
	ErrConflict        = errors.New("the note changed on disk since it was opened")
	ErrGuestNote       = errors.New("guest notes cannot be placed or edited")
	ErrDamagedNote     = errors.New("damaged notes are never rewritten")
	ErrVaultIncomplete = errors.New("vault looks incomplete; placements left untouched")
	ErrCycle           = errors.New("that would file a card behind its own descendant")
	ErrNotSibling      = errors.New("the card to file after is not under that parent")
	ErrParentUnfiled   = errors.New("that card is not in the box yet; file it first")
	ErrNoSuchBox       = errors.New("no such box")
)

// ConflictError carries the version found on disk so the caller can show
// both and let the user decide. errors.Is(err, ErrConflict) matches it.
type ConflictError struct{ OnDisk *Note }

func (e *ConflictError) Error() string        { return ErrConflict.Error() }
func (e *ConflictError) Is(target error) bool { return target == ErrConflict }

// Warning is something the vault noticed and kept going past.
type Warning struct {
	Kind    string `json:"kind"` // broken-link, ambiguous-link, orphan, cycle, damaged, sidecar, index
	ID      ID     `json:"id"`
	Message string `json:"message"`
}

// Event tells subscribers that notes changed, from any source.
type Event struct {
	Kind string `json:"kind"` // changed
	IDs  []ID   `json:"ids"`
}

// First is the position marker for "before every sibling" when filing.
const First = ID("^")

func cloneSlice[T any](s []T) []T {
	out := make([]T, len(s))
	copy(out, s)
	return out
}

// Vault is the engine's public surface. All state changes go through it, and
// every method is safe for concurrent use.
type Vault struct {
	root string

	mu     sync.RWMutex
	notes  map[ID]*Note
	byPath map[string]ID

	// derived by reindex
	bySlug    map[string][]ID
	byTitle   map[string][]ID
	byAddr    map[string]ID
	back      map[ID][]ID
	tree      derived
	drawers   []Drawer
	parseWarn []Warning // from load; survive reindex
	warnings  []Warning
	structure int // bumps on every structural change; _index.md tracks it
	indexed   int

	state      *State
	incomplete bool
	backedUp   bool // one sidecar backup per session, before the first save
	dirty      bool
	saveTimer  *time.Timer

	recent map[string]time.Time // own writes, so the watcher can ignore them
	subs   []chan Event

	watchStop chan struct{}
	watchDone chan struct{}
}

// Open scans a vault, creating the folder and sidecar dir if absent. It
// never writes on its own; anything it repairs is persisted on the next save.
func Open(root string) (*Vault, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, sidecarDir), 0o755); err != nil {
		return nil, fmt.Errorf("open vault: %w", err)
	}
	v := &Vault{root: root, notes: map[ID]*Note{}, byPath: map[string]ID{}, recent: map[string]time.Time{}}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("open vault: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == indexFile {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("open vault: %w", err)
		}
		n := ParseNote(e.Name(), data)
		if info, err := e.Info(); err == nil {
			n.ModTime, n.Size = info.ModTime(), info.Size()
		}
		v.add(n)
	}

	st, recovered, err := loadState(root)
	if err != nil {
		return nil, err
	}
	v.state = st
	if recovered != "" {
		v.parseWarn = append(v.parseWarn, Warning{Kind: "sidecar", Message: recovered})
	}
	changed, incomplete := cleanup(st, v.notes)
	v.incomplete = incomplete
	if !incomplete && migrate(st, v.notes) {
		changed = true
	}
	if !incomplete && len(st.Tree) == 0 && v.hasOwnNotes() {
		// No structure at all but cards exist: the sidecar was lost. The
		// contents file can rebuild the tree.
		if tree, warn := recoverTree(root, st.Settings.Boxes); len(tree) > 0 {
			st.Tree = tree
			v.parseWarn = append(v.parseWarn, Warning{Kind: "index", Message: warn})
			changed = true
		}
	}
	if !incomplete && pruneDesk(st) {
		changed = true
	}
	v.reindex()
	v.indexed = v.structure // nothing to rewrite until something changes
	if changed {
		v.dirty = true
	}
	return v, nil
}

// Create makes a new vault at an empty or absent directory.
func Create(root string) (*Vault, error) {
	if entries, err := os.ReadDir(root); err == nil && len(entries) > 0 {
		return nil, fmt.Errorf("create vault: %s is not empty", root)
	}
	v, err := Open(root)
	if err != nil {
		return nil, err
	}
	return v, v.Save()
}

// Root is the absolute vault path.
func (v *Vault) Root() string { return v.root }

func (v *Vault) hasOwnNotes() bool {
	for _, n := range v.notes {
		if !n.Guest && !n.Damaged {
			return true
		}
	}
	return false
}

// guestID gives guest files a stable, obviously-not-an-ID key.
func guestID(path string) ID { return ID("guest-" + Slugify(strings.TrimSuffix(path, ".md"))) }

// add registers a parsed note; the caller holds the lock or is single-threaded.
func (v *Vault) add(n *Note) {
	if n.Path == indexFile {
		return // the contents file is ours, never a card
	}
	if n.Guest {
		n.ID = guestID(n.Path)
	}
	v.notes[n.ID] = n
	v.byPath[n.Path] = n.ID
}

func (v *Vault) remove(id ID) {
	if n, ok := v.notes[id]; ok {
		delete(v.byPath, n.Path)
		delete(v.notes, id)
	}
}

// reindex rebuilds everything derived: addresses, link resolution,
// backlinks, drawers and warnings. It is cheap enough to run after every
// change (about 150ms for 5,000 notes, most of it parsing).
func (v *Vault) reindex() {
	v.structure++
	v.bySlug = map[string][]ID{}
	v.byTitle = map[string][]ID{}
	v.back = map[ID][]ID{}
	v.warnings = cloneSlice(v.parseWarn)
	for id, n := range v.notes {
		v.bySlug[strings.ToLower(n.Slug)] = append(v.bySlug[strings.ToLower(n.Slug)], id)
		v.byTitle[strings.ToLower(n.Title)] = append(v.byTitle[strings.ToLower(n.Title)], id)
		if n.Damaged {
			v.warnings = append(v.warnings, Warning{Kind: "damaged", ID: id, Message: n.Path + " has unreadable frontmatter and is shown as-is"})
		}
	}
	for _, index := range []map[string][]ID{v.bySlug, v.byTitle} {
		for _, ids := range index {
			sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		}
	}

	v.tree = deriveAddresses(v.notes, v.state.Tree, v.state.Settings.Boxes)
	v.byAddr = map[string]ID{}
	for id, a := range v.tree.addr {
		v.byAddr[strings.ToLower(a)] = id
	}

	ids := make([]ID, 0, len(v.notes))
	for id := range v.notes {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		n := v.notes[id]
		for i := range n.Links {
			n.Links[i].Resolved = v.resolve(id, n.Links[i].Target)
			if r := n.Links[i].Resolved; r != "" && r != id {
				v.back[r] = append(v.back[r], id)
			}
		}
	}
	v.warnings = append(v.warnings, v.tree.warnings...)
	v.drawers = deriveDrawers(v.notes, v.tree, v.state.Settings.Boxes, v.state.Settings.DrawerRule, v.state.Settings.DrawerSize)
}

// resolve implements the resolution order: ID, slug, title, address.
// Ambiguity picks the oldest and records a warning.
func (v *Vault) resolve(from ID, target string) ID {
	if ID(target).Valid() {
		if _, ok := v.notes[ID(target)]; ok {
			return ID(target)
		}
	}
	key := strings.ToLower(target)
	for _, index := range []map[string][]ID{v.bySlug, v.byTitle} {
		if ids := index[key]; len(ids) > 0 {
			if len(ids) > 1 {
				v.warnings = append(v.warnings, Warning{Kind: "ambiguous-link", ID: from, Message: fmt.Sprintf("[[%s]] matches %d notes; using the oldest", target, len(ids))})
			}
			return ids[0]
		}
	}
	// Luhmann linked by address. Accept it for hand-typed links, knowing an
	// address can change when a card is refiled; the editor writes IDs.
	if id, ok := v.byAddr[key]; ok {
		return id
	}
	v.warnings = append(v.warnings, Warning{Kind: "broken-link", ID: from, Message: fmt.Sprintf("[[%s]] does not resolve", target)})
	return ""
}

// ---------------------------------------------------------------- reading

// Notes returns every note, sorted by ID (guests last, by path).
func (v *Vault) Notes() []*Note {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make([]*Note, 0, len(v.notes))
	for _, n := range v.notes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Guest != out[j].Guest {
			return !out[i].Guest
		}
		if out[i].Guest {
			return out[i].Path < out[j].Path
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Note looks one up.
func (v *Vault) Note(id ID) (*Note, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	n, ok := v.notes[id]
	return n, ok
}

// Warnings lists everything noticed during the last index.
func (v *Vault) Warnings() []Warning {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return cloneSlice(v.warnings)
}

// Incomplete is true when the sidecar referenced many notes that are missing,
// which looks like a half-synced folder rather than deletions.
func (v *Vault) Incomplete() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.incomplete
}

// Settings returns the derivation settings.
func (v *Vault) Settings() Settings {
	v.mu.RLock()
	defer v.mu.RUnlock()
	s := v.state.Settings
	s.Boxes = cloneSlice(s.Boxes)
	return s
}

// SetSettings updates derivation settings and re-derives.
func (v *Vault) SetSettings(s Settings) {
	v.mu.Lock()
	defer v.mu.Unlock()
	s.normalise()
	v.state.Settings = s
	v.reindex()
	v.markDirty()
}

// Boxes lists the slip boxes, main first.
func (v *Vault) Boxes() []Box {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return cloneSlice(v.state.Settings.Boxes)
}

// BoxOf returns the box a filed card is in, "" for unfiled cards.
func (v *Vault) BoxOf(id ID) string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.tree.boxOf[id]
}

func (v *Vault) box(id string) (Box, bool) {
	if id == "" {
		return v.state.Settings.Boxes[0], true
	}
	for _, b := range v.state.Settings.Boxes {
		if b.ID == id {
			return b, true
		}
	}
	return Box{}, false
}

// Address returns the derived address, "" for guests and unfiled cards.
func (v *Vault) Address(id ID) string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.tree.addr[id]
}

func (v *Vault) lookup(ids []ID) []*Note {
	out := make([]*Note, 0, len(ids))
	for _, id := range ids {
		if n, ok := v.notes[id]; ok {
			out = append(out, n)
		}
	}
	return out
}

// Children are the cards filed behind this one, in order.
func (v *Vault) Children(id ID) []*Note {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.lookup(v.tree.children[id])
}

// Parent is the card this one was filed behind.
func (v *Vault) Parent(id ID) (*Note, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	entry, ok := v.state.Tree[id]
	if !ok {
		return nil, false
	}
	p, ok := v.notes[entry.Parent]
	return p, ok
}

// Roots are the cards that start a branch in a box, in address order. An
// empty box means the main box.
func (v *Vault) Roots(box string) []*Note {
	v.mu.RLock()
	defer v.mu.RUnlock()
	b, ok := v.box(box)
	if !ok {
		return nil
	}
	return v.lookup(v.tree.roots[b.ID])
}

// Filed reports whether a card has a place in a box.
func (v *Vault) Filed(id ID) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	_, ok := v.state.Tree[id]
	return ok
}

// Unfiled are cards not yet in a box: written, but with no address. They
// lie on the desk until they are filed.
func (v *Vault) Unfiled() []*Note {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.unfiled()
}

func (v *Vault) unfiled() []*Note {
	out := []*Note{}
	for id, n := range v.notes {
		if n.Guest || n.Damaged {
			continue
		}
		if _, ok := v.state.Tree[id]; !ok {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Backlinks are notes whose body links to this one.
func (v *Vault) Backlinks(id ID) []*Note {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.lookup(v.back[id])
}

// descendants collects every note below id; caller holds the lock.
func (v *Vault) descendants(id ID) map[ID]bool {
	seen := map[ID]bool{}
	stack := cloneSlice(v.tree.children[id])
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[cur] {
			continue
		}
		seen[cur] = true
		stack = append(stack, v.tree.children[cur]...)
	}
	return seen
}

// CrossRefs are notes linked to or from this one that are not descendants:
// "mentions this", as opposed to "grew from this".
func (v *Vault) CrossRefs(id ID) []*Note {
	v.mu.RLock()
	defer v.mu.RUnlock()
	n, ok := v.notes[id]
	if !ok {
		return nil
	}
	below := v.descendants(id)
	seen := map[ID]bool{id: true}
	var ids []ID
	for _, l := range n.Links {
		if l.Resolved != "" && !seen[l.Resolved] && !below[l.Resolved] {
			seen[l.Resolved] = true
			ids = append(ids, l.Resolved)
		}
	}
	for _, b := range v.back[id] {
		if !seen[b] && !below[b] {
			seen[b] = true
			ids = append(ids, b)
		}
	}
	return v.lookup(ids)
}

// Drawers returns every drawer of every box, main box first.
func (v *Vault) Drawers() []Drawer {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return cloneSlice(v.drawers)
}

// Stage is how big the main box has grown, from its drawer count: box (on
// the desk), double, cabinet, wall. The room must have room for every drawer.
func (v *Vault) Stage() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	n := 0
	main := v.state.Settings.Boxes[0].ID
	for _, d := range v.drawers {
		if d.Box == main && !d.Unsorted {
			n++
		}
	}
	switch {
	case n <= 1:
		return "box"
	case n <= 3:
		return "double"
	case n <= 9:
		return "cabinet"
	default:
		return "wall"
	}
}

// Rediscover picks one old card a day that is filed and not on a board, so
// the box occasionally talks back. Deterministic for the day; "" if too few.
func (v *Vault) Rediscover(now time.Time) ID {
	v.mu.RLock()
	defer v.mu.RUnlock()
	pinned := map[ID]bool{}
	for _, b := range v.state.Boards {
		for _, p := range b.Cards {
			pinned[p.ID] = true
		}
	}
	var pool []ID
	for id := range v.tree.addr {
		if !pinned[id] {
			pool = append(pool, id)
		}
	}
	if len(pool) < 3 {
		return ""
	}
	sort.Slice(pool, func(i, j int) bool { return pool[i] < pool[j] })
	day := now.UTC().Year()*366 + now.UTC().YearDay()
	// A small multiplier so consecutive days do not walk the list in order.
	return pool[(day*7)%len(pool)]
}
