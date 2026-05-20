package cli

import "github.com/spf13/cobra"

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose cache/junction/lockfile drift (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveScope()
			if err != nil {
				return err
			}
			_ = s
			// TODO(v1) — checks:
			//   (a) every lockfile entry has a cache dir at the right commit
			//   (b) every cache dir has a corresponding junction
			//   (c) every junction's target exists
			//   (d) every manifest entry has a lockfile entry
			//   (e) SKILL.md name vs manifest key mismatch (warn only)
			return errNotImplemented
		},
	}
}
