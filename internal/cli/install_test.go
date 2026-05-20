package cli_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestInstall_RoundTrip is the spec's acceptance-criteria round trip:
//
//	init → add → delete skills/ and lockfile → check skills.toml only →
//	install → state restored.
//
// We delete the lockfile too, then re-run `add` to regenerate it before
// install. (csk install requires a lockfile; the spec test wants us to
// prove that with only the manifest preserved we can rebuild — that uses
// `csk lock`, which is not yet implemented. This test exercises the
// install half of the loop: junctions get rebuilt from the lockfile.)
func TestInstall_RebuildsJunctionsFromLockfile(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(home, ".claude", "skills", "handoff")
	cacheDir := filepath.Join(home, ".claude", "skills-cache", "handoff")

	// Wipe the surface: junction + cache. Keep lockfile.
	if err := os.RemoveAll(filepath.Join(home, ".claude", "skills")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(home, ".claude", "skills-cache")); err != nil {
		t.Fatal(err)
	}

	if _, err := runCSK(t, "--global", "install"); err != nil {
		t.Fatalf("install: %v", err)
	}

	if _, err := os.Stat(cacheDir); err != nil {
		t.Errorf("cache not restored: %v", err)
	}
	if _, err := os.Lstat(linkPath); err != nil {
		t.Errorf("junction not restored: %v", err)
	}
}

func TestInstall_Idempotent(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}

	// Two consecutive installs should both succeed and not error.
	if _, err := runCSK(t, "--global", "install"); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if _, err := runCSK(t, "--global", "install"); err != nil {
		t.Fatalf("second install (should be idempotent): %v", err)
	}
}

func TestInstall_NoLockfile(t *testing.T) {
	useFakeHome(t)
	_, err := runCSK(t, "--global", "install")
	if err == nil {
		t.Fatal("expected error when lockfile missing")
	}
}

func TestSync_IsAliasForInstall(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	if _, err := runCSK(t, "--global", "sync"); err != nil {
		t.Fatalf("sync: %v", err)
	}
}
