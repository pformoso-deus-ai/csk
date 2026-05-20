//go:build windows

package link

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func ensure(target, linkPath string) error {
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return err
	}
	fi, err := os.Lstat(linkPath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return mklinkJunction(target, linkPath)
	case err != nil:
		return err
	}
	// On Windows, Go reports a junction with ModeIrregular | ModeSymlink (it
	// is treated as a reparse point). Read the target via os.Readlink.
	if fi.Mode()&os.ModeSymlink != 0 || fi.Mode()&os.ModeIrregular != 0 {
		current, rerr := os.Readlink(linkPath)
		if rerr == nil && samePath(current, target) {
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
	if fi.Mode()&os.ModeSymlink == 0 && fi.Mode()&os.ModeIrregular == 0 {
		return errors.New("refusing to remove non-junction: " + linkPath)
	}
	// os.Remove works on junctions; rmdir would also work.
	return os.Remove(linkPath)
}

func isManagedLink(linkPath, expectedTarget string) (bool, error) {
	fi, err := os.Lstat(linkPath)
	if err != nil {
		return false, err
	}
	if fi.Mode()&os.ModeSymlink == 0 && fi.Mode()&os.ModeIrregular == 0 {
		return false, nil
	}
	current, err := os.Readlink(linkPath)
	if err != nil {
		return false, nil
	}
	return samePath(current, expectedTarget), nil
}

// mklinkJunction shells out to cmd's built-in mklink. We use /J (junction)
// rather than /D (symbolic link) because junctions do not require admin or
// developer mode.
func mklinkJunction(target, linkPath string) error {
	// cmd's mklink expects: mklink /J <link> <target>
	// Note: linkPath must not already exist for /J.
	cmd := exec.Command("cmd", "/c", "mklink", "/J", linkPath, target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mklink /J %s %s: %w: %s", linkPath, target, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func samePath(a, b string) bool {
	ap, _ := filepath.Abs(a)
	bp, _ := filepath.Abs(b)
	return strings.EqualFold(filepath.Clean(ap), filepath.Clean(bp))
}
