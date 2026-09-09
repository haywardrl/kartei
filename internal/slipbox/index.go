package slipbox

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// indexFile is the contents card, written beside the notes: the whole tree as
// plain markdown, readable in any editor and enough to rebuild the sidecar's
// tree if it is ever lost. The app writes it; nobody should edit it.
const indexFile = "_index.md"

// writeIndex renders the contents file; caller holds the lock.
func (v *Vault) writeIndex() error {
	var b bytes.Buffer
	b.WriteString("# Kartei contents\n\nWritten by Kartei after every change. Read it anywhere; edit the cards, not this file.\n")
	for _, box := range v.state.Settings.Boxes {
		fmt.Fprintf(&b, "\n## %s (%s)\n", box.Name, box.ID)
		for _, d := range v.drawers {
			if d.Box != box.ID || d.Unsorted {
				continue
			}
			fmt.Fprintf(&b, "\n### Drawer %s\n", d.Label)
			for _, id := range d.IDs {
				n := v.notes[id]
				depth := len(splitAddress(strings.TrimPrefix(v.tree.addr[id], box.Prefix))) - 1
				fmt.Fprintf(&b, "%s- %s [[%s|%s]]\n", strings.Repeat("  ", depth), v.tree.addr[id], id, n.Title)
			}
		}
	}
	if unfiled := v.unfiled(); len(unfiled) > 0 {
		b.WriteString("\n## On the desk, not yet filed\n\n")
		for _, n := range unfiled {
			fmt.Fprintf(&b, "- [[%s|%s]]\n", n.ID, n.Title)
		}
	}
	if len(v.state.Register) > 0 {
		b.WriteString("\n## Register\n\n")
		reg := cloneSlice(v.state.Register)
		sort.Slice(reg, func(i, j int) bool { return strings.ToLower(reg[i].Term) < strings.ToLower(reg[j].Term) })
		for _, e := range reg {
			var addrs []string
			for _, id := range e.Targets {
				if a := v.tree.addr[id]; a != "" {
					addrs = append(addrs, a)
				}
			}
			line := "- " + e.Term + ": " + strings.Join(addrs, ", ")
			if e.Note != "" {
				line += " — " + e.Note
			}
			b.WriteString(line + "\n")
		}
	}
	path := filepath.Join(v.root, indexFile)
	v.recent[path] = time.Now()
	return WriteFileAtomic(v.root, path, b.Bytes(), 0o644)
}

var (
	indexBoxRe  = regexp.MustCompile(`^## .* \(([a-z0-9-]+)\)\s*$`)
	indexCardRe = regexp.MustCompile(`^(\s*)- (\S+) \[\[(\d{8}T\d{6})(?:\|[^\]]*)?\]\]`)
)

// recoverTree rebuilds parent pointers from the contents file. Addresses
// encode the tree: a card's parent is the card whose address is its own
// with the last symbol run removed, and sibling order follows address order.
func recoverTree(root string, boxes []Box) (map[ID]TreeEntry, string) {
	data, err := os.ReadFile(filepath.Join(root, indexFile))
	if err != nil {
		return nil, ""
	}
	prefixOf := map[string]string{}
	for _, b := range boxes {
		prefixOf[b.ID] = b.Prefix
	}
	type card struct {
		id   ID
		addr string
		box  string
	}
	var cards []card
	byAddr := map[string]ID{}
	box := ""
	for _, line := range strings.Split(string(data), "\n") {
		if m := indexBoxRe.FindStringSubmatch(line); m != nil {
			if _, known := prefixOf[m[1]]; known {
				box = m[1]
			} else {
				box = ""
			}
			continue
		}
		if strings.HasPrefix(line, "## ") {
			box = ""
			continue
		}
		if box == "" {
			continue
		}
		if m := indexCardRe.FindStringSubmatch(line); m != nil {
			cards = append(cards, card{id: ID(m[3]), addr: m[2], box: box})
			byAddr[box+"/"+m[2]] = ID(m[3])
		}
	}
	if len(cards) == 0 {
		return nil, ""
	}
	sort.Slice(cards, func(i, j int) bool {
		if cards[i].box != cards[j].box {
			return cards[i].box < cards[j].box
		}
		return CompareAddress(cards[i].addr, cards[j].addr) < 0
	})
	tree := map[ID]TreeEntry{}
	order := map[string]int{} // parent key -> next order
	for _, c := range cards {
		pa := parentAddress(c.addr, prefixOf[c.box])
		key := c.box + "/" + pa
		order[key]++
		if pa == "" {
			tree[c.id] = TreeEntry{Parent: "", Order: order[key], Box: c.box}
			continue
		}
		if pid, ok := byAddr[key]; ok {
			tree[c.id] = TreeEntry{Parent: pid, Order: order[key]}
		} else {
			tree[c.id] = TreeEntry{Parent: "", Order: order[c.box+"/"], Box: c.box}
		}
	}
	return tree, fmt.Sprintf("the sidecar had no tree; %d cards were refiled from %s", len(tree), indexFile)
}
