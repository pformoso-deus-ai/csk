package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/pformoso/csk/internal/cache"
	"github.com/pformoso/csk/internal/lockfile"
	"github.com/pformoso/csk/internal/manifest"
	"github.com/pformoso/csk/internal/procguard"
)

func newUpdateCmd() *cobra.Command {
	var discard bool
	cmd := &cobra.Command{
		Use:   "update [name ...]",
		Short: "Re-resolve refs to latest commits, update lockfile, sync cache",
		Args:  cobra.ArbitraryArgs,
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
			lf, err := loadLockfileOrEmpty(s.LockfilePath)
			if err != nil {
				return userErr(err)
			}

			// Decide which skills to update.
			var targets []string
			if len(args) == 0 {
				for k := range mf.Skills {
					targets = append(targets, k)
				}
				sort.Strings(targets)
			} else {
				for _, name := range args {
					if _, ok := mf.Skills[name]; !ok {
						return userErr(fmt.Errorf("skill %q is not in the manifest", name))
					}
					targets = append(targets, name)
				}
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			if err := os.MkdirAll(s.CacheDir, 0o755); err != nil {
				return envErr(err)
			}
			if err := os.MkdirAll(s.SkillsDir, 0o755); err != nil {
				return envErr(err)
			}

			updated := 0
			for _, name := range targets {
				e := mf.Skills[name]
				ref := e.RefOrDefault()
				plan := cache.Plan{Name: name, Source: e.Source, Ref: ref, Subdir: e.Subdir}
				commit, err := cache.Resolve(ctx, s, plan)
				if err != nil {
					return envErr(fmt.Errorf("update %s: resolve: %w", name, err))
				}
				plan.Commit = commit

				oldCommit := ""
				if existing := lf.Find(name); existing != nil {
					oldCommit = existing.Commit
				}

				if err := cache.Reconcile(ctx, s, plan, discard); err != nil {
					return envErr(fmt.Errorf("update %s: reconcile: %w", name, err))
				}
				lf.Upsert(lockfile.Entry{
					Name:   name,
					Source: e.Source,
					Ref:    ref,
					Commit: commit,
					Subdir: e.Subdir,
				})

				if oldCommit == commit {
					fmt.Fprintf(cmd.OutOrStdout(), "csk: %s up to date (%s)\n", name, shortSHA(commit))
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "csk: %s %s → %s\n", name, shortSHA(oldCommit), shortSHA(commit))
				}
				updated++
			}
			if err := lf.Save(s.LockfilePath); err != nil {
				return envErr(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "csk: updated %d skill(s)\n", updated)
			return nil
		},
	}
	cmd.Flags().BoolVar(&discard, "discard", false, "discard local changes in cache directories")
	return cmd
}
