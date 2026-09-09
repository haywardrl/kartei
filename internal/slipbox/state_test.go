package slipbox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSidecarRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st := defaultState()
	st.Desk = []Placement{{ID: "20240101T100000", X: 3, Y: 4}}
	st.Boards = []Board{{ID: "b_x", Name: "X", Created: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Cards: []Placement{{ID: "20240101T100000", X: 1, Y: 2}}}}
	st.Tree["20240102T100000"] = TreeEntry{Parent: "20240101T100000", Order: 2}
	st.Prefs["camera"] = 12.0
	if err := saveState(dir, st); err != nil {
		t.Fatal(err)
	}
	back, _, err := loadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(st)
	b, _ := json.Marshal(back)
	if string(a) != string(b) {
		t.Fatalf("round trip differs:\n%s\n%s", a, b)
	}
}

func TestMissingSidecarIsDefault(t *testing.T) {
	st, _, err := loadState(t.TempDir())
	if err != nil || st.Settings.DeskCapacity != 12 || st.Settings.DrawerSize != 60 || st.Settings.DrawerRule != RuleBranch {
		t.Fatalf("defaults: %+v %v", st, err)
	}
}

func TestMassLossGuard(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".kartei"), 0o755)
	st := defaultState()
	for i := 0; i < 12; i++ {
		id := ID(fmt.Sprintf("2024010%dT1%05d", 1, i))
		os.WriteFile(filepath.Join(dir, Filename(id, "n")), []byte("x"), 0o644)
		st.Tree[id] = TreeEntry{Parent: "", Order: 0}
	}
	for i := 0; i < 8; i++ {
		st.Desk = append(st.Desk, Placement{ID: ID(fmt.Sprintf("2030010%dT100000", i+1))})
	}
	saveState(dir, st)
	before, _ := os.ReadFile(sidecarPath(dir))
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Incomplete() {
		t.Fatal("8 of 20 references missing must flag the vault incomplete")
	}
	if err := v.Save(); err != ErrVaultIncomplete {
		t.Fatalf("save must refuse, got %v", err)
	}
	after, _ := os.ReadFile(sidecarPath(dir))
	if string(before) != string(after) {
		t.Fatal("sidecar was rewritten while incomplete")
	}
	if len(v.state.Desk) != 8 {
		t.Fatal("placements must be left alone while incomplete")
	}
}

func TestCorruptSidecarIsSetAsideAndRestoredFromBackup(t *testing.T) {
	dir := copyFixture(t, "fixture-small")
	v, _ := Open(dir)
	v.NewBoard("Kept")
	if err := v.Close(); err != nil { // first save of the session takes a backup of the fixture sidecar
		t.Fatal(err)
	}
	again, _ := Open(dir)
	again.NewBoard("Second")
	again.Close() // backup now holds the "Kept" state
	if err := os.WriteFile(sidecarPath(dir), []byte("{ this is not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	third, err := Open(dir)
	if err != nil {
		t.Fatalf("open with corrupt sidecar: %v", err)
	}
	defer third.Close()
	if !hasWarning(third.Warnings(), "sidecar") {
		t.Fatalf("expected a sidecar warning, got %+v", third.Warnings())
	}
	boards := third.BoardsList()
	if len(boards) < 2 || boards[1].Name != "Kept" {
		t.Fatalf("layout should come back from the backup, got %+v", boards)
	}
	matches, _ := filepath.Glob(sidecarPath(dir) + ".broken-*")
	if len(matches) != 1 {
		t.Fatal("the corrupt sidecar must be kept aside, not deleted")
	}
	if third.Address("20240110T100000") != "3" {
		t.Fatal("tree lost in recovery")
	}
}
