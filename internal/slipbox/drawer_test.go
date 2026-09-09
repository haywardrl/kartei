package slipbox

import (
	"reflect"
	"testing"
)

func TestDrawersAreBranches(t *testing.T) {
	v := openFixture(t, "fixture-small")
	ds := v.Drawers()
	if len(ds) != 3 {
		t.Fatalf("expected one drawer per branch, got %d: %+v", len(ds), ds)
	}
	if ds[0].Label != "1 · Systems fail under load" || ds[1].Label != "2 · Luhmann on selection" || ds[2].Label != "3 · Notes on redundancy" {
		t.Fatalf("labels: %q %q %q", ds[0].Label, ds[1].Label, ds[2].Label)
	}
	if ds[0].Number != 1 || ds[0].Root != "20240101T100000" || ds[0].First != "1" || ds[0].Last != "1b2" {
		t.Fatalf("drawer 1: %+v", ds[0])
	}
	if !reflect.DeepEqual(ds[0].IDs, []ID{"20240101T100000", "20240105T100000", "20240102T100000", "20240103T100000", "20240104T100000"}) {
		t.Fatalf("drawer 1 order = %v", ds[0].IDs)
	}
	// a branch that outgrows a drawer spills into a second part
	v.SetSettings(Settings{DrawerRule: RuleBranch, DrawerSize: 3})
	ds = v.Drawers()
	if len(ds) != 5 || ds[0].Parts != 2 || ds[1].Part != 2 || ds[1].Label != "2 · Systems fail under load (2 of 2)" || ds[2].Label != "3 · Luhmann on selection (1 of 2)" {
		t.Fatalf("spill: %+v", ds)
	}
	for _, d := range ds {
		if d.Box != "main" {
			t.Fatalf("an empty literature box has no drawers: %+v", d)
		}
	}
	if v.Stage() != "cabinet" {
		t.Fatalf("stage = %q", v.Stage())
	}
}

func TestDrawersByAddressRange(t *testing.T) {
	v := openFixture(t, "fixture-small")
	v.SetSettings(Settings{DrawerRule: RuleRange, DrawerSize: 5})
	ds := v.Drawers()
	if len(ds) != 3 || ds[0].Label != "1 · 1 – 1b2" || ds[1].Label != "2 · 2 – 3" || ds[2].Label != "3 · 3a – 3b" {
		t.Fatalf("range labels: %+v", ds)
	}
}

func TestUnsortedDrawer(t *testing.T) {
	v := openFixture(t, "fixture-guests")
	ds := v.Drawers()
	last := ds[len(ds)-1]
	if !last.Unsorted || len(last.IDs) != 2 {
		t.Fatalf("expected an Unsorted drawer with two guests: %+v", last)
	}
	if v.Address("20240302T100000") != "1a" {
		t.Fatalf("owned tree still derives: %q", v.Address("20240302T100000"))
	}
	if err := v.DeskMove(last.IDs[0], 0, 0); err != ErrGuestNote {
		t.Fatalf("guests cannot be placed, got %v", err)
	}
	if _, err := v.NewNote("x", "", last.IDs[0]); err != ErrGuestNote {
		t.Fatalf("cannot file behind a guest, got %v", err)
	}
	owned, _ := v.Note("20240301T100000")
	if owned.Links[0].Resolved == "" || owned.Links[1].Resolved == "" {
		t.Fatalf("guests are linkable: %+v", owned.Links)
	}
}
