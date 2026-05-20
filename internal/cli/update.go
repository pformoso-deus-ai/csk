package cli

import "github.com/spf13/cobra"

func newUpdateCmd() *cobra.Command {
	var discard bool
	cmd := &cobra.Command{
		Use:   "update [name ...]",
		Short: "Re-resolve refs to latest commits, update lockfile, sync cache",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveScope()
			if err != nil {
				return err
			}
			_ = s
			_ = discard
			// TODO(v1):
			//   1. Acquire procguard lock.
			//   2. Load manifest + lockfile.
			//   3. For each requested name (or all if empty):
			//        a. cache.Resolve(...) → fresh SHA.
			//        b. Update lockfile entry.
			//        c. cache.Reconcile(...).
			//   4. Persist lockfile.
			return errNotImplemented
		},
	}
	cmd.Flags().BoolVar(&discard, "discard", false, "discard local changes in cache directories")
	return cmd
}
