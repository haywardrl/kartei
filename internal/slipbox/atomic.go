package slipbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrOutsideVault is returned for any write whose target is not under the
// vault root. The engine never touches a file it does not own.
var ErrOutsideVault = errors.New("refusing to write outside the vault")

// insideRoot reports whether path resolves to a location under root.
func insideRoot(root, path string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// WriteFileAtomic writes data to path by way of a temp file in the same
// directory, fsync, then rename. A crash mid-write never leaves a truncated
// target, and a failure leaves the original untouched. The target must be
// inside root.
func WriteFileAtomic(root, path string, data []byte, perm os.FileMode) (err error) {
	if !insideRoot(root, path) {
		return fmt.Errorf("atomic write %s: %w", path, ErrOutsideVault)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("atomic write %s: %w", path, err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		return fmt.Errorf("atomic write %s: %w", path, err)
	}
	if err = tmp.Sync(); err != nil {
		return fmt.Errorf("atomic write %s: %w", path, err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("atomic write %s: %w", path, err)
	}
	if err = os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("atomic write %s: %w", path, err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("atomic write %s: %w", path, err)
	}
	return nil
}
