package slipbox

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

func findID(ids []ID, id ID) int {
	for i, x := range ids {
		if x == id {
			return i
		}
	}
	return -1
}

func dropPlacement(ps []Placement, id ID) []Placement {
	out := ps[:0]
	for _, p := range ps {
		if p.ID != id {
			out = append(out, p)
		}
	}
	return out
}

func findPlacement(ps []Placement, id ID) int {
	for i, p := range ps {
		if p.ID == id {
			return i
		}
	}
	return -1
}

// placeable rejects guests and damaged notes; caller holds the lock.
func (v *Vault) placeable(id ID) error {
	n, ok := v.notes[id]
	if !ok {
		return ErrNotFound
	}
	if n.Guest || n.Damaged {
		return ErrGuestNote
	}
	return nil
}

// ---------------------------------------------------------------- desk

// Desk is the inbox: every unfiled card, with its position if one was
// stored, else -1,-1 for "not yet placed". Membership is derived; only
// positions are remembered.
func (v *Vault) Desk() []Placement {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := []Placement{}
	for _, n := range v.unfiled() {
		p := Placement{ID: n.ID, X: -1, Y: -1}
		if i := findPlacement(v.state.Desk, n.ID); i >= 0 {
			p = v.state.Desk[i]
		}
		out = append(out, p)
	}
	return out
}

// DeskMove remembers where an unfiled card lies on the desk.
func (v *Vault) DeskMove(id ID, x, y int) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.placeable(id); err != nil {
		return err
	}
	if _, filed := v.state.Tree[id]; filed {
		return ErrNotFound
	}
	if i := findPlacement(v.state.Desk, id); i >= 0 {
		v.state.Desk[i].X, v.state.Desk[i].Y = x, y
	} else {
		v.state.Desk = append(v.state.Desk, Placement{ID: id, X: x, Y: y})
	}
	v.markDirty()
	return nil
}

// ---------------------------------------------------------------- boards

// BoardsList returns the corkboards.
func (v *Vault) BoardsList() []Board {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make([]Board, len(v.state.Boards))
	for i, b := range v.state.Boards {
		out[i] = b
		out[i].Cards = cloneSlice(b.Cards)
	}
	return out
}

func (v *Vault) board(boardID string) (*Board, error) {
	for i := range v.state.Boards {
		if v.state.Boards[i].ID == boardID {
			return &v.state.Boards[i], nil
		}
	}
	return nil, ErrNotFound
}

// NewBoard creates a corkboard. IDs are b_ plus a slug, deduplicated.
func (v *Vault) NewBoard(name string) (Board, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Board"
	}
	base := "b_" + Slugify(name)
	id := base
	for i := 2; ; i++ {
		if _, err := v.board(id); err != nil {
			break
		}
		id = fmt.Sprintf("%s-%d", base, i)
	}
	b := Board{ID: id, Name: name, Created: time.Now().UTC(), Cards: []Placement{}}
	v.state.Boards = append(v.state.Boards, b)
	v.markDirty()
	v.notify()
	return b, nil
}

// RenameBoard changes a board's display name.
func (v *Vault) RenameBoard(boardID, name string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	b, err := v.board(boardID)
	if err != nil {
		return err
	}
	if name = strings.TrimSpace(name); name != "" {
		b.Name = name
	}
	v.markDirty()
	v.notify()
	return nil
}

// DeleteBoard removes a board. Its cards were only references; nothing is lost.
func (v *Vault) DeleteBoard(boardID string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	for i, b := range v.state.Boards {
		if b.ID == boardID {
			v.state.Boards = append(v.state.Boards[:i], v.state.Boards[i+1:]...)
			v.markDirty()
			v.notify()
			return nil
		}
	}
	return ErrNotFound
}

// BoardPin pins a card by reference; it stays in its drawer. Pinning again
// moves it.
func (v *Vault) BoardPin(boardID string, id ID, x, y int) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.placeable(id); err != nil {
		return err
	}
	b, err := v.board(boardID)
	if err != nil {
		return err
	}
	if i := findPlacement(b.Cards, id); i >= 0 {
		b.Cards[i].X, b.Cards[i].Y = x, y
	} else {
		b.Cards = append(b.Cards, Placement{ID: id, X: x, Y: y})
	}
	v.markDirty()
	v.notify(id)
	return nil
}

// BoardUnpin removes a card from a board.
func (v *Vault) BoardUnpin(boardID string, id ID) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	b, err := v.board(boardID)
	if err != nil {
		return err
	}
	if findPlacement(b.Cards, id) < 0 {
		return ErrNotFound
	}
	b.Cards = dropPlacement(b.Cards, id)
	v.markDirty()
	v.notify(id)
	return nil
}

// ---------------------------------------------------------------- register

// Register returns the user-curated index, terms sorted.
func (v *Vault) Register() []RegisterEntry {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := cloneSlice(v.state.Register)
	for i := range out {
		out[i].Targets = cloneSlice(out[i].Targets)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Term) < strings.ToLower(out[j].Term) })
	return out
}

// RegisterAdd points a term at a card, creating the term if needed. Choosing
// what earns an entry is the user's job; the engine never adds terms itself.
func (v *Vault) RegisterAdd(term string, id ID, note string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	term = strings.TrimSpace(term)
	if term == "" {
		return errors.New("a register term cannot be empty")
	}
	if id != "" {
		if err := v.placeable(id); err != nil {
			return err
		}
	}
	for i := range v.state.Register {
		e := &v.state.Register[i]
		if strings.EqualFold(e.Term, term) {
			if id != "" && findID(e.Targets, id) < 0 {
				e.Targets = append(e.Targets, id)
			}
			if note != "" {
				e.Note = note
			}
			v.structure++
			v.markDirty()
			v.notify(id)
			return nil
		}
	}
	entry := RegisterEntry{Term: term, Targets: []ID{}, Note: note}
	if id != "" {
		entry.Targets = []ID{id}
	}
	v.state.Register = append(v.state.Register, entry)
	v.structure++ // the contents file lists the register
	v.markDirty()
	v.notify(id)
	return nil
}

// RegisterRemove drops a card from a term; with an empty id the whole term goes.
func (v *Vault) RegisterRemove(term string, id ID) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	for i := range v.state.Register {
		e := &v.state.Register[i]
		if !strings.EqualFold(e.Term, term) {
			continue
		}
		if id == "" {
			v.state.Register = append(v.state.Register[:i], v.state.Register[i+1:]...)
		} else if j := findID(e.Targets, id); j >= 0 {
			e.Targets = append(e.Targets[:j], e.Targets[j+1:]...)
		} else {
			return ErrNotFound
		}
		v.structure++
		v.markDirty()
		v.notify(id)
		return nil
	}
	return ErrNotFound
}

// ---------------------------------------------------------------- prefs

// Prefs returns the UI's harmless traces.
func (v *Vault) Prefs() map[string]any {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make(map[string]any, len(v.state.Prefs))
	for k, val := range v.state.Prefs {
		out[k] = val
	}
	return out
}

// SetPrefs merges UI traces into the sidecar.
func (v *Vault) SetPrefs(p map[string]any) {
	v.mu.Lock()
	defer v.mu.Unlock()
	for k, val := range p {
		if val == nil {
			delete(v.state.Prefs, k)
		} else {
			v.state.Prefs[k] = val
		}
	}
	v.markDirty()
}

// ---------------------------------------------------------------- persistence

// markDirty schedules a debounced save; caller holds the lock.
func (v *Vault) markDirty() {
	v.dirty = true
	if v.saveTimer == nil {
		v.saveTimer = time.AfterFunc(500*time.Millisecond, func() { _ = v.Save() })
		return
	}
	v.saveTimer.Reset(500 * time.Millisecond)
}

// Save writes the sidecar now, and the contents file if the structure
// changed. It refuses while the vault looks incomplete, because rewriting
// placements after a sync blip is how layouts get lost.
func (v *Vault) Save() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.incomplete {
		return ErrVaultIncomplete
	}
	if !v.backedUp {
		if err := rotateBackups(v.root); err != nil {
			return fmt.Errorf("backup sidecar: %w", err)
		}
		v.backedUp = true
	}
	if err := saveState(v.root, v.state); err != nil {
		return err
	}
	if v.indexed != v.structure {
		if err := v.writeIndex(); err != nil {
			return err
		}
		v.indexed = v.structure
	}
	v.dirty = false
	return nil
}

// Close stops watching and flushes the sidecar.
func (v *Vault) Close() error {
	v.stopWatch()
	v.mu.Lock()
	if v.saveTimer != nil {
		v.saveTimer.Stop()
	}
	dirty := v.dirty
	v.mu.Unlock()
	if dirty {
		return v.Save()
	}
	return nil
}

// Subscribe receives change events. Call the returned func to stop.
func (v *Vault) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 32)
	v.mu.Lock()
	v.subs = append(v.subs, ch)
	v.mu.Unlock()
	return ch, func() {
		v.mu.Lock()
		defer v.mu.Unlock()
		for i, s := range v.subs {
			if s == ch {
				v.subs = append(v.subs[:i], v.subs[i+1:]...)
				break
			}
		}
	}
}

// notify fans out without blocking; a slow subscriber just misses an event
// and reloads on the next one. Caller holds the lock.
func (v *Vault) notify(ids ...ID) {
	clean := []ID{}
	for _, id := range ids {
		if id != "" {
			clean = append(clean, id)
		}
	}
	ev := Event{Kind: "changed", IDs: clean}
	for _, s := range v.subs {
		select {
		case s <- ev:
		default:
		}
	}
}
