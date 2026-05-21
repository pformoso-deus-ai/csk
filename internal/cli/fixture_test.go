package cli_test

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// makeFixtureRepo creates a local git repo at <basedir>/<repoName>/ containing
// a SKILL.md with the given skillName in its frontmatter. Returns the absolute
// repo path, suitable to pass as a `csk add <source>` argument.
//
// If subdirPath is non-empty, the SKILL.md is placed inside that subdirectory
// instead of at the repo root.
func makeFixtureRepo(t *testing.T, basedir, repoName, skillName, subdirPath string) string {
	t.Helper()
	repo := filepath.Join(basedir, repoName)
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	skillDir := repo
	if subdirPath != "" {
		skillDir = filepath.Join(repo, subdirPath)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		// Avoid inheriting the developer's commit signing config.
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

	content := fmt.Sprintf("---\nname: %s\ndescription: test skill\n---\n\n# %s\n", skillName, skillName)
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "initial")
	return repo
}

// commitToFixtureRepo adds a new file and creates a follow-up commit on the
// fixture repo's current branch. Used to simulate upstream movement for
// update tests.
func commitToFixtureRepo(t *testing.T, repo, filename, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, filename), []byte(content), 0o644); err != nil {
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
	run("add", filename)
	run("commit", "-q", "-m", "follow-up")
}

// handInstall copies repoSrc's working tree (excluding .git) into installDest.
// Used by adopt tests to set up the "skill is already installed by hand"
// precondition: a regular directory at <skills>/<name>/ with a SKILL.md.
func handInstall(t *testing.T, repoSrc, installDest string) {
	t.Helper()
	if err := os.MkdirAll(installDest, 0o755); err != nil {
		t.Fatal(err)
	}
	err := filepath.WalkDir(repoSrc, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(repoSrc, p)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return os.MkdirAll(filepath.Join(installDest, rel), 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(installDest, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
