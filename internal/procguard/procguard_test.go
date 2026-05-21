package procguard

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAcquire_Success(t *testing.T) {
	p := filepath.Join(t.TempDir(), "lock")
	g, err := Acquire(p)
	if err != nil {
		t.Fatal(err)
	}
	if g == nil {
		t.Fatal("Acquire returned nil guard with no error")
	}
	if err := g.Unlock(); err != nil {
		t.Errorf("Unlock: %v", err)
	}
}

func TestAcquire_ErrBusyWhenHeld(t *testing.T) {
	p := filepath.Join(t.TempDir(), "lock")
	g1, err := Acquire(p)
	if err != nil {
		t.Fatal(err)
	}
	defer g1.Unlock()

	// Second acquisition of the same lock should fail with ErrBusy.
	_, err = Acquire(p)
	if !errors.Is(err, ErrBusy) {
		t.Errorf("expected ErrBusy, got %v", err)
	}
}

func TestAcquire_ReleasableThenReacquirable(t *testing.T) {
	p := filepath.Join(t.TempDir(), "lock")
	g1, err := Acquire(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := g1.Unlock(); err != nil {
		t.Fatal(err)
	}
	g2, err := Acquire(p)
	if err != nil {
		t.Fatalf("expected re-acquisition after Unlock, got %v", err)
	}
	_ = g2.Unlock()
}

func TestUnlock_NilSafe(t *testing.T) {
	var g *Guard
	if err := g.Unlock(); err != nil {
		t.Errorf("Unlock on nil guard should be nil, got %v", err)
	}
}

func TestAcquire_CreatesParentDir(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nested", "deeper", "lock")
	g, err := Acquire(p)
	if err != nil {
		t.Fatalf("Acquire should mkdir parents: %v", err)
	}
	_ = g.Unlock()
}

func TestAcquire_ParentIsAFileErrors(t *testing.T) {
	dir := t.TempDir()
	// Place a regular file where the parent dir of the lock would be.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(filepath.Join(blocker, "lock")); err == nil {
		t.Error("expected error when parent path is a regular file")
	}
}
