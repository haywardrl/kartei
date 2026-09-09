package slipbox

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func waitEvent(t *testing.T, ch <-chan Event, want ID) {
	t.Helper()
	deadline := time.After(4 * time.Second)
	for {
		select {
		case ev := <-ch:
			for _, id := range ev.IDs {
				if id == want {
					return
				}
			}
		case <-deadline:
			t.Fatalf("no event for %s", want)
		}
	}
}

func TestWatchReflectsExternalEdits(t *testing.T) {
	dir := copyFixture(t, "fixture-small")
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if err := v.Watch(); err != nil {
		t.Fatal(err)
	}
	ch, stop := v.Subscribe()
	defer stop()

	// External create.
	id := ID("20240201T100000")
	path := filepath.Join(dir, Filename(id, "From Neovim"))
	os.WriteFile(path, []byte("---\ntitle: From Neovim\n---\n\nTyped elsewhere.\n"), 0o644)
	waitEvent(t, ch, id)
	if n, ok := v.Note(id); !ok || n.Title != "From Neovim" || v.Filed(id) || len(v.Unfiled()) != 1 {
		t.Fatalf("external note should be indexed and unfiled: %v %v", n, ok)
	}

	// External modify, written the way editors do: temp file then rename.
	tmp := filepath.Join(dir, ".swap.md")
	os.WriteFile(tmp, []byte("---\ntitle: From Neovim\n---\n\nEdited elsewhere.\n"), 0o644)
	os.Rename(tmp, path)
	waitEvent(t, ch, id)
	if n, _ := v.Note(id); n.Body != "Edited elsewhere.\n" {
		t.Fatalf("edit not reflected: %q", n.Body)
	}

	// External rename keeping the ID keeps the note.
	renamed := filepath.Join(dir, Filename(id, "Renamed outside"))
	os.Rename(path, renamed)
	waitEvent(t, ch, id)
	if n, ok := v.Note(id); !ok || n.Path != filepath.Base(renamed) {
		t.Fatalf("rename lost the note: %v %v", n, ok)
	}

	// External delete.
	os.Remove(renamed)
	waitEvent(t, ch, id)
	if _, ok := v.Note(id); ok {
		t.Fatal("deleted note still indexed")
	}
}

func TestWatchIgnoresOwnWrites(t *testing.T) {
	dir := copyFixture(t, "fixture-small")
	v, _ := Open(dir)
	defer v.Close()
	v.Watch()
	ch, stop := v.Subscribe()
	defer stop()
	n, err := v.NewNote("Own write", "body", "")
	if err != nil {
		t.Fatal(err)
	}
	waitEvent(t, ch, n.ID) // the vault's own notify
	select {
	case ev := <-ch:
		t.Fatalf("own write echoed back as external change: %+v", ev)
	case <-time.After(800 * time.Millisecond):
	}
}

func TestWatchIgnoresTheContentsFile(t *testing.T) {
	dir := copyFixture(t, "fixture-small")
	v, _ := Open(dir)
	v.NewNote("A change", "", "20240101T100000") // a structural change, so Close writes _index.md
	v.Close()
	if _, err := os.Stat(filepath.Join(dir, indexFile)); err != nil {
		t.Fatal("index not written")
	}
	again, _ := Open(dir)
	defer again.Close()
	again.Watch()
	ch, stop := again.Subscribe()
	defer stop()
	data, _ := os.ReadFile(filepath.Join(dir, indexFile))
	os.WriteFile(filepath.Join(dir, indexFile), append(data, '\n'), 0o644)
	select {
	case ev := <-ch:
		t.Fatalf("the contents file must never become a card: %+v", ev)
	case <-time.After(900 * time.Millisecond):
	}
	for _, n := range again.Notes() {
		if n.Path == indexFile || n.Title == "Index" {
			t.Fatal("index file was indexed as a note")
		}
	}
	for _, n := range again.Notes() {
		if n.Guest {
			t.Fatalf("no guest should appear after touching the index: %s", n.Path)
		}
	}
	if len(again.Notes()) != 13 {
		t.Fatalf("expected 12 fixture cards plus one, got %d", len(again.Notes()))
	}
}
