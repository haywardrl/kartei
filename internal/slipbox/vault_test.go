package slipbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func timeout(sec int) <-chan time.Time { return time.After(time.Duration(sec) * time.Second) }

func TestNewNoteWriteBehindAndReload(t *testing.T) {
	dir := copyFixture(t, "fixture-small")
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	n, err := v.NewNote("Circuit breakers", "Trip early, recover slowly. See [[20240102T100000|cascading timeouts]].", "20240102T100000")
	if err != nil {
		t.Fatal(err)
	}
	if v.Address(n.ID) != "1b3" {
		t.Fatalf("new child address = %q", v.Address(n.ID))
	}
	root, err := v.NewNote("A fresh branch", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if v.Address(root.ID) != "" || v.Filed(root.ID) || len(v.Unfiled()) != 1 {
		t.Fatalf("a card with no parent starts unfiled: %q", v.Address(root.ID))
	}
	if err := v.File(root.ID, "", ""); err != nil {
		t.Fatal(err)
	}
	if v.Address(root.ID) != "4" {
		t.Fatalf("filed root address = %q", v.Address(root.ID))
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}

	again, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	got, ok := again.Note(n.ID)
	if !ok || got.Title != "Circuit breakers" || got.Body != n.Body || !got.Created.Equal(n.Created) {
		t.Fatalf("reload mismatch: %+v", got)
	}
	if again.Address(n.ID) != "1b3" || again.Address(root.ID) != "4" {
		t.Fatal("addresses did not survive reload")
	}
	if back := ids(again.Backlinks("20240102T100000")); len(back) != 3 {
		t.Fatalf("backlinks after reload = %v", back)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp") {
			t.Fatal("temp file left behind")
		}
	}
}

func TestUpdateNoteRenamesKeepingID(t *testing.T) {
	v := openFixture(t, "fixture-small")
	id := ID("20240111T100000")
	if err := v.UpdateNote(id, "Bulkheads and compartments", "New body."); err != nil {
		t.Fatal(err)
	}
	n, _ := v.Note(id)
	if n.Path != "20240111T100000--bulkheads-and-compartments.md" {
		t.Fatalf("path = %q", n.Path)
	}
	if _, err := os.Stat(filepath.Join(v.Root(), "20240111T100000--bulkheads.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("old file should be gone")
	}
	if v.Address(id) != "3a" {
		t.Fatal("placement lost on rename")
	}
	if len(v.BoardsList()[0].Cards) != 2 {
		t.Fatal("board pin lost on rename")
	}
}

func TestDeleteReparentsAndTrashes(t *testing.T) {
	v := openFixture(t, "fixture-small")
	if err := v.DeleteNote("20240102T100000"); err != nil { // 1b, has two children
		t.Fatal(err)
	}
	if v.Address("20240103T100000") != "1b" || v.Address("20240104T100000") != "1c" {
		t.Fatalf("children should move up: %q %q", v.Address("20240103T100000"), v.Address("20240104T100000"))
	}
	if _, err := os.Stat(filepath.Join(v.Root(), ".kartei", "trash", "20240102T100000--cascading-timeouts.md")); err != nil {
		t.Fatal("note should be in trash")
	}
	if hasWarning(v.Warnings(), "broken-link") == false {
		t.Fatal("links to the deleted note should now warn, not vanish")
	}
	// Deleting a root makes its children roots.
	if err := v.DeleteNote("20240106T100000"); err != nil {
		t.Fatal(err)
	}
	if v.Address("20240107T100000") != "2" {
		t.Fatalf("child of deleted root = %q", v.Address("20240107T100000"))
	}
}

func TestFileRefusesCycles(t *testing.T) {
	v := openFixture(t, "fixture-small")
	if err := v.File("20240101T100000", "20240104T100000", ""); err != ErrCycle {
		t.Fatalf("want ErrCycle, got %v", err)
	}
	if err := v.File("20240112T100000", "20240106T100000", ""); err != nil {
		t.Fatal(err)
	}
	if v.Address("20240112T100000") != "2b" {
		t.Fatalf("refiled address = %q", v.Address("20240112T100000"))
	}
	if err := v.File("20240112T100000", "", ""); err != nil || v.Address("20240112T100000") != "4" {
		t.Fatalf("make root: %v %q", err, v.Address("20240112T100000"))
	}
}

func TestLiteratureBox(t *testing.T) {
	v := openFixture(t, "fixture-small")
	src, _ := v.NewNote("Luhmann 1992, Communicating with slip boxes", "The essay.", "")
	if err := v.FileAsRoot(src.ID, "lit", ""); err != nil {
		t.Fatal(err)
	}
	if v.Address(src.ID) != "L1" || v.BoxOf(src.ID) != "lit" {
		t.Fatalf("literature root: %q %q", v.Address(src.ID), v.BoxOf(src.ID))
	}
	ex, _ := v.NewNote("p. 3, the box as partner", "Excerpt. Links to [[20240106T100000|Luhmann on selection]].", src.ID)
	if v.Address(ex.ID) != "L1a" || v.BoxOf(ex.ID) != "lit" {
		t.Fatalf("literature child: %q", v.Address(ex.ID))
	}
	// links cross boxes, drawers do not
	if back := ids(v.Backlinks("20240106T100000")); len(back) != 1 || back[0] != ex.ID {
		t.Fatalf("backlink across boxes: %v", back)
	}
	var lit, main int
	for _, d := range v.Drawers() {
		if d.Box == "lit" {
			lit++
			if d.Label != "1 · Luhmann 1992, Communicating with slip boxes" || len(d.IDs) != 2 {
				t.Fatalf("lit drawer: %+v", d)
			}
		} else if !d.Unsorted {
			main++
		}
	}
	if lit != 1 || main != 3 {
		t.Fatalf("drawers per box: lit %d main %d", lit, main)
	}
	// moving a card from one box to the other renumbers both root lists
	if err := v.FileAsRoot(ex.ID, "", First); err != nil {
		t.Fatal(err)
	}
	if v.Address(ex.ID) != "1" || v.Address("20240101T100000") != "2" || len(v.Children(src.ID)) != 0 {
		t.Fatalf("move across boxes: %q %q", v.Address(ex.ID), v.Address("20240101T100000"))
	}
	if err := v.FileAsRoot(ex.ID, "nope", ""); err != ErrNoSuchBox {
		t.Fatalf("want ErrNoSuchBox, got %v", err)
	}
	if got := ids(v.Roots("lit")); len(got) != 1 || got[0] != src.ID {
		t.Fatalf("lit roots: %v", got)
	}
}

func TestIndexFileRebuildsTheTree(t *testing.T) {
	dir := copyFixture(t, "fixture-small")
	v, _ := Open(dir)
	src, _ := v.NewNote("A source", "", "")
	v.FileAsRoot(src.ID, "lit", "")
	v.NewNote("An excerpt", "", src.ID)
	v.RegisterAdd("load", "20240101T100000", "")
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile(filepath.Join(dir, indexFile))
	if err != nil {
		t.Fatal("index file not written")
	}
	for _, want := range []string{"### Drawer 1 · Systems fail under load", "- 1 [[20240101T100000|Systems fail under load]]", "  - 1b [[20240102T100000|Cascading timeouts]]", "    - 1b2 [[20240104T100000|Backpressure]]", "## Literature (lit)", "- L1 [[" + string(src.ID), "  - L1a [[", "## Register", "- load: 1"} {
		if !strings.Contains(string(index), want) {
			t.Fatalf("index missing %q:\n%s", want, index)
		}
	}
	expected := map[ID]string{}
	for _, n := range v.Notes() {
		expected[n.ID] = v.Address(n.ID)
	}
	// lose the sidecar entirely
	os.Remove(sidecarPath(dir))
	os.RemoveAll(filepath.Join(dir, sidecarDir, backupDir))
	again, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if !hasWarning(again.Warnings(), "index") {
		t.Fatalf("expected an index warning, got %+v", again.Warnings())
	}
	for id, addr := range expected {
		if got := again.Address(id); got != addr {
			t.Fatalf("address after recovery: %s = %q, want %q", id, got, addr)
		}
	}
	if again.BoxOf(src.ID) != "lit" {
		t.Fatal("box lost in recovery")
	}
}

func TestBoardsAreReferences(t *testing.T) {
	v := openFixture(t, "fixture-small")
	b, _ := v.NewBoard("Novel")
	b2, _ := v.NewBoard("Novel")
	if b.ID != "b_novel" || b2.ID != "b_novel-2" {
		t.Fatalf("board ids: %q %q", b.ID, b2.ID)
	}
	if err := v.BoardPin(b.ID, "20240101T100000", 5, 6); err != nil {
		t.Fatal(err)
	}
	if err := v.BoardPin(b2.ID, "20240101T100000", 7, 8); err != nil {
		t.Fatal(err)
	}
	if v.Address("20240101T100000") != "1" {
		t.Fatal("pinning must not move the card out of its drawer")
	}
	if err := v.BoardPin(b.ID, "20240101T100000", 50, 60); err != nil {
		t.Fatal(err)
	}
	if boards := v.BoardsList(); boards[1].Cards[0].X != 50 || len(boards[1].Cards) != 1 {
		t.Fatalf("re-pin should move, not duplicate: %+v", boards[1].Cards)
	}
	if err := v.BoardUnpin(b.ID, "20240101T100000"); err != nil {
		t.Fatal(err)
	}
	if err := v.BoardUnpin(b.ID, "20240101T100000"); err != ErrNotFound {
		t.Fatal("unpinning twice should fail")
	}
	if err := v.DeleteBoard(b2.ID); err != nil || len(v.BoardsList()) != 2 {
		t.Fatalf("delete board: %v %d", err, len(v.BoardsList()))
	}
}

func TestSearchTitlesFirst(t *testing.T) {
	v := openFixture(t, "fixture-small")
	got := v.Search("queue")
	if len(got) != 2 || got[0].ID != "20240105T100000" || got[1].ID != "20240112T100000" {
		t.Fatalf("search = %v", ids(got))
	}
	if len(v.Search("  ")) != 0 {
		t.Fatal("blank query returns nothing")
	}
}

func TestRediscoverIsStableWithinADay(t *testing.T) {
	v := openFixture(t, "fixture-small")
	day := time.Date(2026, 9, 9, 8, 0, 0, 0, time.UTC)
	a, b := v.Rediscover(day), v.Rediscover(day.Add(6*time.Hour))
	if a == "" || a != b {
		t.Fatalf("rediscover unstable: %q %q", a, b)
	}
	if !v.Filed(a) {
		t.Fatal("rediscovered card must be a filed card")
	}
}

func TestPrefsPersist(t *testing.T) {
	dir := copyFixture(t, "fixture-small")
	v, _ := Open(dir)
	v.SetPrefs(map[string]any{"lamp": true, "camera": 120.0})
	v.Close()
	again, _ := Open(dir)
	defer again.Close()
	if p := again.Prefs(); p["lamp"] != true || p["camera"] != 120.0 {
		t.Fatalf("prefs = %v", p)
	}
}

// seedVault writes n notes in a realistic tree straight to disk.
func seedVault(tb testing.TB, dir string, n int) {
	tb.Helper()
	os.MkdirAll(filepath.Join(dir, ".kartei"), 0o755)
	st := defaultState()
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	var all []ID
	for i := 0; i < n; i++ {
		id := NewID(base.Add(time.Duration(i) * time.Minute))
		body := fmt.Sprintf("Note %d in a synthetic vault.", i)
		if i > 0 {
			body += " See [[" + string(all[i*7%len(all)]) + "]]."
		}
		note := &Note{ID: id, Title: fmt.Sprintf("Synthetic note %d", i), Created: base, Body: body}
		data, _ := note.Marshal()
		os.WriteFile(filepath.Join(dir, Filename(id, note.Title)), data, 0o644)
		if i > 0 && i%9 != 0 {
			st.Tree[id] = TreeEntry{Parent: all[(i-1)/3], Order: i % 3}
		}
		all = append(all, id)
	}
	saveState(dir, st)
}

func TestOpenFiveThousandNotesUnderTwoSeconds(t *testing.T) {
	if testing.Short() {
		t.Skip("scale test")
	}
	dir := t.TempDir()
	seedVault(t, dir, 5000)
	start := time.Now()
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	took := time.Since(start)
	t.Logf("Open() with 5000 notes took %v; %d drawers; %d warnings", took, len(v.Drawers()), len(v.Warnings()))
	if took > 2*time.Second {
		t.Fatalf("Open took %v, budget is 2s", took)
	}
	if len(v.Notes()) != 5000 {
		t.Fatalf("notes = %d", len(v.Notes()))
	}
}

func BenchmarkOpen5000(b *testing.B) {
	dir := b.TempDir()
	seedVault(b, dir, 5000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, err := Open(dir)
		if err != nil {
			b.Fatal(err)
		}
		v.Close()
	}
}

func TestFilePositions(t *testing.T) {
	v := openFixture(t, "fixture-small")
	n, _ := v.NewNote("Inserted", "", "")
	// after a specific sibling
	if err := v.File(n.ID, "20240101T100000", "20240105T100000"); err != nil {
		t.Fatal(err)
	}
	if v.Address(n.ID) != "1b" || v.Address("20240102T100000") != "1c" {
		t.Fatalf("insert after 1a: %q %q", v.Address(n.ID), v.Address("20240102T100000"))
	}
	// first among siblings
	if err := v.File(n.ID, "20240101T100000", First); err != nil {
		t.Fatal(err)
	}
	if v.Address(n.ID) != "1a" || v.Address("20240105T100000") != "1b" {
		t.Fatalf("insert first: %q %q", v.Address(n.ID), v.Address("20240105T100000"))
	}
	// moving a branch keeps its children
	if err := v.File("20240102T100000", "20240106T100000", ""); err != nil {
		t.Fatal(err)
	}
	if v.Address("20240102T100000") != "2b" || v.Address("20240103T100000") != "2b1" {
		t.Fatalf("branch move: %q %q", v.Address("20240102T100000"), v.Address("20240103T100000"))
	}
	// root level, first
	if err := v.File(n.ID, "", First); err != nil || v.Address(n.ID) != "1" || v.Address("20240101T100000") != "2" {
		t.Fatalf("root first: %v %q %q", err, v.Address(n.ID), v.Address("20240101T100000"))
	}
	if err := v.File(n.ID, "20240101T100000", "20240110T100000"); err != ErrNotSibling {
		t.Fatalf("want ErrNotSibling, got %v", err)
	}
	loose, _ := v.NewNote("Loose", "", "")
	if err := v.File(n.ID, loose.ID, ""); err != ErrParentUnfiled {
		t.Fatalf("cannot file behind an unfiled card, got %v", err)
	}
	// orders are clean integers after all that
	for i, kid := range v.Children("20240101T100000") {
		if v.state.Tree[kid.ID].Order != i+1 {
			t.Fatalf("orders not renumbered: %+v", v.state.Tree[kid.ID])
		}
	}
}

func TestDeskIsTheInbox(t *testing.T) {
	v := openFixture(t, "fixture-small")
	if len(v.Desk()) != 0 {
		t.Fatalf("a fully filed box has an empty desk, got %v", v.Desk())
	}
	n, _ := v.NewNote("Fleeting", "a thought", "")
	desk := v.Desk()
	if len(desk) != 1 || desk[0].ID != n.ID || desk[0].X != -1 {
		t.Fatalf("a new card appears on the desk unplaced: %v", desk)
	}
	if err := v.DeskMove(n.ID, 30, 4); err != nil {
		t.Fatal(err)
	}
	if d := v.Desk(); d[0].X != 30 {
		t.Fatalf("position not kept: %v", d)
	}
	if err := v.DeskMove("20240101T100000", 1, 1); err != ErrNotFound {
		t.Fatalf("filed cards are not on the desk: %v", err)
	}
	for _, d := range v.Drawers() {
		for _, id := range d.IDs {
			if id == n.ID {
				t.Fatal("unfiled card must not appear in a drawer")
			}
		}
	}
	if r := v.Rediscover(time.Now()); r == n.ID {
		t.Fatal("unfiled cards are not rediscovered")
	}
	if err := v.File(n.ID, "20240110T100000", ""); err != nil {
		t.Fatal(err)
	}
	if len(v.Desk()) != 0 || v.Address(n.ID) != "3c" || len(v.state.Desk) != 0 {
		t.Fatalf("filing clears the desk: %v %q", v.Desk(), v.Address(n.ID))
	}
}

func TestUpdateDetectsOutsideEdits(t *testing.T) {
	v := openFixture(t, "fixture-small")
	id := ID("20240111T100000")
	n, _ := v.Note(id)
	path := filepath.Join(v.Root(), n.Path)
	original, _ := os.ReadFile(path)
	time.Sleep(20 * time.Millisecond)
	edited := append(original, []byte("\nAdded in another editor.\n")...)
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	err := v.UpdateNote(id, "Bulkheads", "Mine.")
	var c *ConflictError
	if !errors.As(err, &c) || !errors.Is(err, ErrConflict) {
		t.Fatalf("want ConflictError, got %v", err)
	}
	if !strings.Contains(c.OnDisk.Body, "Added in another editor") {
		t.Fatalf("conflict should carry the disk version: %q", c.OnDisk.Body)
	}
	if now, _ := os.ReadFile(path); string(now) != string(edited) {
		t.Fatal("a conflict must not write anything")
	}
	if err := v.Overwrite(id, "Bulkheads", "Mine."); err != nil {
		t.Fatal(err)
	}
	if err := v.UpdateNote(id, "Bulkheads", "Mine again."); err != nil {
		t.Fatalf("after our own write there is no conflict: %v", err)
	}
	// An editor that opened an older version conflicts even if the engine
	// itself is up to date (the watcher keeps it so).
	opened, _ := v.Note(id)
	seen := opened.ModTime
	time.Sleep(20 * time.Millisecond)
	if err := v.UpdateNote(id, "Bulkheads", "Someone else, via the API."); err != nil {
		t.Fatal(err)
	}
	if err := v.UpdateNoteSeen(id, "Bulkheads", "Stale editor.", seen); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale editor should conflict, got %v", err)
	}
}

func TestMigrationGivesOldRootsEntries(t *testing.T) {
	dir := copyFixture(t, "fixture-small") // version 1 sidecar: roots have no entries
	v, _ := Open(dir)
	if len(v.Unfiled()) != 0 {
		t.Fatalf("version 1 roots must stay roots, got unfiled %v", ids(v.Unfiled()))
	}
	if v.state.Version != stateVersion {
		t.Fatal("version not bumped")
	}
	v.Close()
	again, _ := Open(dir)
	defer again.Close()
	if again.Address("20240110T100000") != "3" || len(again.Unfiled()) != 0 {
		t.Fatal("migration did not persist")
	}
}

func TestRegister(t *testing.T) {
	v := openFixture(t, "fixture-small")
	if err := v.RegisterAdd("Load", "20240101T100000", "when things break"); err != nil {
		t.Fatal(err)
	}
	if err := v.RegisterAdd("load", "20240112T100000", ""); err != nil {
		t.Fatal(err)
	}
	reg := v.Register()
	if len(reg) != 2 || reg[0].Term != "Load" || len(reg[0].Targets) != 2 || reg[0].Note != "when things break" {
		t.Fatalf("register = %+v", reg)
	}
	if err := v.RegisterRemove("load", "20240101T100000"); err != nil || len(v.Register()[0].Targets) != 1 {
		t.Fatalf("remove target: %v %+v", err, v.Register())
	}
	v.DeleteNote("20240112T100000")
	if len(v.Register()[0].Targets) != 0 {
		t.Fatal("deleting a card removes it from the register")
	}
	if err := v.RegisterRemove("load", ""); err != nil || len(v.Register()) != 1 {
		t.Fatalf("remove term: %v %+v", err, v.Register())
	}
	if err := v.RegisterAdd("  ", "", ""); err == nil {
		t.Fatal("empty term refused")
	}
}

func TestAddressLinksResolve(t *testing.T) {
	v := openFixture(t, "fixture-small")
	n, _ := v.NewNote("By address", "See [[2a1]] and [[1B]].", "20240106T100000")
	got, _ := v.Note(n.ID)
	if got.Links[0].Resolved != "20240108T100000" || got.Links[1].Resolved != "20240102T100000" {
		t.Fatalf("address links: %+v", got.Links)
	}
}

func TestAdoptGuest(t *testing.T) {
	v := openFixture(t, "fixture-guests")
	guest := ID("guest-reading-list")
	if _, ok := v.Note(guest); !ok {
		t.Fatal("guest not found")
	}
	if _, err := v.Adopt("20240301T100000"); err == nil {
		t.Fatal("only guests can be adopted")
	}
	n, err := v.Adopt(guest)
	if err != nil {
		t.Fatal(err)
	}
	if n.Guest || !n.ID.Valid() || n.Title != "Reading list" || v.Filed(n.ID) {
		t.Fatalf("adopted: %+v filed=%v", n, v.Filed(n.ID))
	}
	if !strings.HasSuffix(n.Path, "--reading-list.md") {
		t.Fatalf("path should keep the old name as slug: %s", n.Path)
	}
	if _, err := os.Stat(filepath.Join(v.Root(), "reading-list.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("old file should be renamed away")
	}
	if len(v.Desk()) != 1 || v.Desk()[0].ID != n.ID {
		t.Fatalf("an adopted card lies on the desk: %v", v.Desk())
	}
	owner, _ := v.Note("20240301T100000")
	if owner.Links[0].Resolved != n.ID {
		t.Fatalf("links by the old slug should follow the adopted card: %+v", owner.Links[0])
	}
	if n.Tags[0] != "books" {
		t.Fatal("frontmatter must be preserved")
	}
}
