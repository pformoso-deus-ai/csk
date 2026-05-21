package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pformoso-deus-ai/csk/internal/lockfile"
)

func TestList_EmptyShowsHeaderOnly(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	out, err := runCSK(t, "--global", "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "STATE") {
		t.Errorf("expected table header, got %q", out)
	}
}

func TestList_AfterAddShowsClean(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	out, err := runCSK(t, "--global", "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out, "handoff") {
		t.Errorf("expected 'handoff' in list output, got %q", out)
	}
	if !strings.Contains(out, "clean") {
		t.Errorf("expected state 'clean', got %q", out)
	}
}

func TestList_ShowsMissingState(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	// Wipe just the cache dir to simulate a missing skill.
	if err := os.RemoveAll(filepath.Join(home, ".claude", "skills-cache", "handoff")); err != nil {
		t.Fatal(err)
	}
	out, _ := runCSK(t, "--global", "list")
	if !strings.Contains(out, "missing") {
		t.Errorf("expected 'missing' state, got %q", out)
	}
}

func TestList_ShowsUnlinkedState(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	// Remove just the junction; cache stays.
	if err := os.RemoveAll(filepath.Join(home, ".claude", "skills", "handoff")); err != nil {
		t.Fatal(err)
	}
	out, _ := runCSK(t, "--global", "list")
	if !strings.Contains(out, "unlinked") {
		t.Errorf("expected 'unlinked' state, got %q", out)
	}
}

func TestList_ShowsManifestOnly(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	// Wipe just the lockfile so manifest entry has no lockfile counterpart.
	if err := os.Remove(filepath.Join(home, ".claude", "skills.lock")); err != nil {
		t.Fatal(err)
	}
	out, _ := runCSK(t, "--global", "list")
	if !strings.Contains(out, "manifest-only") {
		t.Errorf("expected 'manifest-only' state, got %q", out)
	}
}

func TestList_ShowsDrifted(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	// Advance the cache's HEAD past the locked commit by creating a new
	// commit inside the cache dir.
	commitToFixtureRepo(t, filepath.Join(home, ".claude", "skills-cache", "handoff"), "DRIFT.md", "drifted")

	out, _ := runCSK(t, "--global", "list")
	if !strings.Contains(out, "drifted") {
		t.Errorf("expected 'drifted' state, got %q", out)
	}
}

func TestList_ShowsErrorWhenCacheNotRepo(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	// Break the cache by deleting the .git directory — HeadCommit will fail.
	if err := os.RemoveAll(filepath.Join(home, ".claude", "skills-cache", "handoff", ".git")); err != nil {
		t.Fatal(err)
	}
	out, _ := runCSK(t, "--global", "list")
	if !strings.Contains(out, "error") {
		t.Errorf("expected 'error' state, got %q", out)
	}
}

func TestList_MalformedManifestSurfacesError(t *testing.T) {
	home := useFakeHome(t)
	// Hand-write a manifest with bad syntax.
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "skills.toml"), []byte("not = a = b ="), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCSK(t, "--global", "list"); err == nil {
		t.Error("expected list to fail on malformed manifest")
	}
}

func TestList_ShowsLockOnly(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}

	// Inject a lockfile-only entry directly.
	lockPath := filepath.Join(home, ".claude", "skills.lock")
	lf, _ := lockfile.Load(lockPath)
	lf.Upsert(lockfile.Entry{
		Name:   "phantom",
		Source: "https://example.com/p.git",
		Ref:    "main",
		Commit: "0000000000000000000000000000000000000000",
	})
	if err := lf.Save(lockPath); err != nil {
		t.Fatal(err)
	}

	out, _ := runCSK(t, "--global", "list")
	if !strings.Contains(out, "lock-only") {
		t.Errorf("expected 'lock-only' state, got %q", out)
	}
}
