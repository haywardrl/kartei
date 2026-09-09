package slipbox

import (
	"strings"
	"testing"
	"time"
)

func TestParseNoteFull(t *testing.T) {
	src := "---\ntitle: Systems fail under load\ncreated: 2026-09-08T14:25:30Z\ntags: [systems, philosophy]\nsource: Nygard\n---\n\nBody with [[20260201T101500|a link]].\n"
	n := ParseNote("20260908T142530--systems-fail-under-load.md", []byte(src))
	if n.Guest || n.Damaged {
		t.Fatalf("flags: guest=%v damaged=%v", n.Guest, n.Damaged)
	}
	if n.ID != "20260908T142530" || n.Title != "Systems fail under load" {
		t.Fatalf("id/title: %q %q", n.ID, n.Title)
	}
	if !n.Created.Equal(time.Date(2026, 9, 8, 14, 25, 30, 0, time.UTC)) {
		t.Fatalf("created: %v", n.Created)
	}
	if len(n.Tags) != 2 || n.Tags[1] != "philosophy" {
		t.Fatalf("tags: %v", n.Tags)
	}
	if n.Extra["source"] != "Nygard" {
		t.Fatalf("extra: %v", n.Extra)
	}
	if n.Body != "Body with [[20260201T101500|a link]].\n" {
		t.Fatalf("body: %q", n.Body)
	}
	if len(n.Links) != 1 || n.Links[0].Target != "20260201T101500" || n.Links[0].Alias != "a link" {
		t.Fatalf("links: %+v", n.Links)
	}
}

func TestParseNoteNoFrontmatter(t *testing.T) {
	n := ParseNote("20240101T100000--quiet-note.md", []byte("just text\n"))
	if n.Title != "Quiet note" {
		t.Fatalf("title fallback: %q", n.Title)
	}
	if n.Created.Year() != 2024 {
		t.Fatalf("created fallback: %v", n.Created)
	}
	if n.Body != "just text\n" {
		t.Fatalf("body: %q", n.Body)
	}
}

func TestParseNoteGuestAndDamaged(t *testing.T) {
	g := ParseNote("reading-list.md", []byte("---\ntitle: Reading list\n---\n\nhi\n"))
	if !g.Guest || g.Title != "Reading list" {
		t.Fatalf("guest: %+v", g)
	}
	d := ParseNote("20240101T100000--bad.md", []byte("---\ntitle: [unclosed\n---\n\nbody\n"))
	if !d.Damaged {
		t.Fatal("expected damaged")
	}
	if !strings.Contains(d.Body, "[unclosed") {
		t.Fatalf("damaged body must keep raw content: %q", d.Body)
	}
	if _, err := d.Marshal(); err == nil {
		t.Fatal("damaged note must refuse to marshal")
	}
}

func TestMarshalRoundTripPreservesExtra(t *testing.T) {
	src := "---\ntitle: Load shedding\ncreated: 2024-01-12T10:00:00Z\ntags: [systems]\nsource: Nygard, Release It!\nrating: 5\n---\n\nDrop the least valuable work first.\n"
	n := ParseNote("20240112T100000--load-shedding.md", []byte(src))
	out, err := n.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	again := ParseNote(n.Path, out)
	if again.Title != n.Title || again.Body != n.Body || !again.Created.Equal(n.Created) {
		t.Fatalf("round trip changed core fields:\n%s", out)
	}
	if again.Extra["source"] != "Nygard, Release It!" || again.Extra["rating"] != 5 {
		t.Fatalf("round trip lost extra keys: %v\n%s", again.Extra, out)
	}
	if strings.Contains(string(out), "id:") {
		t.Fatal("never write an id into frontmatter")
	}
}
