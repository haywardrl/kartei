package slipbox

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "note.md")
	if err := WriteFileAtomic(dir, target, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(dir, target, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "two" {
		t.Fatalf("content = %q", got)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}

func TestWriteFileAtomicFailureLeavesOriginal(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "note.md")
	if err := os.WriteFile(target, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	if err := WriteFileAtomic(dir, target, []byte("replacement"), 0o644); err == nil {
		t.Fatal("expected an error writing into a read-only directory")
	}
	os.Chmod(dir, 0o755)
	got, _ := os.ReadFile(target)
	if string(got) != "original" {
		t.Fatalf("original was modified: %q", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("unexpected files after failure: %v", entries)
	}
}

func TestWriteFileAtomicRefusesOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "escaped.md")
	err := WriteFileAtomic(root, filepath.Join(root, "..", filepath.Base(filepath.Dir(outside)), "escaped.md"), []byte("x"), 0o644)
	if !errors.Is(err, ErrOutsideVault) {
		t.Fatalf("want ErrOutsideVault, got %v", err)
	}
	if err := WriteFileAtomic(root, outside, []byte("x"), 0o644); !errors.Is(err, ErrOutsideVault) {
		t.Fatalf("absolute path outside root: want ErrOutsideVault, got %v", err)
	}
	if _, err := os.Stat(outside); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("file was written outside the vault")
	}
}
