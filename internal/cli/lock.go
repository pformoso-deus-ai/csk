package cli

import "github.com/spf13/cobra"

func newLockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lock",
		Short: "Re-resolve the manifest and rewrite the lockfile (no install)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveScope()
			if err != nil {
				return err
			}
			_ = s
			// TODO(v1):
			//   1. Acquire procguard lock.
			//   2. Load manifest.
			//   3. For each entry: cache.Resolve(...) (clone if needed).
			//   4. Rewrite lockfile from scratch.
			//   5. Do NOT touch the junction layer.
			return errNotImplemented
		},
	}
}
