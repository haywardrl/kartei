package slipbox

import (
	"fmt"
	"sort"
)

// Drawer rules. "branch" fills drawers in address order a whole branch at a
// time: small branches share a drawer, a branch never straddles two drawers
// unless it alone is bigger than one, and then it spills into parts. So a
// drawer answers "where is it" without the drawer count tracking the root
// count. "address-range" chunks the whole sequence into equal drawers, which
// is how Luhmann's boxes actually worked: a drawer is a stretch of the
// sequence, not a topic.
const (
	RuleBranch = "branch"
	RuleRange  = "address-range"
)

// Drawer is one physical drawer of one box. Its identity is positional and
// never persisted; Number is what is printed on the front.
type Drawer struct {
	Index    int    `json:"index"`
	Box      string `json:"box"`
	Number   int    `json:"number"` // 1-based within its box; 0 for Unsorted
	Label    string `json:"label"`
	Title    string `json:"title"` // the first root's title (branch rule) or the range
	Root     ID     `json:"root"`  // the first root in the drawer
	Roots    int    `json:"roots"` // how many branches share the drawer
	Part     int    `json:"part"`
	Parts    int    `json:"parts"`
	First    string `json:"first"`
	Last     string `json:"last"`
	IDs      []ID   `json:"ids"`
	Unsorted bool   `json:"unsorted"`
}

func chunk(ids []ID, size int) [][]ID {
	var parts [][]ID
	for start := 0; start < len(ids); start += size {
		parts = append(parts, ids[start:min(start+size, len(ids))])
	}
	return parts
}

// deriveDrawers builds the drawers of every box, main box first. Guests and
// damaged notes go into a final Unsorted drawer.
func deriveDrawers(notes map[ID]*Note, d derived, boxes []Box, rule string, size int) []Drawer {
	if size < 1 {
		size = 60
	}
	var drawers []Drawer
	for _, box := range boxes {
		number := 0
		add := func(title string, root ID, roots int, parts [][]ID) {
			for i, ids := range parts {
				number++
				dr := Drawer{
					Index: len(drawers), Box: box.ID, Number: number, Title: title, Root: root, Roots: roots,
					Part: i + 1, Parts: len(parts), First: d.addr[ids[0]], Last: d.addr[ids[len(ids)-1]],
					IDs: cloneSlice(ids),
				}
				dr.Label = fmt.Sprintf("%d · %s", number, title)
				if roots > 1 {
					dr.Label += fmt.Sprintf(" + %d more", roots-1)
				}
				if len(parts) > 1 {
					dr.Label += fmt.Sprintf(" (%d of %d)", dr.Part, dr.Parts)
				}
				drawers = append(drawers, dr)
			}
		}
		if rule == RuleRange {
			var sorted []ID
			for id, b := range d.boxOf {
				if b == box.ID {
					sorted = append(sorted, id)
				}
			}
			sort.Slice(sorted, func(i, j int) bool { return CompareAddress(d.addr[sorted[i]], d.addr[sorted[j]]) < 0 })
			for _, ids := range chunk(sorted, size) {
				add(fmt.Sprintf("%s – %s", d.addr[ids[0]], d.addr[ids[len(ids)-1]]), "", 0, [][]ID{ids})
			}
		} else {
			// Pack whole branches into drawers in address order.
			var cur []ID
			var curRoot ID
			curRoots := 0
			flush := func() {
				if len(cur) > 0 {
					add(notes[curRoot].Title, curRoot, curRoots, [][]ID{cur})
				}
				cur, curRoot, curRoots = nil, "", 0
			}
			for _, root := range d.roots[box.ID] {
				var branch []ID
				var walk func(id ID)
				walk = func(id ID) {
					branch = append(branch, id)
					for _, kid := range d.children[id] {
						walk(kid)
					}
				}
				walk(root)
				if len(branch) > size { // too big for any drawer: its own parts
					flush()
					add(notes[root].Title, root, 1, chunk(branch, size))
					continue
				}
				if len(cur)+len(branch) > size {
					flush()
				}
				if len(cur) == 0 {
					curRoot = root
				}
				cur = append(cur, branch...)
				curRoots++
			}
			flush()
		}
		if number == 0 && box.ID == boxes[0].ID {
			drawers = append(drawers, Drawer{Index: len(drawers), Box: box.ID, Number: 1, Label: "1 · Empty", Title: "Empty", IDs: []ID{}})
		}
	}
	var unsorted []ID
	for id, n := range notes {
		if n.Guest || n.Damaged {
			unsorted = append(unsorted, id)
		}
	}
	if len(unsorted) > 0 {
		sort.Slice(unsorted, func(i, j int) bool { return notes[unsorted[i]].Path < notes[unsorted[j]].Path })
		drawers = append(drawers, Drawer{Index: len(drawers), Box: boxes[0].ID, Label: "Unsorted", Title: "Unsorted", IDs: unsorted, Unsorted: true})
	}
	return drawers
}
