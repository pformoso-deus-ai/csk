package cli_test

import (
	"fmt"
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
