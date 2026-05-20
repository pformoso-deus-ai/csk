package cli

import "github.com/spf13/cobra"

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Show installed skills, pinned commits, and drift state",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveScope()
			if err != nil {
				return err
			}
			_ = s
			// TODO(v1) — read-only. Columns:
			//   name | source | ref | commit | state
			// where state ∈ {clean, drifted, missing, manifest-only, lock-only}.
			return errNotImplemented
		},
	}
}
