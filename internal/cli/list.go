package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/pformoso/csk/internal/cache"
	"github.com/pformoso/csk/internal/gitx"
	"github.com/pformoso/csk/internal/lockfile"
	"github.com/pformoso/csk/internal/manifest"
	"github.com/pformoso/csk/internal/scope"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Show installed skills, pinned commits, and drift state",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveScope()
			if err != nil {
				return userErr(err)
			}

			mf, err := loadManifestOrEmpty(s.ManifestPath)
			if err != nil {
				return userErr(err)
			}
			lf, err := loadLockfileOrEmpty(s.LockfilePath)
			if err != nil {
				return userErr(err)
			}

			names := map[string]struct{}{}
			for k := range mf.Skills {
				names[k] = struct{}{}
			}
			for _, e := range lf.Skills {
				names[e.Name] = struct{}{}
			}
			sorted := make([]string, 0, len(names))
			for n := range names {
				sorted = append(sorted, n)
			}
			sort.Strings(sorted)

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tREF\tCOMMIT\tSTATE")
			for _, name := range sorted {
				mfe, inMf := mf.Skills[name]
				lfe := lf.Find(name)
				ref, commit, state := computeState(ctx, s, name, inMf, mfe, lfe)
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", name, ref, commit, state)
			}
			return tw.Flush()
		},
	}
}

// loadManifestOrEmpty loads the manifest, treating "missing file" as an empty
// manifest rather than an error. Used by read-only commands.
func loadManifestOrEmpty(path string) (*manifest.File, error) {
	mf, err := manifest.Load(path)
	if err == nil {
		return mf, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return manifest.New(), nil
	}
	return nil, err
}

func loadLockfileOrEmpty(path string) (*lockfile.File, error) {
	lf, err := lockfile.Load(path)
	if err == nil {
		return lf, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return lockfile.New(), nil
	}
	return nil, err
}

// computeState returns the display columns (ref, commit, state) for one skill.
//
// Possible states:
//   - "clean":         cache HEAD == lockfile commit AND junction is correct
//   - "drifted":       cache HEAD != lockfile commit (user pulled in cache)
//   - "missing":       lockfile entry but no cache dir
//   - "unlinked":      cache exists at right commit but no junction
//   - "manifest-only": in manifest but not in lockfile (run `csk lock`)
//   - "lock-only":     in lockfile but not in manifest
//   - "error":         git operation failed inside the cache dir
func computeState(ctx context.Context, s *scope.Scope, name string, inMf bool, mfe manifest.Entry, lfe *lockfile.Entry) (ref, commit, state string) {
	switch {
	case lfe == nil:
		// manifest-only
		return mfe.RefOrDefault(), "-", "manifest-only"
	case !inMf:
		return lfe.Ref, shortSHA(lfe.Commit), "lock-only"
	}

	ref = lfe.Ref
	commit = shortSHA(lfe.Commit)

	cdir := cache.CacheDir(s, name)
	if _, err := os.Stat(cdir); errors.Is(err, os.ErrNotExist) {
		return ref, commit, "missing"
	}
	head, herr := gitx.HeadCommit(ctx, cdir)
	if herr != nil {
		return ref, commit, "error"
	}
	if head != lfe.Commit {
		return ref, commit, "drifted"
	}
	if _, err := os.Lstat(cache.LinkPath(s, name)); errors.Is(err, os.ErrNotExist) {
		return ref, commit, "unlinked"
	}
	return ref, commit, "clean"
}
