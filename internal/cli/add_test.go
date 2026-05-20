package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pformoso/csk/internal/exitcode"
	"github.com/pformoso/csk/internal/lockfile"
	"github.com/pformoso/csk/internal/manifest"
)

func TestAdd_RequiresInit(t *testing.T) {
	useFakeHome(t)
	_, err := runCSK(t, "--global", "add", "https://example.com/foo.git")
	if err == nil {
		t.Fatal("expected error when manifest is missing")
	}
	if got := exitcode.From(err); got != exitcode.UserErr {
		t.Errorf("exit = %d, want %d", got, exitcode.UserErr)
	}
}

func TestAdd_LocalFixtureBasename(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}

	// SKILL.md name MATCHES the repo basename — no rename should happen.
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")

	out, err := runCSK(t, "--global", "add", repo)
	if err != nil {
		t.Fatalf("add: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "added handoff") {
		t.Errorf("expected output to mention 'added handoff', got %q", out)
	}

	// Cache dir + junction exist.
	cacheDir := filepath.Join(home, ".claude", "skills-cache", "handoff")
	linkPath := filepath.Join(home, ".claude", "skills", "handoff")
	if _, err := os.Stat(cacheDir); err != nil {
		t.Errorf("expected cache dir: %v", err)
	}
	fi, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("expected link at %s: %v", linkPath, err)
	}
	if fi.Mode()&os.ModeSymlink == 0 && fi.Mode()&os.ModeIrregular == 0 {
		t.Errorf("expected %s to be a symlink/junction, mode=%v", linkPath, fi.Mode())
	}

	// Manifest + lockfile registered.
	mf, err := manifest.Load(filepath.Join(home, ".claude", "skills.toml"))
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := mf.Skills["handoff"]
	if !ok {
		t.Fatal("manifest missing handoff entry")
	}
	if entry.Source != repo {
		t.Errorf("entry.Source = %q, want %q", entry.Source, repo)
	}

	lf, err := lockfile.Load(filepath.Join(home, ".claude", "skills.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if lfe := lf.Find("handoff"); lfe == nil || lfe.Commit == "" {
		t.Errorf("lockfile missing handoff entry with commit, got %+v", lfe)
	}
}

func TestAdd_NameUpgradeFromSKILLMD(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}

	// SKILL.md name DIFFERS from the repo basename — install name should
	// be the SKILL.md name ("handoff"), not the basename ("handoff-skill").
	repo := makeFixtureRepo(t, t.TempDir(), "handoff-skill", "handoff", "")

	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatalf("add: %v", err)
	}

	mf, _ := manifest.Load(filepath.Join(home, ".claude", "skills.toml"))
	if _, ok := mf.Skills["handoff"]; !ok {
		t.Errorf("expected manifest key 'handoff', got %v", mapKeys(mf.Skills))
	}
	if _, ok := mf.Skills["handoff-skill"]; ok {
		t.Errorf("did not expect tentative name 'handoff-skill' in manifest")
	}

	// Cache dir should also be renamed.
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills-cache", "handoff")); err != nil {
		t.Errorf("expected cache dir at final name: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills-cache", "handoff-skill")); err == nil {
		t.Errorf("did not expect leftover cache dir at tentative name")
	}
}

func TestAdd_ExplicitNameOverridesSKILLMD(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}

	repo := makeFixtureRepo(t, t.TempDir(), "handoff-skill", "handoff", "")

	if _, err := runCSK(t, "--global", "add", repo, "--name", "myhandoff"); err != nil {
		t.Fatalf("add: %v", err)
	}

	mf, _ := manifest.Load(filepath.Join(home, ".claude", "skills.toml"))
	if _, ok := mf.Skills["myhandoff"]; !ok {
		t.Errorf("expected manifest key 'myhandoff', got %v", mapKeys(mf.Skills))
	}
}

func TestAdd_DuplicateNameDifferentSourceFails(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	repo1 := makeFixtureRepo(t, base, "handoff", "handoff", "")
	repo2 := makeFixtureRepo(t, base, "handoff-other", "handoff", "")

	if _, err := runCSK(t, "--global", "add", repo1); err != nil {
		t.Fatal(err)
	}
	_, err := runCSK(t, "--global", "add", repo2, "--name", "handoff")
	if err == nil {
		t.Fatal("expected error: same name, different source")
	}
	if got := exitcode.From(err); got != exitcode.UserErr {
		t.Errorf("exit = %d, want %d", got, exitcode.UserErr)
	}
}

func TestAdd_Subdir(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "monorepo", "handoff", "packages/handoff")

	if _, err := runCSK(t, "--global", "add", repo, "--subdir", "packages/handoff"); err != nil {
		t.Fatalf("add: %v", err)
	}

	// Junction should point at <cache>/<name>/packages/handoff, where the
	// SKILL.md actually lives. We assert by checking that SKILL.md is
	// directly visible through the link.
	linkPath := filepath.Join(home, ".claude", "skills", "handoff")
	if _, err := os.Stat(filepath.Join(linkPath, "SKILL.md")); err != nil {
		t.Errorf("expected SKILL.md visible through junction: %v", err)
	}
}

func TestAdd_SameSourceIsIdempotent(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")

	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatalf("re-add: %v", err)
	}
}

func mapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
