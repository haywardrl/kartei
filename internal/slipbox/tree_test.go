package slipbox

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestGoldenAddresses(t *testing.T) {
	v := openFixture(t, "fixture-small")
	golden := map[ID]string{
		"20240101T100000": "1",
		"20240105T100000": "1a", // order 1 beats the older ID
		"20240102T100000": "1b",
		"20240103T100000": "1b1",
		"20240104T100000": "1b2",
		"20240106T100000": "2",
		"20240107T100000": "2a",
		"20240108T100000": "2a1",
		"20240109T100000": "2a1a",
		"20240110T100000": "3",
		"20240111T100000": "3a",
		"20240112T100000": "3b",
	}
	for id, want := range golden {
		if got := v.Address(id); got != want {
			t.Errorf("address(%s) = %q, want %q", id, got, want)
		}
	}
	if got := ids(v.Children("20240101T100000")); !reflect.DeepEqual(got, []ID{"20240105T100000", "20240102T100000"}) {
		t.Errorf("children = %v", got)
	}
	if got := ids(v.Roots("")); !reflect.DeepEqual(got, []ID{"20240101T100000", "20240106T100000", "20240110T100000"}) {
		t.Errorf("roots = %v", got)
	}
	if p, ok := v.Parent("20240109T100000"); !ok || p.ID != "20240108T100000" {
		t.Errorf("parent = %v %v", p, ok)
	}
}

func TestLetters(t *testing.T) {
	cases := map[int]string{1: "a", 2: "b", 26: "z", 27: "aa", 28: "ab", 52: "az", 53: "ba", 703: "aaa"}
	for n, want := range cases {
		if got := letters(n); got != want {
			t.Errorf("letters(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestCompareAddress(t *testing.T) {
	in := []string{"1a10", "1a9", "2", "1", "1a", "1b", "1a1", "10", "1z", "1aa", "1a1a", "1a2"}
	sort.Slice(in, func(i, j int) bool { return CompareAddress(in[i], in[j]) < 0 })
	want := []string{"1", "1a", "1a1", "1a1a", "1a2", "1a9", "1a10", "1b", "1z", "1aa", "2", "10"}
	if !reflect.DeepEqual(in, want) {
		t.Fatalf("sorted = %v\nwant     %v", in, want)
	}
	if CompareAddress("1a", "1a") != 0 {
		t.Fatal("equal addresses must compare 0")
	}
}

func TestBrokenTreeLoadsWithWarnings(t *testing.T) {
	done := make(chan *Vault, 1)
	go func() { done <- openFixture(t, "fixture-broken") }()
	var v *Vault
	select {
	case v = <-done:
	case <-timeout(5):
		t.Fatal("open hung on a cyclic tree")
	}
	ws := v.Warnings()
	if !hasWarning(ws, "cycle") || !hasWarning(ws, "orphan") {
		t.Fatalf("expected cycle and orphan warnings, got %+v", ws)
	}
	a, b := v.Address("20240206T100000"), v.Address("20240207T100000")
	if a != "6" || b != "6a" {
		t.Fatalf("cycle should be broken with the lowest ID as root: %q %q", a, b)
	}
	if a := v.Address("20240208T100000"); a == "" || strings.ContainsAny(a, "abcdefghijklmnopqrstuvwxyz") {
		t.Fatalf("orphan child should be a root, got %q", a)
	}
	if v.Address("20240201T100000") != "" {
		t.Fatal("damaged notes get no address")
	}
	if v.Incomplete() {
		t.Fatal("small vault should not trip the mass-loss guard")
	}
	if _, ok := v.state.Tree["20240297T100000"]; ok {
		t.Fatal("tree entry for a missing note should be cleaned up")
	}
	if len(v.state.Desk) != 0 {
		t.Fatalf("desk positions for missing or filed cards should be dropped, got %v", v.state.Desk)
	}
}

func TestParentAddress(t *testing.T) {
	cases := [][3]string{{"1b3", "", "1b"}, {"1b", "", "1"}, {"1", "", ""}, {"1a10", "", "1a"}, {"1aa", "", "1"}, {"L2a", "L", "L2"}, {"L2", "L", ""}, {"L12b3", "L", "L12b"}}
	for _, c := range cases {
		if got := parentAddress(c[0], c[1]); got != c[2] {
			t.Errorf("parentAddress(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}
