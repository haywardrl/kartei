package slipbox

import (
	"sort"
	"strings"
)

// Search matches titles first, then bodies, case-insensitively. Damaged and
// guest notes are included so nothing is hidden.
func (v *Vault) Search(query string) []*Note {
	v.mu.RLock()
	defer v.mu.RUnlock()
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	var titles, bodies []*Note
	for _, n := range v.notes {
		switch {
		case strings.Contains(strings.ToLower(n.Title), q):
			titles = append(titles, n)
		case strings.Contains(strings.ToLower(n.Body), q):
			bodies = append(bodies, n)
		}
	}
	byTitle := func(list []*Note) {
		sort.Slice(list, func(i, j int) bool { return strings.ToLower(list[i].Title) < strings.ToLower(list[j].Title) })
	}
	byTitle(titles)
	byTitle(bodies)
	return append(titles, bodies...)
}
