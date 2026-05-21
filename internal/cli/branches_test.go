// Tests targeting specific uncovered branches across the command set.

package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemove_RegularDirAtSkillsPathWarns(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	// Replace the junction with a regular directory. csk should leave it
	// alone and warn rather than rm -rf'ing user data.
	link := filepath.Join(home, ".claude", "skills", "handoff")
	if err := os.RemoveAll(link); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(link, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(link, "PRECIOUS.md"), []byte("user data"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runCSK(t, "--global", "remove", "handoff")
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !strings.Contains(out, "warning") || !strings.Contains(out, "not a csk-managed link") {
		t.Errorf("expected warning about unmanaged link, got %q", out)
	}
	// Precious file untouched.
	if _, err := os.Stat(filepath.Join(link, "PRECIOUS.md")); err != nil {
		t.Errorf("expected user data preserved: %v", err)
	}
}

func TestInstall_WithDiscardOnDirtyCache(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	// Dirty the cache.
	cache := filepath.Join(home, ".claude", "skills-cache", "handoff")
	if err := os.WriteFile(filepath.Join(cache, "DIRTY"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// install --discard should succeed and clean the cache.
	if _, err := runCSK(t, "--global", "install", "--discard"); err != nil {
		t.Fatalf("install --discard: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cache, "DIRTY")); !os.IsNotExist(err) {
		t.Errorf("expected DIRTY removed after --discard, err=%v", err)
	}
}

func TestInstall_RefusesDirtyCacheWithoutDiscard(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(home, ".claude", "skills-cache", "handoff")
	if err := os.WriteFile(filepath.Join(cache, "DIRTY"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCSK(t, "--global", "install"); err == nil {
		t.Error("expected install to refuse dirty cache without --discard")
	}
}

func TestUpdate_WithDiscardOnDirtyCache(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(home, ".claude", "skills-cache", "handoff")
	if err := os.WriteFile(filepath.Join(cache, "DIRTY"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCSK(t, "--global", "update", "handoff", "--discard"); err != nil {
		t.Fatalf("update --discard: %v", err)
	}
}

func TestDoctor_DetectsSKILLMDNameMismatch(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "h-skill", "handoff", "")
	// Use --name to override SKILL.md name promotion, creating a mismatch.
	if _, err := runCSK(t, "--global", "add", repo, "--name", "renamed"); err != nil {
		t.Fatal(err)
	}
	_ = home
	out, _ := runCSK(t, "--global", "doctor")
	if !strings.Contains(out, "SKILL.md frontmatter name") {
		t.Errorf("expected frontmatter-name-mismatch warning, got %q", out)
	}
}

func TestAdopt_WithNoSKILLMDInLocalDirFails(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	// Create skills/handoff/ as a directory WITHOUT a SKILL.md.
	dst := filepath.Join(home, ".claude", "skills", "handoff")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "OTHER.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := runCSK(t, "--global", "adopt", "handoff", "--source", repo)
	if err == nil {
		t.Error("expected adopt to refuse a dir without SKILL.md")
	}
}

func TestAdopt_WithSubdirOption(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "monorepo", "handoff", "pkg/inner")
	// Hand-install the subdir contents into skills/handoff/.
	dst := filepath.Join(home, ".claude", "skills", "handoff")
	handInstall(t, filepath.Join(repo, "pkg", "inner"), dst)
	if _, err := runCSK(t, "--global", "adopt", "handoff", "--source", repo, "--subdir", "pkg/inner"); err != nil {
		t.Fatalf("adopt with subdir: %v", err)
	}
}

func TestList_MalformedLockfileFails(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	// Corrupt the lockfile.
	if err := os.WriteFile(filepath.Join(home, ".claude", "skills.lock"), []byte("not = good = ===="), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCSK(t, "--global", "list"); err == nil {
		t.Error("expected list to fail on malformed lockfile")
	}
}

func TestDoctor_SubdirMissingReports(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "monorepo", "handoff", "pkg/inner")
	if _, err := runCSK(t, "--global", "add", repo, "--subdir", "pkg/inner"); err != nil {
		t.Fatal(err)
	}
	// Remove the subdir so the cache HEAD no longer has it.
	subdir := filepath.Join(home, ".claude", "skills-cache", "handoff", "pkg")
	if err := os.RemoveAll(subdir); err != nil {
		t.Fatal(err)
	}
	out, _ := runCSK(t, "--global", "doctor")
	if !strings.Contains(out, "subdir") {
		t.Errorf("expected doctor to report missing subdir, got %q", out)
	}
}

func TestList_AfterRemoveNoEntries(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	if _, err := runCSK(t, "--global", "remove", "handoff"); err != nil {
		t.Fatal(err)
	}
	out, err := runCSK(t, "--global", "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if strings.Contains(out, "handoff") {
		t.Errorf("did not expect handoff in list after remove, got %q", out)
	}
}
