package slipbox

import (
	"regexp"
	"strings"
)

// Link is one [[wikilink]] as written in a body.
type Link struct {
	Raw      string `json:"raw"`      // "[[20260201T101500|alias]]"
	Target   string `json:"target"`   // "20260201T101500" as written
	Alias    string `json:"alias"`    // "" if none
	Resolved ID     `json:"resolved"` // "" if unresolved
}

var (
	linkRe       = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
	inlineCodeRe = regexp.MustCompile("`[^`\n]*`")
)

// ExtractLinks finds wikilinks in a body, skipping fenced code blocks and
// inline code spans. Resolution happens later, in the vault.
func ExtractLinks(body string) []Link {
	var links []Link
	inFence := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		clean := inlineCodeRe.ReplaceAllString(line, "")
		for _, m := range linkRe.FindAllStringSubmatch(clean, -1) {
			links = append(links, Link{
				Raw:    m[0],
				Target: strings.TrimSpace(m[1]),
				Alias:  strings.TrimSpace(m[2]),
			})
		}
	}
	return links
}

// LinkText is how the editor should write a link: by ID, with the title as a
// display alias, so the link survives any rename.
func LinkText(n *Note) string {
	return "[[" + string(n.ID) + "|" + n.Title + "]]"
}
