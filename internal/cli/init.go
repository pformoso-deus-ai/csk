package cli

import "github.com/spf13/cobra"

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create an empty manifest (and lockfile) in the current scope",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveScope()
			if err != nil {
				return err
			}
			_ = s
			// TODO(v1):
			//   1. Acquire procguard lock at s.ProcLockPath.
			//   2. Refuse if s.ManifestPath already exists.
			//   3. Write manifest.New() to s.ManifestPath.
			//   4. Write lockfile.New() to s.LockfilePath.
			//   5. Create s.CacheDir and s.SkillsDir.
			return errNotImplemented
		},
	}
}
