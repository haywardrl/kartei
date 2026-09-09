// Package model is the JSON shape of the vault as any front end sees it. The
// localhost server and the Wails bridge both hand out exactly these types,
// so a UI written against one transport works against the other.
package model

import (
	"regexp"
	"strings"
	"time"

	"github.com/haywardrl/kartei/internal/slipbox"
)

// Note is the summary of one card, enough to draw it anywhere in the room.
type Note struct {
	ID       slipbox.ID   `json:"id"`
	Title    string       `json:"title"`
	Slug     string       `json:"slug"`
	Path     string       `json:"path"`
	Address  string       `json:"address"`
	Box      string       `json:"box"`
	Filed    bool         `json:"filed"`
	Parent   slipbox.ID   `json:"parent"`
	Children []slipbox.ID `json:"children"`
	Links    []slipbox.ID `json:"links"`
	Tags     []string     `json:"tags"`
	Created  time.Time    `json:"created"`
	Guest    bool         `json:"guest"`
	Damaged  bool         `json:"damaged"`
	Excerpt  string       `json:"excerpt"`
}

// Vault is the whole room in one object.
type Vault struct {
	Root       string                  `json:"root"`
	Notes      []Note                  `json:"notes"`
	Drawers    []slipbox.Drawer        `json:"drawers"`
	Desk       []slipbox.Placement     `json:"desk"`
	Boards     []slipbox.Board         `json:"boards"`
	Settings   slipbox.Settings        `json:"settings"`
	Warnings   []slipbox.Warning       `json:"warnings"`
	Stage      string                  `json:"stage"`
	Rediscover slipbox.ID              `json:"rediscover"`
	Unfiled    []slipbox.ID            `json:"unfiled"`
	Boxes      []slipbox.Box           `json:"boxes"`
	Register   []slipbox.RegisterEntry `json:"register"`
	Prefs      map[string]any          `json:"prefs"`
	Incomplete bool                    `json:"incomplete"`
}

// Detail is one card with its body and everything linked to it.
type Detail struct {
	Note
	Body      string         `json:"body"`
	ModTime   time.Time      `json:"modTime"`
	Links     []slipbox.Link `json:"links"`
	Backlinks []slipbox.ID   `json:"backlinks"`
	CrossRefs []slipbox.ID   `json:"crossrefs"`
	Extra     map[string]any `json:"extra"`
}

// Conflict is what a save returns when the file changed underneath it.
type Conflict struct {
	Error    string `json:"error"`
	Conflict bool   `json:"conflict"`
	Disk     struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	} `json:"disk"`
}

// wikilinkRe matches [[target]] and [[target|alias]] so an excerpt can show
// the alias (or target) rather than the raw link.
var wikilinkRe = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)

func excerpt(body string) string {
	body = strings.TrimSpace(body)
	if i := strings.IndexByte(body, '\n'); i >= 0 {
		body = body[:i]
	}
	body = wikilinkRe.ReplaceAllStringFunc(body, func(m string) string {
		parts := wikilinkRe.FindStringSubmatch(m)
		if parts[2] != "" {
			return parts[2]
		}
		return parts[1]
	})
	if len(body) > 120 {
		body = body[:117] + "..."
	}
	return body
}

// IDs extracts the IDs of a note list, never nil.
func IDs(notes []*slipbox.Note) []slipbox.ID {
	out := make([]slipbox.ID, len(notes))
	for i, n := range notes {
		out[i] = n.ID
	}
	return out
}

// Summary builds a Note from an engine note.
func Summary(v *slipbox.Vault, n *slipbox.Note) Note {
	var parent slipbox.ID
	if p, ok := v.Parent(n.ID); ok {
		parent = p.ID
	}
	tags := n.Tags
	if tags == nil {
		tags = []string{}
	}
	links := []slipbox.ID{}
	for _, l := range n.Links {
		if l.Resolved != "" && l.Resolved != n.ID {
			links = append(links, l.Resolved)
		}
	}
	return Note{
		ID: n.ID, Title: n.Title, Slug: n.Slug, Path: n.Path, Address: v.Address(n.ID), Box: v.BoxOf(n.ID), Filed: v.Filed(n.ID),
		Parent: parent, Children: IDs(v.Children(n.ID)), Links: links, Tags: tags, Created: n.Created,
		Guest: n.Guest, Damaged: n.Damaged, Excerpt: excerpt(n.Body),
	}
}

// NewDetail builds a Detail from an engine note.
func NewDetail(v *slipbox.Vault, n *slipbox.Note) Detail {
	links := n.Links
	if links == nil {
		links = []slipbox.Link{}
	}
	return Detail{
		Note:      Summary(v, n),
		Body:      n.Body,
		ModTime:   n.ModTime,
		Links:     links,
		Backlinks: IDs(v.Backlinks(n.ID)),
		CrossRefs: IDs(v.CrossRefs(n.ID)),
		Extra:     n.Extra,
	}
}

// Build renders the whole vault.
func Build(v *slipbox.Vault) Vault {
	notes := v.Notes()
	out := Vault{
		Root:       v.Root(),
		Notes:      make([]Note, 0, len(notes)),
		Drawers:    v.Drawers(),
		Desk:       v.Desk(),
		Boards:     v.BoardsList(),
		Settings:   v.Settings(),
		Warnings:   v.Warnings(),
		Stage:      v.Stage(),
		Rediscover: v.Rediscover(time.Now()),
		Unfiled:    IDs(v.Unfiled()),
		Boxes:      v.Boxes(),
		Register:   v.Register(),
		Prefs:      v.Prefs(),
		Incomplete: v.Incomplete(),
	}
	for _, n := range notes {
		out.Notes = append(out.Notes, Summary(v, n))
	}
	return out
}

// Summaries renders a note list, never nil.
func Summaries(v *slipbox.Vault, notes []*slipbox.Note) []Note {
	out := make([]Note, 0, len(notes))
	for _, n := range notes {
		out = append(out, Summary(v, n))
	}
	return out
}
