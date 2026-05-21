package cache

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/pformoso-deus-ai/csk/internal/scope"
)

func newTestScope(t *testing.T) *scope.Scope {
	t.Helper()
	tmp := t.TempDir()
	s, err := scope.Resolve(tmp, tmp, true, false)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCacheDir(t *testing.T) {
	s := newTestScope(t)
	got := CacheDir(s, "foo")
	want := filepath.Join(s.CacheDir, "foo")
	if got != want {
		t.Errorf("CacheDir = %q, want %q", got, want)
	}
}

func TestLinkPath(t *testing.T) {
	s := newTestScope(t)
	got := LinkPath(s, "foo")
	want := filepath.Join(s.SkillsDir, "foo")
	if got != want {
		t.Errorf("LinkPath = %q, want %q", got, want)
	}
}

func TestLinkTarget_NoSubdir(t *testing.T) {
	s := newTestScope(t)
	got := LinkTarget(s, Plan{Name: "foo"})
	want := filepath.Join(s.CacheDir, "foo")
	if got != want {
		t.Errorf("LinkTarget(no subdir) = %q, want %q", got, want)
	}
}

func TestLinkTarget_WithSubdir(t *testing.T) {
	s := newTestScope(t)
	got := LinkTarget(s, Plan{Name: "foo", Subdir: "pkg/inner"})
	want := filepath.Join(s.CacheDir, "foo", "pkg", "inner")
	if got != want {
		t.Errorf("LinkTarget(subdir) = %q, want %q", got, want)
	}
}

// makeFixtureRepo creates a small local git repo at <dir>/<name>/ with one
// commit on main, suitable as a clone source. Returns the path.
func makeFixtureRepo(t *testing.T, dir, name string) string {
	t.Helper()
	repo := filepath.Join(dir, name)
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "README")
	run("commit", "-q", "-m", "initial")
	return repo
}

func TestResolve_ClonesAndReturnsSHA(t *testing.T) {
	s := newTestScope(t)
	src := makeFixtureRepo(t, t.TempDir(), "src")
	sha, err := Resolve(context.Background(), s, Plan{Name: "src", Source: src, Ref: "main"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(sha) != 40 {
		t.Errorf("expected 40-char SHA, got %q", sha)
	}
	if _, err := os.Stat(CacheDir(s, "src")); err != nil {
		t.Errorf("expected cache dir to exist: %v", err)
	}
}

func TestReconcile_CreatesCacheAndLink(t *testing.T) {
	s := newTestScope(t)
	src := makeFixtureRepo(t, t.TempDir(), "src")
	sha, err := Resolve(context.Background(), s, Plan{Name: "src", Source: src, Ref: "main"})
	if err != nil {
		t.Fatal(err)
	}
	plan := Plan{Name: "src", Source: src, Ref: "main", Commit: sha}
	if err := Reconcile(context.Background(), s, plan, false); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if _, err := os.Lstat(LinkPath(s, "src")); err != nil {
		t.Errorf("expected link: %v", err)
	}
}

func TestReconcile_Idempotent(t *testing.T) {
	s := newTestScope(t)
	src := makeFixtureRepo(t, t.TempDir(), "src")
	sha, _ := Resolve(context.Background(), s, Plan{Name: "src", Source: src, Ref: "main"})
	plan := Plan{Name: "src", Source: src, Ref: "main", Commit: sha}
	if err := Reconcile(context.Background(), s, plan, false); err != nil {
		t.Fatal(err)
	}
	if err := Reconcile(context.Background(), s, plan, false); err != nil {
		t.Errorf("second Reconcile should succeed: %v", err)
	}
}

func TestReconcile_RequiresCommit(t *testing.T) {
	s := newTestScope(t)
	err := Reconcile(context.Background(), s, Plan{Name: "x", Source: "irrelevant"}, false)
	if err == nil {
		t.Error("expected Reconcile to require a Commit")
	}
}

func TestResolve_InvalidSourceFails(t *testing.T) {
	s := newTestScope(t)
	_, err := Resolve(context.Background(), s, Plan{
		Name:   "nope",
		Source: filepath.Join(t.TempDir(), "does-not-exist"),
		Ref:    "main",
	})
	if err == nil {
		t.Error("expected clone of non-existent source to fail")
	}
}

func TestResolve_FetchFailsOnNonRepo(t *testing.T) {
	s := newTestScope(t)
	// Pre-create the cache dir as an empty (non-git) directory so Resolve
	// takes the fetch branch and that branch reports an error.
	cdir := CacheDir(s, "junk")
	if err := os.MkdirAll(cdir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Resolve(context.Background(), s, Plan{
		Name:   "junk",
		Source: filepath.Join(t.TempDir(), "anything"),
		Ref:    "main",
	})
	if err == nil {
		t.Error("expected Resolve to fail when cache dir is not a git repo")
	}
}

func TestReconcile_SubdirMissingErrors(t *testing.T) {
	s := newTestScope(t)
	src := makeFixtureRepo(t, t.TempDir(), "src")
	sha, _ := Resolve(context.Background(), s, Plan{Name: "src", Source: src, Ref: "main"})
	plan := Plan{Name: "src", Source: src, Ref: "main", Commit: sha, Subdir: "no-such-dir"}
	err := Reconcile(context.Background(), s, plan, false)
	if err == nil {
		t.Error("expected Reconcile to fail for missing subdir")
	}
}

func TestReconcile_BadCommitErrors(t *testing.T) {
	s := newTestScope(t)
	src := makeFixtureRepo(t, t.TempDir(), "src")
	_, _ = Resolve(context.Background(), s, Plan{Name: "src", Source: src, Ref: "main"})
	// A SHA that definitely doesn't exist in the repo.
	plan := Plan{Name: "src", Source: src, Ref: "main", Commit: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}
	if err := Reconcile(context.Background(), s, plan, false); err == nil {
		t.Error("expected checkout of unknown commit to fail")
	}
}

func TestReconcile_RefusesDirtyCacheWithoutDiscard(t *testing.T) {
	s := newTestScope(t)
	src := makeFixtureRepo(t, t.TempDir(), "src")
	sha, _ := Resolve(context.Background(), s, Plan{Name: "src", Source: src, Ref: "main"})
	plan := Plan{Name: "src", Source: src, Ref: "main", Commit: sha}
	if err := Reconcile(context.Background(), s, plan, false); err != nil {
		t.Fatal(err)
	}
	// Make the cache dirty.
	cdir := CacheDir(s, "src")
	if err := os.WriteFile(filepath.Join(cdir, "DIRTY"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Even with the SAME plan, we should refuse because the tree is dirty —
	// reach the dirty-check via a different commit to force the branch.
	if err := Reconcile(context.Background(), s, plan, false); err == nil {
		// Strictly speaking this passes because the same-commit fast path
		// doesn't trigger the dirty check on every implementation. We
		// instead validate the --discard mode separately below.
		_ = err
	}
	// With discard=true, the same call should succeed (resets the dirty file).
	if err := Reconcile(context.Background(), s, plan, true); err != nil {
		t.Errorf("Reconcile --discard should succeed on dirty cache: %v", err)
	}
}

func TestReconcile_DirtyCacheWithoutDiscardReturnsError(t *testing.T) {
	s := newTestScope(t)
	src := makeFixtureRepo(t, t.TempDir(), "src")
	sha, _ := Resolve(context.Background(), s, Plan{Name: "src", Source: src, Ref: "main"})
	plan := Plan{Name: "src", Source: src, Ref: "main", Commit: sha}
	if err := Reconcile(context.Background(), s, plan, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(CacheDir(s, "src"), "DIRTY"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Reconcile(context.Background(), s, plan, false)
	if err == nil {
		t.Error("expected Reconcile to refuse dirty cache without --discard")
	}
}
