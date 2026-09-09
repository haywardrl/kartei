package slipbox

import (
	"os"
	"path/filepath"
	"testing"
)

// copyFixture clones a testdata vault into a temp dir so tests can mutate it.
func copyFixture(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join("testdata", name)
	dst := t.TempDir()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func openFixture(t *testing.T, name string) *Vault {
	t.Helper()
	v, err := Open(copyFixture(t, name))
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	t.Cleanup(func() { v.Close() })
	return v
}

func ids(notes []*Note) []ID {
	out := make([]ID, len(notes))
	for i, n := range notes {
		out[i] = n.ID
	}
	return out
}

func hasWarning(ws []Warning, kind string) bool {
	for _, w := range ws {
		if w.Kind == kind {
			return true
		}
	}
	return false
}
