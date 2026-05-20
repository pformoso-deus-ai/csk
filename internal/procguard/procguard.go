// Package procguard provides the advisory dot-lock that prevents two csk
// invocations from mutating the same scope concurrently.
//
// We use a separate file (skills.toml.lock) rather than locking the manifest
// itself so that the manifest stays atomically writable.
package procguard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

// ErrBusy means another csk process holds the scope lock.
var ErrBusy = errors.New("another csk operation is in progress for this scope")

// Guard wraps an acquired advisory lock. Release with Unlock().
type Guard struct {
	fl *flock.Flock
}

// Acquire takes an exclusive non-blocking lock at lockPath. If the lock is
// held by another process, returns ErrBusy.
func Acquire(lockPath string) (*Guard, error) {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, err
	}
	fl := flock.New(lockPath)
	ok, err := fl.TryLock()
	if err != nil {
		return nil, fmt.Errorf("acquire lock %s: %w", lockPath, err)
	}
	if !ok {
		return nil, ErrBusy
	}
	return &Guard{fl: fl}, nil
}

// Unlock releases the lock. Safe to call multiple times.
func (g *Guard) Unlock() error {
	if g == nil || g.fl == nil {
		return nil
	}
	return g.fl.Unlock()
}
