package slipbox

import (
	"strings"
	"testing"
	"time"
)

func TestSlugify(t *testing.T) {
	long := strings.Repeat("word-", 20)
	cases := []struct{ in, want string }{
		{"Systems fail under load!", "systems-fail-under-load"},
		{"  Hello,   World  ", "hello-world"},
		{"Café Ölçek naïve", "cafe-olcek-naive"},
		{"!!!", "untitled"},
		{"", "untitled"},
		{"CamelCase123", "camelcase123"},
		{"日本語 only", "only"},
		{long, strings.TrimSuffix(long[:60], "-")},
		{strings.Repeat("a", 70), strings.Repeat("a", 60)},
	}
	for _, c := range cases {
		if got := Slugify(c.in); got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseFilename(t *testing.T) {
	cases := []struct {
		in   string
		id   ID
		slug string
		ok   bool
	}{
		{"20260908T142530--systems-fail-under-load.md", "20260908T142530", "systems-fail-under-load", true},
		{"20260908T142530.md", "20260908T142530", "", true},
		{"20260908T142530--.md", "20260908T142530", "", true},
		{"reading-list.md", "", "", false},
		{"2026090T142530--x.md", "", "", false},
		{"20260908T142530--x.txt", "", "", false},
	}
	for _, c := range cases {
		id, slug, ok := ParseFilename(c.in)
		if id != c.id || slug != c.slug || ok != c.ok {
			t.Errorf("ParseFilename(%q) = (%q,%q,%v), want (%q,%q,%v)", c.in, id, slug, ok, c.id, c.slug, c.ok)
		}
	}
}

func TestIDRoundTrip(t *testing.T) {
	at := time.Date(2026, 9, 8, 14, 25, 30, 0, time.UTC)
	id := NewID(at)
	if id != "20260908T142530" || !id.Valid() {
		t.Fatalf("NewID = %q", id)
	}
	back, err := id.Time()
	if err != nil || !back.Equal(at) {
		t.Fatalf("Time() = %v, %v", back, err)
	}
	if id.Next() != "20260908T142531" {
		t.Fatalf("Next() = %q", id.Next())
	}
	if Filename(id, "Hello World") != "20260908T142530--hello-world.md" {
		t.Fatalf("Filename = %q", Filename(id, "Hello World"))
	}
	if ID("nope").Valid() {
		t.Fatal("nope should not be valid")
	}
}
