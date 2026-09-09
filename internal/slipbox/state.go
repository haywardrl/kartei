package slipbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Placement is a card on the desk or a board, with a position the room uses.
type Placement struct {
	ID ID  `json:"id"`
	X  int `json:"x"`
	Y  int `json:"y"`
}

// Board is a corkboard: a named working set. Membership is a reference.
type Board struct {
	ID      string      `json:"id"`
	Name    string      `json:"name"`
	Created time.Time   `json:"created"`
	Cards   []Placement `json:"cards"`
}

// RegisterEntry is one line of the user-curated index.
type RegisterEntry struct {
	Term    string `json:"term"`
	Targets []ID   `json:"targets"`
	Note    string `json:"note"`
}

// Box is one slip box. The first box in Settings.Boxes is the main one;
// Prefix is prepended to its addresses so the boxes never collide.
type Box struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
}

// DefaultBoxes returns the two boxes Luhmann kept: notes and literature.
func DefaultBoxes() []Box {
	return []Box{{ID: "main", Name: "Notes", Prefix: ""}, {ID: "lit", Name: "Literature", Prefix: "L"}}
}

// Settings controls derivation rules.
type Settings struct {
	DrawerRule   string `json:"drawerRule"`
	DrawerSize   int    `json:"drawerSize"`
	DeskCapacity int    `json:"deskCapacity"`
	Boxes        []Box  `json:"boxes"`
}

// normalise fills defaults so no caller has to check for zero values.
func (s *Settings) normalise() {
	if s.DrawerRule != RuleRange {
		s.DrawerRule = RuleBranch
	}
	if s.DrawerSize < 1 {
		s.DrawerSize = 60
	}
	if s.DeskCapacity < 1 {
		s.DeskCapacity = 12
	}
	if len(s.Boxes) == 0 {
		s.Boxes = DefaultBoxes()
	}
	for i := range s.Boxes {
		if s.Boxes[i].ID == "" {
			s.Boxes[i].ID = Slugify(s.Boxes[i].Name)
		}
	}
}

// State is the sidecar: every placement and structural decision, nothing that
// belongs in a note. Only the exception is stored; the default is derived.
type State struct {
	Version  int              `json:"version"`
	Desk     []Placement      `json:"desk"`
	Boards   []Board          `json:"boards"`
	Tree     map[ID]TreeEntry `json:"tree"`
	Register []RegisterEntry  `json:"register"`
	Settings Settings         `json:"settings"`
	// Prefs holds harmless UI traces (camera, last drawer, lamp) so the room
	// remembers how it was left. The engine never reads them.
	Prefs map[string]any `json:"prefs,omitempty"`
}

const (
	sidecarDir  = ".kartei"
	sidecarFile = "state.json"
	trashDir    = "trash"
	backupDir   = "backups"
	keepBackups = 5
)

// stateVersion 2 made filing explicit: every card in the box has a tree
// entry (roots with an empty parent). Version 1 treated "no entry" as root.
const stateVersion = 2

func defaultState() *State {
	return &State{
		Version:  stateVersion,
		Desk:     []Placement{},
		Boards:   []Board{},
		Tree:     map[ID]TreeEntry{},
		Register: []RegisterEntry{},
		Settings: Settings{DrawerRule: RuleBranch, DrawerSize: 60, DeskCapacity: 12, Boxes: DefaultBoxes()},
		Prefs:    map[string]any{},
	}
}

func sidecarPath(root string) string { return filepath.Join(root, sidecarDir, sidecarFile) }

func parseState(data []byte) (*State, error) {
	st := defaultState()
	if err := json.Unmarshal(data, st); err != nil {
		return nil, err
	}
	return st, nil
}

func backupPath(root string, n int) string {
	return filepath.Join(root, sidecarDir, backupDir, fmt.Sprintf("state-%d.json", n))
}

// rotateBackups keeps the last few sidecars: state-1 is the newest.
func rotateBackups(root string) error {
	cur, err := os.ReadFile(sidecarPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, sidecarDir, backupDir), 0o755); err != nil {
		return err
	}
	for n := keepBackups - 1; n >= 1; n-- {
		os.Rename(backupPath(root, n), backupPath(root, n+1))
	}
	return WriteFileAtomic(root, backupPath(root, 1), cur, 0o644)
}

// loadState reads the sidecar, returning defaults if it is absent. A sidecar
// that will not parse is set aside, never overwritten, and the newest backup
// that parses takes its place; recovered says what happened.
func loadState(root string) (st *State, recovered string, err error) {
	data, err := os.ReadFile(sidecarPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return defaultState(), "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("read sidecar: %w", err)
	}
	st, perr := parseState(data)
	if perr != nil {
		aside := sidecarPath(root) + ".broken-" + time.Now().UTC().Format("20060102T150405")
		if err := os.Rename(sidecarPath(root), aside); err != nil {
			return nil, "", fmt.Errorf("sidecar unreadable and could not be set aside: %w", err)
		}
		recovered = fmt.Sprintf("state.json could not be read (%v); it was kept as %s", perr, filepath.Base(aside))
		st = nil
		for n := 1; n <= keepBackups; n++ {
			b, err := os.ReadFile(backupPath(root, n))
			if err != nil {
				continue
			}
			if cand, err := parseState(b); err == nil {
				st = cand
				recovered += fmt.Sprintf(" and the layout was restored from backup %d", n)
				break
			}
		}
		if st == nil {
			st = defaultState()
			recovered += "; no usable backup, so placements start empty"
		}
	}
	if st.Tree == nil {
		st.Tree = map[ID]TreeEntry{}
	}
	if st.Desk == nil {
		st.Desk = []Placement{}
	}
	if st.Boards == nil {
		st.Boards = []Board{}
	}
	if st.Register == nil {
		st.Register = []RegisterEntry{}
	}
	if st.Prefs == nil {
		st.Prefs = map[string]any{}
	}
	st.Settings.normalise()
	return st, recovered, nil
}

func saveState(root string, st *State) error {
	if err := os.MkdirAll(filepath.Join(root, sidecarDir), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(root, sidecarPath(root), append(data, '\n'), 0o644)
}

// pruneDesk drops desk positions for cards that are in the box; the desk is
// derived from what is unfiled, positions are the only thing stored.
func pruneDesk(st *State) bool {
	kept := st.Desk[:0]
	changed := false
	for _, p := range st.Desk {
		if _, filed := st.Tree[p.ID]; filed {
			changed = true
			continue
		}
		kept = append(kept, p)
	}
	st.Desk = kept
	return changed
}

// migrate brings an older sidecar up to date. Version 1 had no unfiled state,
// so every untracked card was a root; give those cards explicit root entries
// so they stay where they were. Returns true if anything changed.
func migrate(st *State, notes map[ID]*Note) bool {
	if st.Version >= stateVersion {
		return false
	}
	var untracked []ID
	for id, n := range notes {
		if n.Guest || n.Damaged {
			continue
		}
		if _, ok := st.Tree[id]; !ok {
			untracked = append(untracked, id)
		}
	}
	sort.Slice(untracked, func(i, j int) bool { return untracked[i] < untracked[j] })
	next := 0
	for _, e := range st.Tree {
		if e.Parent == "" && e.Order > next {
			next = e.Order
		}
	}
	for _, id := range untracked {
		next++
		st.Tree[id] = TreeEntry{Parent: "", Order: next}
	}
	// Version 1 knew only one drawer rule, so it was never a choice.
	st.Settings.DrawerRule = RuleBranch
	st.Version = stateVersion
	return true
}

// cleanup drops placements and tree entries that no longer resolve to a note.
// If a large share fail at once the vault looks half-synced, so nothing is
// touched and the caller is told to warn instead.
func cleanup(st *State, notes map[ID]*Note) (changed bool, incomplete bool) {
	referenced := map[ID]bool{}
	for _, p := range st.Desk {
		referenced[p.ID] = true
	}
	for _, b := range st.Boards {
		for _, p := range b.Cards {
			referenced[p.ID] = true
		}
	}
	for id := range st.Tree {
		referenced[id] = true
	}
	missing := 0
	for id := range referenced {
		if _, ok := notes[id]; !ok {
			missing++
		}
	}
	if missing == 0 {
		return false, false
	}
	if len(notes) > 10 && missing*5 > len(referenced) {
		return false, true
	}
	keep := func(ps []Placement) []Placement {
		out := ps[:0]
		for _, p := range ps {
			if _, ok := notes[p.ID]; ok {
				out = append(out, p)
			}
		}
		return out
	}
	st.Desk = keep(st.Desk)
	for i := range st.Boards {
		st.Boards[i].Cards = keep(st.Boards[i].Cards)
	}
	for id := range st.Tree {
		if _, ok := notes[id]; !ok {
			delete(st.Tree, id)
		}
	}
	return true, false
}
