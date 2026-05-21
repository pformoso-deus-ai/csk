// Package registry consumes the Skill Central index.json published by
// csk-registry on GitHub Pages.
//
// The index is small enough to fetch in full (kilobytes), so we don't bother
// with a smarter protocol. We cache the parsed JSON to disk with a 1-hour
// TTL; commands offer --refresh to bypass the cache.
package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// DefaultURL points at the published index for the official registry.
	// Override via the CSK_REGISTRY_URL env var.
	DefaultURL = "https://pformoso-deus-ai.github.io/csk-registry/index.json"

	// DefaultTTL is how long a cached index is considered fresh.
	DefaultTTL = 1 * time.Hour

	cacheFile = "registry.json"
)

// Index mirrors the JSON document produced by build_index.py.
type Index struct {
	Version    int               `json:"version"`
	Generated  string            `json:"generated"`
	Categories map[string]string `json:"categories"`
	Skills     []Skill           `json:"skills"`
}

// Skill is one entry in the registry.
type Skill struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name,omitempty"`
	Description string   `json:"description"`
	Source      string   `json:"source"`
	Subdir      string   `json:"subdir,omitempty"`
	DefaultRef  string   `json:"default_ref,omitempty"`
	License     string   `json:"license"`
	Homepage    string   `json:"homepage,omitempty"`
	Maintainer  string   `json:"maintainer"`
	Categories  []string `json:"categories"`
	Tags        []string `json:"tags,omitempty"`
	Added       string   `json:"added,omitempty"`
	Updated     string   `json:"updated,omitempty"`
}

// Client fetches and caches the index.
type Client struct {
	URL       string
	HTTP      *http.Client
	CacheFile string
	TTL       time.Duration
	Now       func() time.Time
}

// New returns a Client wired against the official index, honoring the
// CSK_REGISTRY_URL and CSK_REGISTRY_CACHE_DIR env vars for overrides.
func New() *Client {
	url := DefaultURL
	if v := os.Getenv("CSK_REGISTRY_URL"); v != "" {
		url = v
	}
	cacheDir := ""
	if v := os.Getenv("CSK_REGISTRY_CACHE_DIR"); v != "" {
		cacheDir = v
	} else if d, err := os.UserCacheDir(); err == nil {
		cacheDir = filepath.Join(d, "csk")
	}
	return &Client{
		URL:       url,
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		CacheFile: filepath.Join(cacheDir, cacheFile),
		TTL:       DefaultTTL,
		Now:       time.Now,
	}
}

// Fetch returns the index. If force is false and the cache file exists and is
// younger than c.TTL, the cached copy is returned without touching the network.
// On a network failure we fall back to a (possibly stale) cache if one exists.
func (c *Client) Fetch(ctx context.Context, force bool) (*Index, error) {
	if !force {
		if idx, ok := c.readCacheIfFresh(); ok {
			return idx, nil
		}
	}

	idx, err := c.fetchHTTP(ctx)
	if err != nil {
		if idx2 := c.readCacheRaw(); idx2 != nil {
			return idx2, nil
		}
		return nil, err
	}
	_ = c.writeCache(idx) // best effort
	return idx, nil
}

func (c *Client) fetchHTTP(ctx context.Context) (*Index, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", c.URL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("fetch %s: %s: %s", c.URL, resp.Status, strings.TrimSpace(string(body)))
	}
	var idx Index
	if err := json.NewDecoder(resp.Body).Decode(&idx); err != nil {
		return nil, fmt.Errorf("decode index: %w", err)
	}
	return &idx, nil
}

func (c *Client) readCacheIfFresh() (*Index, bool) {
	fi, err := os.Stat(c.CacheFile)
	if err != nil {
		return nil, false
	}
	if c.Now().Sub(fi.ModTime()) > c.TTL {
		return nil, false
	}
	idx := c.readCacheRaw()
	return idx, idx != nil
}

func (c *Client) readCacheRaw() *Index {
	data, err := os.ReadFile(c.CacheFile)
	if err != nil {
		return nil
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil
	}
	return &idx
}

func (c *Client) writeCache(idx *Index) error {
	if err := os.MkdirAll(filepath.Dir(c.CacheFile), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.CacheFile, data, 0o644)
}

// Find returns the skill named name (case-sensitive), or nil.
func (idx *Index) Find(name string) *Skill {
	for i := range idx.Skills {
		if idx.Skills[i].Name == name {
			return &idx.Skills[i]
		}
	}
	return nil
}

// SearchResult pairs a matched skill with a relevance score and a short
// rationale ("name match", "tag match", etc.).
type SearchResult struct {
	Skill *Skill
	Score int
	Why   string
}

// Search returns matches against the query, ranked by relevance. An empty
// query returns every skill, sorted alphabetically by name.
//
// Scoring (higher beats lower):
//
//	100 — exact name match (case-insensitive)
//	 80 — name has query as prefix
//	 60 — name contains query
//	 40 — exact tag match
//	 30 — tag contains query
//	 20 — description contains query
//	 10 — maintainer contains query
func (idx *Index) Search(query string) []SearchResult {
	if query == "" {
		out := make([]SearchResult, len(idx.Skills))
		for i := range idx.Skills {
			out[i] = SearchResult{Skill: &idx.Skills[i], Score: 0, Why: ""}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Skill.Name < out[j].Skill.Name })
		return out
	}
	q := strings.ToLower(query)
	var out []SearchResult
	for i := range idx.Skills {
		s := &idx.Skills[i]
		score, why := scoreSkill(s, q)
		if score > 0 {
			out = append(out, SearchResult{Skill: s, Score: score, Why: why})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Skill.Name < out[j].Skill.Name
	})
	return out
}

func scoreSkill(s *Skill, q string) (int, string) {
	name := strings.ToLower(s.Name)
	switch {
	case name == q:
		return 100, "name match"
	case strings.HasPrefix(name, q):
		return 80, "name prefix"
	case strings.Contains(name, q):
		return 60, "name contains"
	}
	for _, t := range s.Tags {
		lt := strings.ToLower(t)
		if lt == q {
			return 40, "tag match: " + t
		}
		if strings.Contains(lt, q) {
			return 30, "tag contains: " + t
		}
	}
	if strings.Contains(strings.ToLower(s.Description), q) {
		return 20, "description contains"
	}
	if strings.Contains(strings.ToLower(s.Maintainer), q) {
		return 10, "maintainer contains"
	}
	return 0, ""
}

// LooksLikeRegistryName returns true if arg is a bare registry name rather
// than a URL or local path. Caller-side helper so commands can decide whether
// to consult the registry.
func LooksLikeRegistryName(arg string) bool {
	if arg == "" {
		return false
	}
	if strings.Contains(arg, "://") {
		return false
	}
	if strings.HasPrefix(arg, "git@") {
		return false
	}
	if strings.HasPrefix(arg, "/") || strings.HasPrefix(arg, ".") {
		return false
	}
	// Windows absolute path like "C:\..."
	if len(arg) >= 2 && arg[1] == ':' {
		return false
	}
	return true
}

// ErrNotFound is returned by helpers that resolve a name in the index when
// no such entry exists.
var ErrNotFound = errors.New("not in registry")
