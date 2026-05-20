//go:build !windows

package link

import (
	"errors"
	"os"
	"path/filepath"
)

func ensure(target, linkPath string) error {
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return err
	}
	fi, err := os.Lstat(linkPath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return os.Symlink(target, linkPath)
	case err != nil:
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		current, rerr := os.Readlink(linkPath)
		if rerr != nil {
			return rerr
		}
		if current == target {
			return nil
		}
		return &ErrWouldClobber{Path: linkPath}
	}
	return &ErrWouldClobber{Path: linkPath}
}

func remove(linkPath string) error {
	fi, err := os.Lstat(linkPath)
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return errors.New("refusing to remove non-symlink: " + linkPath)
	}
	return os.Remove(linkPath)
}

func isManagedLink(linkPath, expectedTarget string) (bool, error) {
	fi, err := os.Lstat(linkPath)
	if err != nil {
		return false, err
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}
	current, err := os.Readlink(linkPath)
	if err != nil {
		return false, err
	}
	return current == expectedTarget, nil
}
