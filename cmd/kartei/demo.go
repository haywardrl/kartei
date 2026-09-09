package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/haywardrl/kartei/internal/slipbox"
)

// demoNote is one seeded card. parent is the index of the card it is filed
// behind, -1 for a root, -2 for a card that is still on the desk. links
// refer to other cards by index.
type demoNote struct {
	title  string
	body   string
	parent int
	links  []int
	box    string // for roots: which box; "" is the main box
}

// Four small branches, so the demo can be followed in a few minutes.
var demoNotes = []demoNote{
	// 0: attention
	{"Attention is the scarce input", "Everything else can be bought or borrowed. The hours in which I can hold one thing in mind cannot.", -1, nil, ""},
	{"Notifications are interruptions with good PR", "A badge is a request for attention that does not have to justify itself. Most would not survive being asked.", 0, nil, ""},
	{"Deep work needs a room", "Not a metaphor. A place with a door, where the tools for one kind of work are out and the others are put away.", 0, []int{15}, ""},
	{"The desk should be nearly empty", "Whatever is on the desk is waiting to be filed. A full desk is a backlog, and a backlog on the desk is a wall between me and the work.", 2, nil, ""},
	{"Rest is part of the work", "Ideas connect while I am not looking at them. A walk after filing is not time away from the box.", 0, nil, ""},
	// 5: systems
	{"Systems fail under load", "Every system has a load past which its behaviour changes shape. Design for the shape after, not the shape before.", -1, nil, ""},
	{"Cascading timeouts", "A timeout upstream becomes a retry downstream, which becomes load upstream. The loop closes on itself.", 5, nil, ""},
	{"Retry storms", "Retries multiply load exactly when it is least affordable. Every client is being polite, and together they are a stampede.", 6, []int{8}, ""},
	{"Backpressure", "Refuse work early rather than late. A queue that says no at the door is kinder than one that accepts and then drops.", 6, nil, ""},
	{"Queue depth as a signal", "A growing queue is a leading indicator; latency is a lagging one. Watch the queue.", 5, nil, ""},
	// 10: writing
	{"Writing is thinking made visible", "I do not know what I think until I have tried to write it and watched it fail. The failure is the information.", -1, nil, ""},
	{"The first draft is for finding out what you think", "Nobody sees it. Its only job is to exist so the second draft has something to argue with.", 10, nil, ""},
	{"Sentences carry one idea", "A sentence with two ideas has a hinge in the middle, and hinges are where readers fall off.", 10, nil, ""},
	{"Twenty words is a good limit", "Not a rule. A smell. Past twenty words, check whether the sentence is carrying two thoughts.", 12, nil, ""},
	{"Cut the throat-clearing", "The first paragraph of most drafts is the writer warming up. Delete it and the piece starts where it should.", 10, nil, ""},
	// 15: the room
	{"The cosy room", "A study that is pleasant to be in gets used. The room is not decoration; it is the reason I come back.", -1, []int{2}, ""},
	{"One warm light in a dark room", "Cheapest effect with the largest payoff. A lamp pool on the desk and the rest in shadow.", 15, nil, ""},
	{"Sound does more than art", "A drawer rolling, cards riffling, rain on the window. A weekend of samples beats a month of pixels.", 15, nil, ""},
	{"Traces of use create attachment", "Yesterday's pile is still on the desk. The mug stayed where it was. A room that remembers you is one you return to.", 15, []int{3}, ""},
	{"Filing is deciding", "Every other tool stores notes. Placing a card forces a decision about what it relates to, and that decision is the structure.", 15, nil, ""},
	// 20, 21: on the desk, not yet filed
	{"Does the desk need a cap?", "Maybe it should just look messy past twelve instead of refusing. Try both.", -2, []int{3}, ""},
	{"Rain on the window", "A sound, not a feature. But sound was the second-cheapest cosiness.", -2, []int{17}, ""},
	// 22, 23: the literature box, one source and one excerpt
	{"Luhmann 1992, Communicating with slip boxes", "Luhmann, N. (1992). Kommunikation mit Zettelkästen. The essay behind the whole idea.", -1, nil, "lit"},
	{"p. 2: the box as a communication partner", "A box that has been kept long enough surprises its keeper: it answers with connections that were never put in on purpose.", 22, []int{19, 0}, ""},
}

// seedDemo writes the demo vault by hand rather than through the engine so
// the IDs are stable, then lets the engine load it like any other folder.
func seedDemo(dir string) error {
	if err := os.MkdirAll(filepath.Join(dir, ".kartei"), 0o755); err != nil {
		return err
	}
	base := time.Date(2025, 1, 6, 9, 0, 0, 0, time.UTC)
	ids := make([]slipbox.ID, len(demoNotes))
	for i := range demoNotes {
		ids[i] = slipbox.NewID(base.Add(time.Duration(i*31) * time.Hour))
	}
	tree := map[slipbox.ID]slipbox.TreeEntry{}
	orders := map[int]int{}
	for i, d := range demoNotes {
		body := d.body
		for _, l := range d.links {
			body += fmt.Sprintf(" See [[%s|%s]].", ids[l], demoNotes[l].title)
		}
		created, _ := ids[i].Time()
		n := &slipbox.Note{ID: ids[i], Title: d.title, Created: created, Body: body + "\n"}
		data, err := n.Marshal()
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, slipbox.Filename(ids[i], d.title)), data, 0o644); err != nil {
			return err
		}
		switch {
		case d.parent >= 0:
			orders[d.parent]++
			tree[ids[i]] = slipbox.TreeEntry{Parent: ids[d.parent], Order: orders[d.parent]}
		case d.parent == -1:
			key := -1
			if d.box != "" {
				key = -100
			}
			orders[key]++
			tree[ids[i]] = slipbox.TreeEntry{Parent: "", Order: orders[key], Box: d.box}
		}
	}
	st := map[string]any{
		"version": 2,
		"tree":    tree,
		"desk":    []slipbox.Placement{{ID: ids[20], X: 30, Y: 4}, {ID: ids[21], X: 70, Y: 9}},
		"boards": []slipbox.Board{
			{ID: "b_the-room", Name: "The room", Created: base, Cards: []slipbox.Placement{
				{ID: ids[15], X: 80, Y: 60}, {ID: ids[2], X: 520, Y: 90}, {ID: ids[18], X: 300, Y: 360}, {ID: ids[3], X: 760, Y: 300}}},
		},
		"register": []slipbox.RegisterEntry{
			{Term: "attention", Targets: []slipbox.ID{ids[0], ids[2]}, Note: "the scarce input"},
			{Term: "filing", Targets: []slipbox.ID{ids[19], ids[3]}, Note: ""},
			{Term: "load", Targets: []slipbox.ID{ids[5], ids[9]}, Note: "systems under stress"},
			{Term: "Luhmann", Targets: []slipbox.ID{ids[22], ids[23], ids[19]}, Note: ""},
		},
		"settings": slipbox.Settings{DrawerRule: slipbox.RuleBranch, DrawerSize: 5, DeskCapacity: 12}, // five a drawer, so the demo shows one drawer per branch
		"prefs":    map[string]any{"lamp": true},
	}
	data, err := jsonIndent(st)
	if err != nil {
		return err
	}
	return slipbox.WriteFileAtomic(dir, filepath.Join(dir, ".kartei", "state.json"), data, 0o644)
}
