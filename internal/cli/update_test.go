package cli_test

import (
	"path/filepath"
	"testing"

	"github.com/pformoso/csk/internal/lockfile"
)

func TestUpdate_AdvancesLockfileToNewCommit(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}

	lockPath := filepath.Join(home, ".claude", "skills.lock")
	lf, _ := lockfile.Load(lockPath)
	before := lf.Find("handoff").Commit

	commitToFixtureRepo(t, repo, "NEW.md", "second commit")

	if _, err := runCSK(t, "--global", "update", "handoff"); err != nil {
		t.Fatalf("update: %v", err)
	}
	lf2, _ := lockfile.Load(lockPath)
	after := lf2.Find("handoff").Commit
	if before == after {
		t.Errorf("expected commit to advance, both = %s", before)
	}
}

func TestUpdate_AllSkillsWhenNoArgs(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	r1 := makeFixtureRepo(t, base, "a", "a", "")
	r2 := makeFixtureRepo(t, base, "b", "b", "")
	if _, err := runCSK(t, "--global", "add", r1); err != nil {
		t.Fatal(err)
	}
	if _, err := runCSK(t, "--global", "add", r2); err != nil {
		t.Fatal(err)
	}
	out, err := runCSK(t, "--global", "update")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	// Each up-to-date line names the skill; just sanity-check both appeared.
	for _, name := range []string{"a", "b"} {
		if !contains(out, name) {
			t.Errorf("expected mention of %q in update output, got %q", name, out)
		}
	}
}

func TestUpdate_UnknownNameFails(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	_, err := runCSK(t, "--global", "update", "nope")
	if err == nil {
		t.Fatal("expected error for unknown name")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
