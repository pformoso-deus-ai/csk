package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctor_AllCleanAfterAdd(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	out, err := runCSK(t, "--global", "doctor")
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if !strings.Contains(out, "ok") {
		t.Errorf("expected 'ok', got %q", out)
	}
}

func TestDoctor_DetectsMissingCache(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	// Wipe the cache; lockfile + manifest still reference it.
	if err := os.RemoveAll(filepath.Join(home, ".claude", "skills-cache", "handoff")); err != nil {
		t.Fatal(err)
	}
	out, _ := runCSK(t, "--global", "doctor")
	if !strings.Contains(out, "cache dir missing") {
		t.Errorf("expected 'cache dir missing' diagnosis, got %q", out)
	}
}

func TestDoctor_DetectsMissingJunction(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(home, ".claude", "skills", "handoff")); err != nil {
		t.Fatal(err)
	}
	out, _ := runCSK(t, "--global", "doctor")
	if !strings.Contains(out, "junction missing") {
		t.Errorf("expected 'junction missing', got %q", out)
	}
}

func TestDoctor_DetectsManifestOnly(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	// Wipe the lockfile only.
	if err := os.Remove(filepath.Join(home, ".claude", "skills.lock")); err != nil {
		t.Fatal(err)
	}
	out, _ := runCSK(t, "--global", "doctor")
	if !strings.Contains(out, "not in lockfile") {
		t.Errorf("expected manifest-only diagnosis, got %q", out)
	}
}
