package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pformoso/csk/internal/cache"
	"github.com/pformoso/csk/internal/lockfile"
	"github.com/pformoso/csk/internal/procguard"
)

func newInstallCmd() *cobra.Command {
	var discard bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Reconcile cache + junctions to match the lockfile",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveScope()
			if err != nil {
				return userErr(err)
			}
			if _, err := os.Stat(s.LockfilePath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return userErr(fmt.Errorf("no lockfile at %s — nothing to install", s.LockfilePath))
				}
				return envErr(err)
			}

			g, err := procguard.Acquire(s.ProcLockPath)
			if err != nil {
				return classifyProcguard(err)
			}
			defer g.Unlock()

			lf, err := lockfile.Load(s.LockfilePath)
			if err != nil {
				return userErr(err)
			}

			if err := os.MkdirAll(s.CacheDir, 0o755); err != nil {
				return envErr(err)
			}
			if err := os.MkdirAll(s.SkillsDir, 0o755); err != nil {
				return envErr(err)
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			for _, e := range lf.Skills {
				plan := cache.Plan{
					Name:   e.Name,
					Source: e.Source,
					Ref:    e.Ref,
					Commit: e.Commit,
					Subdir: e.Subdir,
				}
				if err := cache.Reconcile(ctx, s, plan, discard); err != nil {
					return envErr(fmt.Errorf("install %s: %w", e.Name, err))
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "csk: installed %d skill(s)\n", len(lf.Skills))
			return nil
		},
	}
	cmd.Flags().BoolVar(&discard, "discard", false, "discard local changes in cache directories")
	return cmd
}

func newSyncCmd() *cobra.Command {
	c := newInstallCmd()
	c.Use = "sync"
	c.Short = "Alias for `install` — reconcile to the lockfile"
	return c
}
