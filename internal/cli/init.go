package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pformoso/csk/internal/lockfile"
	"github.com/pformoso/csk/internal/manifest"
	"github.com/pformoso/csk/internal/procguard"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create an empty manifest (and lockfile) in the current scope",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveScope()
			if err != nil {
				return userErr(err)
			}
			if err := os.MkdirAll(s.Root, 0o755); err != nil {
				return envErr(err)
			}
			g, err := procguard.Acquire(s.ProcLockPath)
			if err != nil {
				return classifyProcguard(err)
			}
			defer g.Unlock()

			if _, err := os.Stat(s.ManifestPath); err == nil {
				return userErr(fmt.Errorf("manifest already exists at %s", s.ManifestPath))
			}

			for _, d := range []string{s.CacheDir, s.SkillsDir} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					return envErr(err)
				}
			}
			if err := manifest.New().Save(s.ManifestPath); err != nil {
				return envErr(err)
			}
			if err := lockfile.New().Save(s.LockfilePath); err != nil {
				return envErr(err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "csk: initialized %s scope at %s\n", s.Kind, s.Root)
			return nil
		},
	}
}
