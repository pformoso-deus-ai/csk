package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newFakeIndex() *Index {
	return &Index{
		Version:   1,
		Generated: "2026-05-21T00:00:00Z",
		Categories: map[string]string{
			"workflow":           "workflow stuff",
			"context-management": "context",
		},
		Skills: []Skill{
			{
				Name:        "handoff",
				Description: "Compress a session into a handoff.",
				Source:      "https://github.com/x/handoff.git",
				Subdir:      "handoff",
				License:     "MIT",
				Maintainer:  "pablo",
				Categories:  []string{"workflow", "context-management"},
				Tags:        []string{"session", "resume"},
			},
			{
				Name:        "code-review",
				Description: "Reviews a diff for security issues.",
				Source:      "https://github.com/x/code-review.git",
				License:     "Apache-2.0",
				Maintainer:  "alice",
				Categories:  []string{"workflow"},
				Tags:        []string{"review", "security"},
			},
		},
	}
}

func serveIndex(t *testing.T, idx *Index) *httptest.Server {
	t.Helper()
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
	return srv
}

func newTestClient(t *testing.T, srvURL string) *Client {
	t.Helper()
	tmp := t.TempDir()
	return &Client{
		URL:       srvURL + "/index.json",
		HTTP:      &http.Client{Timeout: 5 * time.Second},
		CacheFile: filepath.Join(tmp, "registry.json"),
		TTL:       DefaultTTL,
		Now:       time.Now,
	}
}

func TestFetch_LiveAndCached(t *testing.T) {
	idx := newFakeIndex()
	srv := serveIndex(t, idx)

	c := newTestClient(t, srv.URL)
	got, err := c.Fetch(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Skills) != 2 {
		t.Errorf("got %d skills, want 2", len(got.Skills))
	}
	// Cache file exists.
	if _, err := os.Stat(c.CacheFile); err != nil {
		t.Errorf("expected cache written: %v", err)
	}
	// Second fetch within TTL returns from cache without hitting network.
	// We can prove this by closing the server first.
	srv.Close()
	got2, err := c.Fetch(context.Background(), false)
	if err != nil {
		t.Fatalf("cached fetch should succeed after server down: %v", err)
	}
	if len(got2.Skills) != 2 {
		t.Errorf("cached fetch lost skills")
	}
}

func TestFetch_StaleCacheRefreshesFromNetwork(t *testing.T) {
	idx := newFakeIndex()
	srv := serveIndex(t, idx)

	c := newTestClient(t, srv.URL)
	c.TTL = 1 * time.Millisecond
	if _, err := c.Fetch(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	// Force the cache mtime into the past.
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(c.CacheFile, past, past); err != nil {
		t.Fatal(err)
	}
	got, err := c.Fetch(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Skills) != 2 {
		t.Errorf("refresh lost skills")
	}
}

func TestFetch_FallsBackToCacheOnHTTPError(t *testing.T) {
	idx := newFakeIndex()
	srv := serveIndex(t, idx)
	c := newTestClient(t, srv.URL)
	if _, err := c.Fetch(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	srv.Close()
	// Force fresh fetch by setting --force-like behavior; even then we
	// should fall back to the stale cache rather than erroring.
	got, err := c.Fetch(context.Background(), true)
	if err != nil {
		t.Fatalf("expected stale-cache fallback after network failure: %v", err)
	}
	if len(got.Skills) != 2 {
		t.Errorf("fallback lost skills")
	}
}

func TestFetch_NoCacheNoNetworkErrors(t *testing.T) {
	c := newTestClient(t, "http://127.0.0.1:0") // unreachable
	if _, err := c.Fetch(context.Background(), false); err == nil {
		t.Error("expected error when no cache and unreachable network")
	}
}

func TestFind(t *testing.T) {
	idx := newFakeIndex()
	if idx.Find("handoff") == nil {
		t.Error("expected to find handoff")
	}
	if idx.Find("nope") != nil {
		t.Error("expected nil for unknown name")
	}
}

func TestSearch_EmptyReturnsAllSorted(t *testing.T) {
	idx := newFakeIndex()
	out := idx.Search("")
	if len(out) != 2 {
		t.Fatalf("got %d, want 2", len(out))
	}
	if out[0].Skill.Name != "code-review" || out[1].Skill.Name != "handoff" {
		t.Errorf("not sorted alphabetically: %s, %s", out[0].Skill.Name, out[1].Skill.Name)
	}
}

func TestSearch_ScoresAndOrders(t *testing.T) {
	idx := newFakeIndex()
	cases := []struct {
		query string
		first string
	}{
		{"handoff", "handoff"},     // exact name match wins
		{"hand", "handoff"},        // name prefix
		{"review", "code-review"},  // name contains
		{"security", "code-review"},// tag match
		{"session", "handoff"},     // tag match
		{"diff", "code-review"},    // description contains
		{"alice", "code-review"},   // maintainer
	}
	for _, c := range cases {
		out := idx.Search(c.query)
		if len(out) == 0 {
			t.Errorf("query %q returned 0 results", c.query)
			continue
		}
		if out[0].Skill.Name != c.first {
			t.Errorf("query %q: first = %q, want %q", c.query, out[0].Skill.Name, c.first)
		}
	}
}

func TestSearch_NoMatches(t *testing.T) {
	idx := newFakeIndex()
	out := idx.Search("zzzzzzzz")
	if len(out) != 0 {
		t.Errorf("expected 0 matches, got %d", len(out))
	}
}

func TestLooksLikeRegistryName(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"handoff", true},
		{"code-review", true},
		{"", false},
		{"https://github.com/x/y.git", false},
		{"git@github.com:x/y.git", false},
		{"/local/path", false},
		{"./local", false},
		{"C:\\some\\path", false},
	}
	for _, c := range cases {
		if got := LooksLikeRegistryName(c.in); got != c.want {
			t.Errorf("LooksLikeRegistryName(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
