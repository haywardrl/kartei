package slipbox

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Note is one card. Body is everything after the frontmatter, verbatim.
type Note struct {
	ID      ID
	Path    string // relative to the vault root
	Slug    string
	Title   string
	Created time.Time
	Tags    []string
	Body    string
	Links   []Link
	Extra   map[string]any // unknown frontmatter keys, preserved on rewrite
	Guest   bool           // no valid ID prefix
	Damaged bool           // frontmatter failed to parse; never rewritten
	Raw     []byte         // original bytes, kept for damaged notes
	ModTime time.Time      // of the file when it was last read or written
	Size    int64
}

const fence = "---\n"

// ParseNote reads a note from its bytes. relPath is relative to the vault root.
// A YAML error does not fail the parse: the note is marked Damaged and its raw
// content is kept so it is never rewritten.
func ParseNote(relPath string, data []byte) *Note {
	n := &Note{Path: relPath, Extra: map[string]any{}}
	name := filepath.Base(relPath)
	id, slug, ok := ParseFilename(name)
	if ok {
		n.ID, n.Slug = id, slug
	} else {
		n.Guest = true
		n.Slug = Slugify(strings.TrimSuffix(name, ".md"))
	}

	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	body := text
	var fm map[string]any
	if strings.HasPrefix(text, fence) {
		rest := text[len(fence):]
		end, skip := -1, 0
		switch {
		case strings.HasPrefix(rest, "---\n"):
			end, skip = 0, 4
		case strings.Contains(rest, "\n---\n"):
			end, skip = strings.Index(rest, "\n---\n"), 5
		case strings.HasSuffix(rest, "\n---"):
			end, skip = len(rest)-4, 4
		}
		if end >= 0 {
			if err := yaml.Unmarshal([]byte(rest[:end]), &fm); err != nil {
				n.Damaged = true
				n.Raw = data
				n.Body = text
				n.Title = fallbackTitle(n)
				return n
			}
			body = strings.TrimPrefix(rest[end+skip:], "\n")
		}
	}
	n.Body = body

	for k, v := range fm {
		switch k {
		case "title":
			n.Title = fmt.Sprint(v)
		case "created":
			if t, ok := v.(time.Time); ok {
				n.Created = t
			} else if t, err := time.Parse(time.RFC3339, fmt.Sprint(v)); err == nil {
				n.Created = t
			}
		case "tags":
			if list, ok := v.([]any); ok {
				for _, item := range list {
					n.Tags = append(n.Tags, fmt.Sprint(item))
				}
			} else if s, ok := v.(string); ok && s != "" {
				for _, item := range strings.Split(s, ",") {
					n.Tags = append(n.Tags, strings.TrimSpace(item))
				}
			}
		default:
			n.Extra[k] = v
		}
	}
	if n.Title == "" {
		n.Title = fallbackTitle(n)
	}
	if n.Created.IsZero() && n.ID != "" {
		if t, err := n.ID.Time(); err == nil {
			n.Created = t
		}
	}
	n.Links = ExtractLinks(n.Body)
	return n
}

func fallbackTitle(n *Note) string {
	if n.Slug == "" {
		return "Untitled"
	}
	s := strings.ReplaceAll(n.Slug, "-", " ")
	return strings.ToUpper(s[:1]) + s[1:]
}

// Marshal serialises the note. Only title, created and tags are written by the
// app; unknown keys are preserved after them. Damaged notes refuse to marshal.
func (n *Note) Marshal() ([]byte, error) {
	if n.Damaged {
		return nil, fmt.Errorf("note %s is damaged and will not be rewritten", n.Path)
	}
	var buf bytes.Buffer
	buf.WriteString(fence)
	head := struct {
		Title   string   `yaml:"title"`
		Created string   `yaml:"created"`
		Tags    []string `yaml:"tags,omitempty"`
	}{n.Title, n.Created.UTC().Format(time.RFC3339), n.Tags}
	out, err := yaml.Marshal(head)
	if err != nil {
		return nil, err
	}
	buf.Write(out)
	if len(n.Extra) > 0 {
		keys := make([]string, 0, len(n.Extra))
		for k := range n.Extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		ordered := yaml.Node{Kind: yaml.MappingNode}
		for _, k := range keys {
			var v yaml.Node
			if err := v.Encode(n.Extra[k]); err != nil {
				return nil, err
			}
			ordered.Content = append(ordered.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: k}, &v)
		}
		extra, err := yaml.Marshal(&ordered)
		if err != nil {
			return nil, err
		}
		buf.Write(extra)
	}
	buf.WriteString("---\n\n")
	buf.WriteString(n.Body)
	if !strings.HasSuffix(n.Body, "\n") {
		buf.WriteString("\n")
	}
	return buf.Bytes(), nil
}
