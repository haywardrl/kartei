package slipbox

import (
	"sort"
	"strconv"
	"strings"
)

// TreeEntry records which card a note was filed behind. It lives only in the
// sidecar; the address is derived from it and never stored. Box matters only
// on roots (empty parent): children belong to their root's box.
type TreeEntry struct {
	Parent ID     `json:"parent"`
	Order  int    `json:"order"`
	Box    string `json:"box,omitempty"`
}

// derived holds everything reindex computes from the tree.
type derived struct {
	addr     map[ID]string
	children map[ID][]ID
	roots    map[string][]ID // box id -> roots in order
	boxOf    map[ID]string   // every filed card -> its box
	warnings []Warning
}

// letters renders n (1-based) in bijective base-26: 1=a, 26=z, 27=aa.
func letters(n int) string {
	var b []byte
	for n > 0 {
		n--
		b = append([]byte{byte('a' + n%26)}, b...)
		n /= 26
	}
	return string(b)
}

func symbol(depth, n int) string {
	if depth%2 == 0 {
		return strconv.Itoa(n)
	}
	return letters(n)
}

// deriveAddresses walks the tree from the roots of each box and assigns every
// filed note an address. Cycles are broken by promoting the lowest ID in the
// cycle to a root of the main box; entries whose parent does not resolve make
// their note a root. Never loops, never panics.
func deriveAddresses(notes map[ID]*Note, tree map[ID]TreeEntry, boxes []Box) derived {
	d := derived{addr: map[ID]string{}, children: map[ID][]ID{}, roots: map[string][]ID{}, boxOf: map[ID]string{}}
	parentOf := map[ID]ID{}
	filed := func(id ID) bool {
		n, ok := notes[id]
		if !ok || n.Guest || n.Damaged {
			return false
		}
		_, ok = tree[id]
		return ok
	}
	for id, entry := range tree {
		if !filed(id) || entry.Parent == "" || entry.Parent == id {
			continue
		}
		if !filed(entry.Parent) {
			d.warnings = append(d.warnings, Warning{Kind: "orphan", ID: id, Message: "parent " + string(entry.Parent) + " is not in the box; treated as root"})
			continue
		}
		parentOf[id] = entry.Parent
		d.children[entry.Parent] = append(d.children[entry.Parent], id)
	}
	less := func(a, b ID) bool {
		if oa, ob := tree[a].Order, tree[b].Order; oa != ob {
			return oa < ob
		}
		return a < b
	}
	for _, kids := range d.children {
		sort.Slice(kids, func(i, j int) bool { return less(kids[i], kids[j]) })
	}

	boxIDs := map[string]bool{}
	for _, b := range boxes {
		boxIDs[b.ID] = true
	}
	mainBox := boxes[0].ID
	for id := range tree {
		if !filed(id) {
			continue
		}
		if _, hasParent := parentOf[id]; hasParent {
			continue
		}
		box := tree[id].Box
		if !boxIDs[box] {
			box = mainBox
		}
		d.roots[box] = append(d.roots[box], id)
	}

	visited := map[ID]bool{}
	var walk func(id ID, box, prefix string, depth int)
	walk = func(id ID, box, prefix string, depth int) {
		visited[id] = true
		d.addr[id] = prefix
		d.boxOf[id] = box
		n := 0
		for _, kid := range d.children[id] {
			if visited[kid] {
				continue
			}
			n++
			walk(kid, box, prefix+symbol(depth+1, n), depth+1)
		}
	}
	for _, b := range boxes {
		roots := d.roots[b.ID]
		sort.Slice(roots, func(i, j int) bool { return less(roots[i], roots[j]) })
		d.roots[b.ID] = roots
		for i, id := range roots {
			walk(id, b.ID, b.Prefix+strconv.Itoa(i+1), 0)
		}
	}

	// Anything filed but unvisited sits in a cycle. Promote the lowest ID of
	// each component to a root of the main box and keep going.
	var stranded []ID
	for id := range tree {
		if filed(id) && !visited[id] {
			stranded = append(stranded, id)
		}
	}
	sort.Slice(stranded, func(i, j int) bool { return stranded[i] < stranded[j] })
	for _, id := range stranded {
		if visited[id] {
			continue
		}
		d.warnings = append(d.warnings, Warning{Kind: "cycle", ID: id, Message: "tree cycle detected; promoted to root"})
		d.roots[mainBox] = append(d.roots[mainBox], id)
		delete(parentOf, id)
		walk(id, mainBox, boxes[0].Prefix+strconv.Itoa(len(d.roots[mainBox])), 0)
	}
	// Children lists must not point across a broken cycle edge.
	for parent, kids := range d.children {
		kept := kids[:0]
		for _, k := range kids {
			if parentOf[k] == parent {
				kept = append(kept, k)
			}
		}
		d.children[parent] = kept
	}
	return d
}

type addrPart struct {
	num   int
	str   string
	isNum bool
}

func splitAddress(a string) []addrPart {
	var parts []addrPart
	i := 0
	for i < len(a) {
		j := i
		if a[i] >= '0' && a[i] <= '9' {
			for j < len(a) && a[j] >= '0' && a[j] <= '9' {
				j++
			}
			n, _ := strconv.Atoi(a[i:j])
			parts = append(parts, addrPart{num: n, isNum: true})
		} else {
			for j < len(a) && !(a[j] >= '0' && a[j] <= '9') {
				j++
			}
			parts = append(parts, addrPart{str: a[i:j]})
		}
		i = j
	}
	return parts
}

// CompareAddress orders addresses naturally: 1a9 before 1a10, and a shorter
// address before one that extends it. Addresses from different boxes carry
// different prefixes and sort by prefix first.
func CompareAddress(a, b string) int {
	pa, pb := splitAddress(a), splitAddress(b)
	for i := 0; i < len(pa) && i < len(pb); i++ {
		x, y := pa[i], pb[i]
		switch {
		case x.isNum && y.isNum:
			if x.num != y.num {
				if x.num < y.num {
					return -1
				}
				return 1
			}
		case !x.isNum && !y.isNum:
			if len(x.str) != len(y.str) {
				if len(x.str) < len(y.str) {
					return -1
				}
				return 1
			}
			if c := strings.Compare(x.str, y.str); c != 0 {
				return c
			}
		case x.isNum:
			return -1
		default:
			return 1
		}
	}
	switch {
	case len(pa) < len(pb):
		return -1
	case len(pa) > len(pb):
		return 1
	}
	return 0
}

// parentAddress strips the last symbol run: 1b3 -> 1b, 1b -> 1, 1 -> "",
// L2a -> L2, L2 -> "" (the box prefix is not a parent).
func parentAddress(a, prefix string) string {
	body := strings.TrimPrefix(a, prefix)
	parts := splitAddress(body)
	if len(parts) <= 1 {
		return ""
	}
	last := parts[len(parts)-1]
	cut := 0
	if last.isNum {
		cut = len(strconv.Itoa(last.num))
	} else {
		cut = len(last.str)
	}
	return prefix + body[:len(body)-cut]
}
