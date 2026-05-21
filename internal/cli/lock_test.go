package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pformoso/csk/internal/lockfile"
)

// TestLock_RebuildsFromManifestOnly is the full round-trip from the spec
// acceptance criteria:
//
//	init → add → preserve only skills.toml → lock → install → state restored
//
// The manifest is the single source of truth handed to a new machine; lock
// regenerates the lockfile from it, then install rebuilds cache + junctions.
func TestLock_RebuildsFromManifestOnly(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}

	lockPath := filepath.Join(home, ".claude", "skills.lock")
	cacheDir := filepath.Join(home, ".claude", "skills-cache")
	skillsDir := filepath.Join(home, ".claude", "skills")

	// Wipe everything except the manifest.
	for _, p := range []string{lockPath, cacheDir, skillsDir} {
		if err := os.RemoveAll(p); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := runCSK(t, "--global", "lock"); err != nil {
		t.Fatalf("lock: %v", err)
	}

	lf, err := lockfile.Load(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if lf.Find("handoff") == nil {
		t.Errorf("expected handoff in regenerated lockfile")
	}

	if _, err := runCSK(t, "--global", "install"); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "handoff")); err != nil {
		t.Errorf("expected cache after install: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(skillsDir, "handoff")); err != nil {
		t.Errorf("expected junction after install: %v", err)
	}
}

func TestLock_DropsOrphanedLockfileEntries(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}

	// Manually inject a fake lockfile entry that has no manifest counterpart.
	lockPath := filepath.Join(home, ".claude", "skills.lock")
	lf, _ := lockfile.Load(lockPath)
	lf.Upsert(lockfile.Entry{Name: "ghost", Source: "https://example.com/ghost.git", Ref: "main", Commit: "deadbeef" + "deadbeef" + "deadbeef" + "deadbeef" + "deadbeef"})
	if err := lf.Save(lockPath); err != nil {
		t.Fatal(err)
	}

	if _, err := runCSK(t, "--global", "lock"); err != nil {
		t.Fatalf("lock: %v", err)
	}
	lf2, _ := lockfile.Load(lockPath)
	if lf2.Find("ghost") != nil {
		t.Error("expected ghost to be dropped after re-lock")
	}
	if lf2.Find("handoff") == nil {
		t.Error("expected handoff to survive re-lock")
	}
}
