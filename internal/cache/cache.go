// Package cache orchestrates the per-skill clone under <scope>/skills-cache/<name>
// and pairs it with the right junction/symlink under <scope>/skills/<name>.
//
// The cache directory is the source of truth for installed bits; the skills/
// directory is just the surface Claude Code reads.
package cache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pformoso-deus-ai/csk/internal/gitx"
	"github.com/pformoso-deus-ai/csk/internal/link"
	"github.com/pformoso-deus-ai/csk/internal/scope"
)

// Plan describes the desired state for one skill.
type Plan struct {
	Name   string // install name = junction basename
	Source string // git URL
	Ref    string // branch/tag/commit
	Commit string // resolved SHA (empty for fresh add — Resolve will fill it)
	Subdir string // optional subdir inside the repo
}

// CacheDir returns the per-skill cache path under the scope.
func CacheDir(s *scope.Scope, name string) string {
	return filepath.Join(s.CacheDir, name)
}

// LinkTarget returns the absolute path the junction/symlink should point at.
// If Subdir is set, the target is <cache-dir>/<subdir>.
func LinkTarget(s *scope.Scope, p Plan) string {
	dir := CacheDir(s, p.Name)
	if p.Subdir == "" {
		return dir
	}
	return filepath.Join(dir, p.Subdir)
}

// LinkPath returns <scope>/skills/<name>.
func LinkPath(s *scope.Scope, name string) string {
	return filepath.Join(s.SkillsDir, name)
}

// Reconcile brings one skill's cache + link to the state described by p.
// It assumes p.Commit is set (resolved). For installing from a manifest where
// the commit isn't known yet, call Resolve first.
//
// Behavior:
//
//   - If cache dir is missing → clone, then checkout p.Commit.
//   - If cache dir exists, dirty, and !discard → return an error.
//   - If cache dir exists at a different commit → fetch + checkout.
//   - Refresh the link.
func Reconcile(ctx context.Context, s *scope.Scope, p Plan, discard bool) error {
	if p.Commit == "" {
		return errors.New("cache.Reconcile: Plan.Commit is required (call Resolve first)")
	}
	dir := CacheDir(s, p.Name)

	// 1. ensure clone exists at the right commit
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return err
		}
		if err := gitx.Clone(ctx, p.Source, dir); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		dirty, err := gitx.IsDirty(ctx, dir)
		if err != nil {
			return err
		}
		if dirty {
			if !discard {
				return fmt.Errorf("skill %q: cache has uncommitted changes (rerun with --discard to overwrite)", p.Name)
			}
			if err := gitx.HardReset(ctx, dir); err != nil {
				return err
			}
		}
		head, err := gitx.HeadCommit(ctx, dir)
		if err != nil {
			return err
		}
		if head != p.Commit {
			if err := gitx.Fetch(ctx, dir); err != nil {
				return err
			}
		}
	}
	if err := gitx.Checkout(ctx, dir, p.Commit); err != nil {
		return err
	}

	// 2. ensure subdir exists if specified
	target := LinkTarget(s, p)
	if _, err := os.Stat(target); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("skill %q: subdir %q does not exist at commit %s", p.Name, p.Subdir, p.Commit)
	} else if err != nil {
		return err
	}

	// 3. ensure link
	linkPath := LinkPath(s, p.Name)
	if err := link.Ensure(target, linkPath); err != nil {
		return err
	}
	return nil
}

// Resolve runs `git fetch` + `git rev-parse` against the source to turn a ref
// into a commit SHA. If the cache is fresh, it is cloned. Returns the SHA.
func Resolve(ctx context.Context, s *scope.Scope, p Plan) (string, error) {
	dir := CacheDir(s, p.Name)
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return "", err
		}
		if err := gitx.Clone(ctx, p.Source, dir); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	} else {
		if err := gitx.Fetch(ctx, dir); err != nil {
			return "", err
		}
	}
	return gitx.ResolveRef(ctx, dir, p.Ref)
}
