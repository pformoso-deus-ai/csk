package cli

import "github.com/spf13/cobra"

func newRemoveCmd() *cobra.Command {
	var pruneCache bool
	cmd := &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Remove a skill: drop the junction, optionally delete the cache",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			s, err := resolveScope()
			if err != nil {
				return err
			}
			_ = s
			_ = name
			// TODO(v1):
			//   1. Acquire procguard lock.
			//   2. Load manifest + lockfile; fail clearly if name absent.
			//   3. link.Remove(<skills>/<name>) if managed.
			//   4. If --prune, remove cache dir.
			//   5. Delete manifest + lockfile entries; persist.
			return errNotImplemented
		},
	}
	cmd.Flags().BoolVar(&pruneCache, "prune", false, "also delete the cache directory for this skill")
	return cmd
}
