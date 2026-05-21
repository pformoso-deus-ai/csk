package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pformoso-deus-ai/csk/internal/cache"
	"github.com/pformoso-deus-ai/csk/internal/lockfile"
	"github.com/pformoso-deus-ai/csk/internal/manifest"
	"github.com/pformoso-deus-ai/csk/internal/procguard"
)

func newLockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lock",
		Short: "Re-resolve the manifest and rewrite the lockfile (no install)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveScope()
			if err != nil {
				return userErr(err)
			}
			if _, err := os.Stat(s.ManifestPath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return userErr(fmt.Errorf("no manifest at %s — run `csk init` first", s.ManifestPath))
				}
				return envErr(err)
			}

			g, err := procguard.Acquire(s.ProcLockPath)
			if err != nil {
				return classifyProcguard(err)
			}
			defer g.Unlock()

			mf, err := manifest.Load(s.ManifestPath)
			if err != nil {
				return userErr(err)
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			if err := os.MkdirAll(s.CacheDir, 0o755); err != nil {
				return envErr(err)
			}

			// Build a fresh lockfile from the manifest. Drop any old lockfile
			// entries that no longer have a manifest counterpart.
			out := lockfile.New()
			for name, e := range mf.Skills {
				ref := e.RefOrDefault()
				plan := cache.Plan{Name: name, Source: e.Source, Ref: ref, Subdir: e.Subdir}
				commit, err := cache.Resolve(ctx, s, plan)
				if err != nil {
					return envErr(fmt.Errorf("lock %s: %w", name, err))
				}
				out.Upsert(lockfile.Entry{
					Name:   name,
					Source: e.Source,
					Ref:    ref,
					Commit: commit,
					Subdir: e.Subdir,
				})
			}
			if err := out.Save(s.LockfilePath); err != nil {
				return envErr(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "csk: locked %d skill(s)\n", len(out.Skills))
			return nil
		},
	}
}
