// End-to-end tests for csk search / csk info / csk add <name>.
//
// We spin up an httptest server that mimics the published index.json and
// point csk at it via CSK_REGISTRY_URL. Each test also redirects the
// registry cache via CSK_REGISTRY_CACHE_DIR so it never touches the real
// user-cache directory.

package cli_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pformoso-deus-ai/csk/internal/lockfile"
	"github.com/pformoso-deus-ai/csk/internal/manifest"
)

// useFakeRegistry serves a one-skill index.json pointing at the given
// fixture repo, and points csk at the server via env vars.
func useFakeRegistry(t *testing.T, sourceURL, subdir string) string {
	t.Helper()
	idx := map[string]any{
		"version":   1,
		"generated": "2026-05-21T00:00:00Z",
		"categories": map[string]string{
			"workflow":           "workflow stuff",
			"context-management": "context",
		},
		"skills": []map[string]any{
			{
				"name":        "handoff",
				"description": "Compress a Claude Code session into a handoff document.",
				"source":      sourceURL,
				"subdir":      subdir,
				"default_ref": "main",
				"license":     "MIT",
				"maintainer":  "pformoso-deus-ai",
				"categories":  []string{"workflow", "context-management"},
				"tags":        []string{"session", "resume", "claude-code"},
			},
			{
				"name":        "code-review",
				"description": "Reviews diffs for security issues.",
				"source":      "https://example.com/code-review.git",
				"license":     "MIT",
				"maintainer":  "alice",
				"categories":  []string{"workflow"},
				"tags":        []string{"review", "security"},
			},
		},
	}
	body, err := json.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/index.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("CSK_REGISTRY_URL", srv.URL+"/index.json")
	t.Setenv("CSK_REGISTRY_CACHE_DIR", t.TempDir())
	return srv.URL
}

func TestSearch_ByQuery(t *testing.T) {
	useFakeHome(t)
	useFakeRegistry(t, "https://example.com/handoff.git", "handoff")

	out, err := runCSK(t, "search", "handoff")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(out, "handoff") {
		t.Errorf("expected handoff in results, got %q", out)
	}
	if strings.Contains(out, "code-review") {
		t.Errorf("did not expect code-review in results for query 'handoff': %q", out)
	}
}

func TestSearch_NoArgsListsAll(t *testing.T) {
	useFakeHome(t)
	useFakeRegistry(t, "https://example.com/handoff.git", "handoff")
	out, err := runCSK(t, "search")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for _, want := range []string{"handoff", "code-review", "NAME"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got %q", want, out)
		}
	}
}

func TestSearch_NoMatch(t *testing.T) {
	useFakeHome(t)
	useFakeRegistry(t, "https://example.com/handoff.git", "handoff")
	out, err := runCSK(t, "search", "zzzzzz")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(out, "no matches") {
		t.Errorf("expected 'no matches' message, got %q", out)
	}
}

func TestInfo_KnownSkill(t *testing.T) {
	useFakeHome(t)
	useFakeRegistry(t, "https://example.com/handoff.git", "handoff")
	out, err := runCSK(t, "info", "handoff")
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	for _, want := range []string{"handoff", "source", "MIT", "csk add handoff"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got %q", want, out)
		}
	}
}

func TestInfo_Unknown(t *testing.T) {
	useFakeHome(t)
	useFakeRegistry(t, "https://example.com/handoff.git", "handoff")
	_, err := runCSK(t, "info", "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
}

func TestAdd_ByRegistryName(t *testing.T) {
	home := useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	// Real local git fixture (so the clone actually works), and have the
	// fake registry advertise it under the name "handoff".
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	useFakeRegistry(t, repo, "")

	out, err := runCSK(t, "--global", "add", "handoff")
	if err != nil {
		t.Fatalf("add by name: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "resolved handoff") {
		t.Errorf("expected 'resolved handoff' line, got %q", out)
	}

	mf, _ := manifest.Load(filepath.Join(home, ".claude", "skills.toml"))
	entry, ok := mf.Skills["handoff"]
	if !ok {
		t.Fatal("manifest missing handoff")
	}
	if entry.Source != repo {
		t.Errorf("entry.Source = %q, want %q", entry.Source, repo)
	}
	lf, _ := lockfile.Load(filepath.Join(home, ".claude", "skills.lock"))
	if lf.Find("handoff") == nil {
		t.Error("lockfile missing handoff")
	}
}

func TestAdd_ByUnknownRegistryName(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	useFakeRegistry(t, "https://example.com/x.git", "")
	_, err := runCSK(t, "--global", "add", "no-such-skill")
	if err == nil {
		t.Fatal("expected error for unknown registry name")
	}
}

func TestAdd_URLIsUnchanged(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	// Even with a fake registry serving "handoff", passing the URL directly
	// must skip the registry and clone the URL.
	repo := makeFixtureRepo(t, t.TempDir(), "handoff", "handoff", "")
	useFakeRegistry(t, "https://wrong.example.com/handoff.git", "handoff")
	out, err := runCSK(t, "--global", "add", repo)
	if err != nil {
		t.Fatalf("add by URL: %v\nout=%s", err, out)
	}
	if strings.Contains(out, "resolved handoff") {
		t.Errorf("URL add should NOT consult the registry, got: %q", out)
	}
}

func TestAdd_SubdirFromRegistryDefault(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	// Fixture repo has SKILL.md under subdir/, registry advertises that.
	// Without csk reading subdir from the registry, this would fail with
	// "no SKILL.md at root".
	repo := makeFixtureRepo(t, t.TempDir(), "monorepo", "handoff", "pkg/handoff")
	useFakeRegistry(t, repo, "pkg/handoff")
	out, err := runCSK(t, "--global", "add", "handoff")
	if err != nil {
		t.Fatalf("add by name with registry subdir: %v\nout=%s", err, out)
	}
	_ = out
}

func TestAdd_ExplicitFlagsOverrideRegistryDefaults(t *testing.T) {
	useFakeHome(t)
	if _, err := runCSK(t, "--global", "init"); err != nil {
		t.Fatal(err)
	}
	// Registry says subdir = "wrong"; the user's --subdir flag must win.
	repo := makeFixtureRepo(t, t.TempDir(), "monorepo", "handoff", "pkg/handoff")
	useFakeRegistry(t, repo, "wrong")
	_, err := runCSK(t, "--global", "add", "handoff", "--subdir", "pkg/handoff")
	if err != nil {
		t.Fatalf("--subdir should override registry default: %v", err)
	}
}

// Sanity check that the test fixture's index is what we think it is. Helpful
// when a search/info test fails mysteriously — this confirms the server side
// is wired right.
func TestRegistry_FixtureRoundtrip(t *testing.T) {
	useFakeHome(t)
	useFakeRegistry(t, "https://example.com/handoff.git", "handoff")
	out, err := runCSK(t, "search")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "NAME") {
		t.Fatal("expected table header")
	}
}
