package slipbox

import (
	"reflect"
	"testing"
)

func TestExtractLinks(t *testing.T) {
	cases := []struct {
		body string
		want []Link
	}{
		{"plain [[20240101T100000]] here", []Link{{Raw: "[[20240101T100000]]", Target: "20240101T100000"}}},
		{"alias [[20240101T100000|shown text]]", []Link{{Raw: "[[20240101T100000|shown text]]", Target: "20240101T100000", Alias: "shown text"}}},
		{"two [[a]] and [[b]]", []Link{{Raw: "[[a]]", Target: "a"}, {Raw: "[[b]]", Target: "b"}}},
		{"inline `[[skip]]` but [[keep]]", []Link{{Raw: "[[keep]]", Target: "keep"}}},
		{"```\n[[skip]]\n```\n[[keep]]", []Link{{Raw: "[[keep]]", Target: "keep"}}},
		{"~~~\n[[skip]]\n~~~", nil},
		{"nothing here", nil},
		{"[[]] empty is not a link", nil},
	}
	for _, c := range cases {
		got := ExtractLinks(c.body)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ExtractLinks(%q) = %+v, want %+v", c.body, got, c.want)
		}
	}
}

func TestResolutionAndBacklinks(t *testing.T) {
	v := openFixture(t, "fixture-small")
	resolved := func(id ID) []ID {
		n, _ := v.Note(id)
		var out []ID
		for _, l := range n.Links {
			out = append(out, l.Resolved)
		}
		return out
	}
	// by ID, by ID with alias, by title, by slug
	cases := map[ID][]ID{
		"20240101T100000": {"20240110T100000"},
		"20240103T100000": {"20240104T100000"},
		"20240110T100000": {"20240102T100000"},
		"20240111T100000": {"20240102T100000"},
		"20240112T100000": {"20240105T100000"},
		"20240104T100000": nil, // both links are inside code
	}
	for id, want := range cases {
		if got := resolved(id); !reflect.DeepEqual(got, want) {
			t.Errorf("links of %s = %v, want %v", id, got, want)
		}
	}
	back := ids(v.Backlinks("20240102T100000"))
	if !reflect.DeepEqual(back, []ID{"20240110T100000", "20240111T100000"}) {
		t.Errorf("backlinks = %v", back)
	}
	if ws := v.Warnings(); len(ws) != 0 {
		t.Errorf("fixture-small should be clean, got %+v", ws)
	}
}

func TestBrokenAndAmbiguousLinksWarn(t *testing.T) {
	v := openFixture(t, "fixture-broken")
	ws := v.Warnings()
	if !hasWarning(ws, "broken-link") || !hasWarning(ws, "ambiguous-link") || !hasWarning(ws, "damaged") {
		t.Fatalf("missing warnings: %+v", ws)
	}
	n, _ := v.Note("20240205T100000")
	if n.Links[0].Resolved != "20240203T100000" {
		t.Fatalf("ambiguous link should pick the oldest, got %s", n.Links[0].Resolved)
	}
	b, _ := v.Note("20240202T100000")
	if b.Links[0].Resolved != "" || b.Links[0].Raw != "[[does-not-exist]]" {
		t.Fatalf("broken link must be kept unresolved: %+v", b.Links[0])
	}
}

func TestCrossRefsExcludeDescendants(t *testing.T) {
	v := openFixture(t, "fixture-small")
	// 1 links to 3 (cross ref); 1's descendants link back to it (2 links to 1),
	// which must not count as a cross reference.
	got := ids(v.CrossRefs("20240101T100000"))
	if !reflect.DeepEqual(got, []ID{"20240110T100000"}) {
		t.Fatalf("crossrefs = %v", got)
	}
}
