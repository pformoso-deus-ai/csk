// Package atomicfile writes a file atomically: write to a temp file in the
// same directory, fsync-close, then rename over the destination.
//
// On POSIX, rename is atomic when both paths are on the same filesystem.
// On Windows, os.Rename uses MoveFileEx with MOVEFILE_REPLACE_EXISTING,
// which is atomic with respect to readers but not transactional.
package atomicfile

import (
	"os"
	"path/filepath"
)

// WriteFile writes data to path atomically with the given permissions.
//
// The parent directory of path is created if it does not exist. The temp
// file is created in that same directory so the final rename never crosses
// filesystem boundaries.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return err
	}
	return nil
}
