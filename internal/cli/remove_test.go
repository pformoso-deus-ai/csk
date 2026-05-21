package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pformoso/csk/internal/exitcode"
	"github.com/pformoso/csk/internal/lockfile"
	"github.com/pformoso/csk/internal/manifest"
)

func TestRemove_DropsEntryAndJunction(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}

	if _, err := runCSK(t, "--global", "remove", "handoff"); err != nil {
		t.Fatalf("remove: %v", err)
	}

	// Junction is gone.
	if _, err := os.Lstat(filepath.Join(home, ".claude", "skills", "handoff")); !os.IsNotExist(err) {
		t.Errorf("expected junction removed, got err=%v", err)
	}
	// Cache dir is preserved (no --prune).
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills-cache", "handoff")); err != nil {
		t.Errorf("cache should be preserved without --prune: %v", err)
	}
	// Manifest + lockfile entries are gone.
	mf, _ := manifest.Load(filepath.Join(home, ".claude", "skills.toml"))
	if _, ok := mf.Skills["handoff"]; ok {
		t.Error("manifest still contains handoff")
	}
	lf, _ := lockfile.Load(filepath.Join(home, ".claude", "skills.lock"))
	if lf.Find("handoff") != nil {
		t.Error("lockfile still contains handoff")
	}
}

func TestRemove_PruneAlsoDeletesCache(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}

	if _, err := runCSK(t, "--global", "remove", "handoff", "--prune"); err != nil {
		t.Fatalf("remove --prune: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills-cache", "handoff")); !os.IsNotExist(err) {
		t.Errorf("expected cache pruned, got err=%v", err)
	}
}

func TestRemove_UnknownNameFails(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	_, err := runCSK(t, "--global", "remove", "nope")
	if err == nil {
		t.Fatal("expected error for unknown name")
	}
	if got := exitcode.From(err); got != exitcode.UserErr {
		t.Errorf("exit=%d, want %d", got, exitcode.UserErr)
	}
}
