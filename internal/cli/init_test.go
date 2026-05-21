package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/pformoso-deus-ai/csk/internal/cli"
	"github.com/pformoso-deus-ai/csk/internal/exitcode"
	"github.com/pformoso-deus-ai/csk/internal/lockfile"
	"github.com/pformoso-deus-ai/csk/internal/manifest"
)

// useFakeHome points HOME/USERPROFILE at a temp dir so tests don't touch
// the real ~/.claude. Also chdirs to that dir so --project tests have a
// clean starting cwd.
func useFakeHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	return tmp
}

func runCSK(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := cli.NewRootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), err
}

func TestInit_GlobalFreshScope(t *testing.T) {
	home := useFakeHome(t)

	out, err := runCSK(t, "--global", "init")
	if err != nil {
		t.Fatalf("init: %v (out=%q)", err, out)
	}

	manifestPath := filepath.Join(home, ".claude", "skills.toml")
	lockPath := filepath.Join(home, ".claude", "skills.lock")
	for _, p := range []string{manifestPath, lockPath} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s to exist: %v", p, err)
		}
	}
	for _, d := range []string{
		filepath.Join(home, ".claude", "skills-cache"),
		filepath.Join(home, ".claude", "skills"),
	} {
		fi, err := os.Stat(d)
		if err != nil {
			t.Errorf("expected dir %s: %v", d, err)
			continue
		}
		if !fi.IsDir() {
			t.Errorf("%s is not a directory", d)
		}
	}

	mf, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if mf.Version != manifest.Version {
		t.Errorf("manifest version = %d, want %d", mf.Version, manifest.Version)
	}
	if len(mf.Skills) != 0 {
		t.Errorf("expected empty manifest, got %d skills", len(mf.Skills))
	}

	lf, err := lockfile.Load(lockPath)
	if err != nil {
		t.Fatalf("load lockfile: %v", err)
	}
	if lf.Version != lockfile.Version {
		t.Errorf("lockfile version = %d, want %d", lf.Version, lockfile.Version)
	}
	if len(lf.Skills) != 0 {
		t.Errorf("expected empty lockfile, got %d skills", len(lf.Skills))
	}
}

func TestInit_RefusesExistingManifest(t *testing.T) {
	useFakeHome(t)

	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatalf("first init: %v", err)
	}
	_, err := runCSK(t, "--global", "init")
	if err == nil {
		t.Fatal("expected second init to fail")
	}
	if got := exitcode.From(err); got != exitcode.UserErr {
		t.Errorf("exit code = %d, want %d", got, exitcode.UserErr)
	}
}

func TestInit_ProjectScope(t *testing.T) {
	useFakeHome(t)
	// cwd is the fake-home tmpdir from useFakeHome.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runCSK(t, "--project", "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cwd, ".claude", "skills.toml")); err != nil {
		t.Errorf("expected project manifest: %v", err)
	}
}

func TestInit_AutoDetectsProjectWhenManifestExists(t *testing.T) {
	useFakeHome(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Pre-seed a project manifest so the scope auto-detects to project.
	projectClaude := filepath.Join(cwd, ".claude")
	if err := os.MkdirAll(projectClaude, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectClaude, "skills.toml"), []byte("version = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Now `csk init` (no --global/--project) should detect the project
	// scope and refuse, because the manifest already exists there.
	_, err = runCSK(t, "init")
	if err == nil {
		t.Fatal("expected init to refuse (project manifest exists)")
	}
	if got := exitcode.From(err); got != exitcode.UserErr {
		t.Errorf("exit code = %d, want %d", got, exitcode.UserErr)
	}
}

func TestInit_GlobalAndProjectMutuallyExclusive(t *testing.T) {
	useFakeHome(t)
	_, err := runCSK(t, "--global", "--project", "init")
	if err == nil {
		t.Fatal("expected error when both flags set")
	}
}
