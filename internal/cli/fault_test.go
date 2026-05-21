// Fault-injection tests. Swap gitx.Default for a fake runner that returns
// errors from chosen git subcommands, so error-return branches inside the
// big RunE functions get exercised without relying on filesystem games.

package cli_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pformoso-deus-ai/csk/internal/exitcode"
	"github.com/pformoso-deus-ai/csk/internal/gitx"
)

type fakeRunner struct {
	delegate gitx.Runner
	failOn   string // substring; if a git command's args contain this, fail
}

func (f *fakeRunner) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if f.failOn != "" {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, f.failOn) {
			return nil, errors.New("fake: forced failure on " + f.failOn)
		}
	}
	return f.delegate.Run(ctx, dir, args...)
}

// withFakeGit replaces gitx.Default for the duration of the test. Any git
// subcommand whose argv contains failOn is rejected; everything else falls
// through to the real git binary so happy paths still work.
func withFakeGit(t *testing.T, failOn string) {
	t.Helper()
	old := gitx.Default
	gitx.Default = &fakeRunner{delegate: old, failOn: failOn}
	t.Cleanup(func() { gitx.Default = old })
}

func TestAdd_GitCloneFailureClassifiesAsEnvErr(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	withFakeGit(t, "clone")
	_, err := runCSK(t, "--global", "add", "https://example.com/x.git")
	if err == nil {
		t.Fatal("expected add to fail when clone fails")
	}
	if got := exitcode.From(err); got != exitcode.EnvErr {
		t.Errorf("exit=%d, want %d", got, exitcode.EnvErr)
	}
}

func TestInstall_CheckoutFailureSurfacesAsEnvErr(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	withFakeGit(t, "checkout")
	_, err := runCSK(t, "--global", "install")
	if err == nil {
		t.Fatal("expected install to fail when checkout fails")
	}
	if got := exitcode.From(err); got != exitcode.EnvErr {
		t.Errorf("exit=%d, want %d", got, exitcode.EnvErr)
	}
}

func TestUpdate_RevParseFailureSurfacesAsEnvErr(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	withFakeGit(t, "rev-parse")
	_, err := runCSK(t, "--global", "update", "handoff")
	if err == nil {
		t.Fatal("expected update to fail when rev-parse fails")
	}
}

func TestUpdate_FetchFailureSurfacesAsEnvErr(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	withFakeGit(t, "fetch")
	_, err := runCSK(t, "--global", "update")
	if err == nil {
		t.Fatal("expected update to fail when fetch fails")
	}
	if got := exitcode.From(err); got != exitcode.EnvErr {
		t.Errorf("exit=%d, want %d", got, exitcode.EnvErr)
	}
}

func TestLock_FetchFailureSurfacesAsEnvErr(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	if _, err := runCSK(t, "--global", "add", repo); err != nil {
		t.Fatal(err)
	}
	withFakeGit(t, "fetch")
	_, err := runCSK(t, "--global", "lock")
	if err == nil {
		t.Fatal("expected lock to fail when re-resolving fetch fails")
	}
}

func TestAdopt_CloneFailureSurfacesAsEnvErr(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	// Pre-install a hand-installed skill, then make adopt fail at clone time.
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	existing := home + "/.claude/skills/handoff"
	handInstall(t, repo, existing)
	withFakeGit(t, "clone")
	_, err := runCSK(t, "--global", "adopt", "handoff", "--source", repo)
	if err == nil {
		t.Fatal("expected adopt to fail when clone fails")
	}
}
