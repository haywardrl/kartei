// Package slipbox is the headless engine: it owns a folder of markdown notes
// plus a JSON sidecar and exposes a queryable model. It knows nothing about
// any user interface.
package slipbox

import (
	"regexp"
	"strings"
	"time"
	"unicode"
)

// ID is a note's identity: its creation timestamp in Denote format, UTC,
// second precision. It appears in the filename and nowhere else.
type ID string

const idLayout = "20060102T150405"

var (
	idRe   = regexp.MustCompile(`^\d{8}T\d{6}$`)
	fileRe = regexp.MustCompile(`^(\d{8}T\d{6})(?:--(.*))?\.md$`)
)

// NewID builds an ID from a time.
func NewID(t time.Time) ID { return ID(t.UTC().Format(idLayout)) }

// Valid reports whether the string has the shape of an ID.
func (id ID) Valid() bool { return idRe.MatchString(string(id)) }

// Time parses the ID back into a time.
func (id ID) Time() (time.Time, error) { return time.Parse(idLayout, string(id)) }

// Next returns the ID one second later, used to resolve collisions.
func (id ID) Next() ID {
	t, err := id.Time()
	if err != nil {
		return id
	}
	return NewID(t.Add(time.Second))
}

// Filename builds the on-disk name for an ID and title.
func Filename(id ID, title string) string {
	return string(id) + "--" + Slugify(title) + ".md"
}

// ParseFilename splits a filename into ID and slug. ok is false for guests.
func ParseFilename(name string) (id ID, slug string, ok bool) {
	m := fileRe.FindStringSubmatch(name)
	if m == nil {
		return "", "", false
	}
	return ID(m[1]), m[2], true
}

// accents maps the common Latin accented letters to ASCII without pulling in
// a normalisation library.
var accents = map[rune]string{
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'æ': "ae",
	'ç': "c", 'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ì': "i", 'í': "i",
	'î': "i", 'ï': "i", 'ñ': "n", 'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o",
	'ö': "o", 'ø': "o", 'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ý': "y",
	'ÿ': "y", 'ß': "ss", 'œ': "oe",
}

// Slugify turns a title into a filename-safe slug: lowercase ASCII, runs of
// non-alphanumerics collapsed to one hyphen, at most 60 characters cut at a
// hyphen boundary, "untitled" if nothing survives.
func Slugify(s string) string {
	var b strings.Builder
	lastHyphen := true
	for _, r := range strings.ToLower(s) {
		if rep, ok := accents[r]; ok {
			b.WriteString(rep)
			lastHyphen = false
			continue
		}
		if r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			b.WriteRune(r)
			lastHyphen = false
			continue
		}
		if !lastHyphen {
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 60 {
		cut := strings.LastIndex(out[:61], "-")
		if cut <= 0 {
			cut = 60
		}
		out = strings.Trim(out[:cut], "-")
	}
	if out == "" {
		return "untitled"
	}
	return out
}
