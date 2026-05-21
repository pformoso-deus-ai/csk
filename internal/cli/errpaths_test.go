// Tests that exercise the user-error early-return branches in the commands.
// These don't need network or fixture git, just a clean fake-home + a
// malformed scope state.

package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pformoso-deus-ai/csk/internal/exitcode"
)

func TestInit_ErrorWhenManifestParentBlocked(t *testing.T) {
	home := useFakeHome(t)
	// Make ~/.claude a regular file — init's MkdirAll(scope.Root) will fail.
	if err := os.WriteFile(filepath.Join(home, ".claude"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := runCSK(t, "--global", "init")
	if err == nil {
		t.Error("expected init to fail when .claude is a regular file")
	}
}

func TestAdd_EmptySourceFails(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	_, err := runCSK(t, "--global", "add", "")
	if err == nil {
		t.Fatal("expected error for empty source")
	}
	if got := exitcode.From(err); got != exitcode.UserErr {
		t.Errorf("exit=%d, want %d", got, exitcode.UserErr)
	}
}

func TestAdd_NonGitSourceFails(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	// Path that exists but is not a git repo.
	_, err := runCSK(t, "--global", "add", t.TempDir())
	if err == nil {
		t.Error("expected error cloning a non-repo path")
	}
}

func TestUpdate_NoManifestFails(t *testing.T) {
	useFakeHome(t)
	_, err := runCSK(t, "--global", "update")
	if err == nil {
		t.Fatal("expected update to fail without init")
	}
	if got := exitcode.From(err); got != exitcode.UserErr {
		t.Errorf("exit=%d, want %d", got, exitcode.UserErr)
	}
}

func TestLock_NoManifestFails(t *testing.T) {
	useFakeHome(t)
	_, err := runCSK(t, "--global", "lock")
	if err == nil {
		t.Fatal("expected lock to fail without init")
	}
}

func TestRemove_NoManifestFails(t *testing.T) {
	useFakeHome(t)
	_, err := runCSK(t, "--global", "remove", "anything")
	if err == nil {
		t.Fatal("expected remove to fail without init")
	}
}

func TestAdopt_NoSourceFails(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	// Cobra's MarkFlagRequired catches the missing --source before our
	// RunE even fires.
	_, err := runCSK(t, "--global", "adopt", "x")
	if err == nil {
		t.Error("expected adopt to fail without --source")
	}
}

func TestAdopt_NoManifestFails(t *testing.T) {
	useFakeHome(t)
	_, err := runCSK(t, "--global", "adopt", "x", "--source", "https://example.com/x.git")
	if err == nil {
		t.Fatal("expected adopt to fail without init")
	}
}

func TestDoctor_NoManifestReportsCleanEmpty(t *testing.T) {
	useFakeHome(t)
	// No init; doctor uses loadManifestOrEmpty / loadLockfileOrEmpty so it
	// should run and report "ok" against an empty set.
	out, err := runCSK(t, "--global", "doctor")
	if err != nil {
		t.Fatalf("doctor with empty scope: %v", err)
	}
	_ = out
}

func TestAdd_AlreadyManagedJunctionRejectedByAdopt(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "h", "h", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	// Trying to adopt a name that's already in the manifest is a user error.
	_, err := runCSK(t, "--global", "adopt", "h", "--source", repo)
	if err == nil {
		t.Error("expected adopt to refuse adopting an already-registered name")
	}
}
