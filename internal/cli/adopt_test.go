package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pformoso-deus-ai/csk/internal/exitcode"
	"github.com/pformoso-deus-ai/csk/internal/lockfile"
	"github.com/pformoso-deus-ai/csk/internal/manifest"
)

func TestAdopt_HappyPathSwapsToJunction(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")

	// Pre-install the skill by hand (regular directory, content matches HEAD).
	existing := filepath.Join(home, ".claude", "skills", "handoff")
	handInstall(t, repo, existing)
	fi, _ := os.Lstat(existing)
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Fatal("pre-condition: expected a regular directory, got a symlink")
	}

	if _, err := runCSK(t, "--global", "adopt", "handoff", "--source", repo); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	// After adopt: skills/handoff is a junction, cache populated, registered.
	fi2, err := os.Lstat(existing)
	if err != nil {
		t.Fatal(err)
	}
	if fi2.Mode()&os.ModeSymlink == 0 && fi2.Mode()&os.ModeIrregular == 0 {
		t.Errorf("expected junction at %s, mode=%v", existing, fi2.Mode())
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills-cache", "handoff")); err != nil {
		t.Errorf("expected cache dir: %v", err)
	}
	mf, _ := manifest.Load(filepath.Join(home, ".claude", "skills.toml"))
	if _, ok := mf.Skills["handoff"]; !ok {
		t.Error("manifest missing handoff after adopt")
	}
	lf, _ := lockfile.Load(filepath.Join(home, ".claude", "skills.lock"))
	if lf.Find("handoff") == nil {
		t.Error("lockfile missing handoff after adopt")
	}
}

func TestAdopt_RefusesDivergentWithoutForce(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	existing := filepath.Join(home, ".claude", "skills", "handoff")
	handInstall(t, repo, existing)

	// Introduce divergence: extra file present locally.
	if err := os.WriteFile(filepath.Join(existing, "EXTRA.md"), []byte("local-only"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runCSK(t, "--global", "adopt", "handoff", "--source", repo)
	if err == nil {
		t.Fatal("expected adopt to refuse divergent content")
	}
	if got := exitcode.From(err); got != exitcode.UserErr {
		t.Errorf("exit=%d, want %d", got, exitcode.UserErr)
	}
	// Existing directory still intact.
	if _, err := os.Stat(filepath.Join(existing, "EXTRA.md")); err != nil {
		t.Errorf("expected EXTRA.md still in place: %v", err)
	}
	_ = out
}

func TestAdopt_ForceRequiresYes(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	existing := filepath.Join(home, ".claude", "skills", "handoff")
	handInstall(t, repo, existing)
	if err := os.WriteFile(filepath.Join(existing, "EXTRA.md"), []byte("local-only"), 0o644); err != nil {
		t.Fatal(err)
	}

	// --force without --yes should bail.
	_, err := runCSK(t, "--global", "adopt", "handoff", "--source", repo, "--force")
	if err == nil {
		t.Fatal("expected --force-without-yes to fail")
	}
}

func TestAdopt_ForceYesOverwrites(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	existing := filepath.Join(home, ".claude", "skills", "handoff")
	handInstall(t, repo, existing)
	if err := os.WriteFile(filepath.Join(existing, "EXTRA.md"), []byte("local-only"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runCSK(t, "--global", "adopt", "handoff", "--source", repo, "--force", "--yes")
	if err != nil {
		t.Fatalf("adopt --force --yes: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "differ") {
		t.Errorf("expected warning about diverging files, got %q", out)
	}
	// After adopt, EXTRA.md is gone (we swapped to the source content).
	if _, err := os.Stat(filepath.Join(existing, "EXTRA.md")); !os.IsNotExist(err) {
		t.Errorf("expected EXTRA.md gone after adopt, err=%v", err)
	}
}

func TestAdopt_RefusesWhenAlreadyManaged(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	// Now skills/handoff is a junction. Adopt should refuse.
	_, err := runCSK(t, "--global", "adopt", "handoff", "--source", repo)
	if err == nil {
		t.Fatal("expected adopt to refuse a managed link")
	}
}

func TestAdopt_RequiresExistingSkillDir(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	_, err := runCSK(t, "--global", "adopt", "handoff", "--source", repo)
	if err == nil {
		t.Fatal("expected adopt to refuse with no existing dir")
	}
}
