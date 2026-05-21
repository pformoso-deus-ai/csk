package gitx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// makeRepo creates a small git repo at <dir> with one commit on `main`, then
// returns the dir. Used as both a clone source and a target for fetch tests.
func makeRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
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
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "README")
	run("commit", "-q", "-m", "initial")
}

func addCommit(t *testing.T, dir, file, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", dir, "add", file)
	cmd.Env = append(os.Environ(),
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "-C", dir, "commit", "-q", "-m", "more")
	cmd.Env = append(os.Environ(),
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
}

func TestClone_LocalSource(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	makeRepo(t, src)
	dst := filepath.Join(t.TempDir(), "dst")
	if err := Clone(context.Background(), src, dst); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".git")); err != nil {
		t.Errorf("expected .git in clone target: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "README")); err != nil {
		t.Errorf("expected README copied: %v", err)
	}
}

func TestResolveRef_OriginPreferredForBranch(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	makeRepo(t, src)
	dst := filepath.Join(t.TempDir(), "dst")
	if err := Clone(context.Background(), src, dst); err != nil {
		t.Fatal(err)
	}
	sha, err := ResolveRef(context.Background(), dst, "main")
	if err != nil {
		t.Fatalf("ResolveRef: %v", err)
	}
	if len(sha) != 40 {
		t.Errorf("expected 40-char SHA, got %q", sha)
	}
}

func TestResolveRef_FallbackToTag(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	makeRepo(t, src)
	// Tag inside the source.
	tagCmd := exec.Command("git", "-C", src, "tag", "v1")
	if out, err := tagCmd.CombinedOutput(); err != nil {
		t.Fatalf("tag: %v\n%s", err, out)
	}

	dst := filepath.Join(t.TempDir(), "dst")
	if err := Clone(context.Background(), src, dst); err != nil {
		t.Fatal(err)
	}
	sha, err := ResolveRef(context.Background(), dst, "v1")
	if err != nil {
		t.Fatalf("ResolveRef(tag): %v", err)
	}
	if len(sha) != 40 {
		t.Errorf("expected 40-char SHA for tag, got %q", sha)
	}
}

func TestResolveRef_UnknownRefErrors(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	makeRepo(t, src)
	dst := filepath.Join(t.TempDir(), "dst")
	if err := Clone(context.Background(), src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveRef(context.Background(), dst, "nope-zzz"); err == nil {
		t.Error("expected error for unknown ref")
	}
}

func TestFetchAndAdvance(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	makeRepo(t, src)
	dst := filepath.Join(t.TempDir(), "dst")
	if err := Clone(context.Background(), src, dst); err != nil {
		t.Fatal(err)
	}
	before, _ := ResolveRef(context.Background(), dst, "main")
	addCommit(t, src, "two.txt", "second")
	if err := Fetch(context.Background(), dst); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	after, _ := ResolveRef(context.Background(), dst, "main")
	if before == after {
		t.Errorf("ref did not advance after fetch: %s", before)
	}
}

func TestCheckoutAndHead(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	makeRepo(t, src)
	dst := filepath.Join(t.TempDir(), "dst")
	if err := Clone(context.Background(), src, dst); err != nil {
		t.Fatal(err)
	}
	sha, _ := ResolveRef(context.Background(), dst, "main")
	if err := Checkout(context.Background(), dst, sha); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	head, err := HeadCommit(context.Background(), dst)
	if err != nil {
		t.Fatal(err)
	}
	if head != sha {
		t.Errorf("HeadCommit = %s, want %s", head, sha)
	}
}

func TestIsDirty_CleanAndDirty(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "src")
	makeRepo(t, dir)
	dirty, err := IsDirty(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if dirty {
		t.Errorf("freshly committed repo reported dirty")
	}
	// Make the tree dirty.
	if err := os.WriteFile(filepath.Join(dir, "dirty"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirty, err = IsDirty(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !dirty {
		t.Errorf("expected dirty=true after writing untracked file")
	}
}

func TestHardReset_ClearsDirtyState(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "src")
	makeRepo(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := HardReset(context.Background(), dir); err != nil {
		t.Fatalf("HardReset: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "README"))
	if strings.TrimSpace(string(body)) != "hello" {
		t.Errorf("README not reset, got %q", body)
	}
}

func TestRun_FailingCommandReportsError(t *testing.T) {
	dir := t.TempDir()
	// Not a git repo — any command should fail.
	_, err := Default.Run(context.Background(), dir, "status")
	if err == nil {
		t.Error("expected error running git in a non-repo dir")
	}
}
