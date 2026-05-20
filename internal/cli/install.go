package cli

import "github.com/spf13/cobra"

func newInstallCmd() *cobra.Command {
	var discard bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Reconcile cache + junctions to match the lockfile",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveScope()
			if err != nil {
				return err
			}
			_ = s
			_ = discard
			// TODO(v1):
			//   1. Acquire procguard lock.
			//   2. Load lockfile (error if missing).
			//   3. For each entry: cache.Reconcile(ctx, s, Plan{...}, discard).
			//   4. Idempotent: re-running produces no changes.
			return errNotImplemented
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
