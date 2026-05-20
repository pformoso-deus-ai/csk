package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

func newAdoptCmd() *cobra.Command {
	var (
		source string
		ref    string
		subdir string
		yes    bool
		force  bool
	)
	cmd := &cobra.Command{
		Use:   "adopt <name>",
		Short: "Register an existing hand-installed skill into the manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if source == "" {
				return errors.New("--source is required")
			}
			name := args[0]
			s, err := resolveScope()
			if err != nil {
				return err
			}
			_ = s
			_ = name
			_ = ref
			_ = subdir
			_ = yes
			_ = force
			// TODO(v1):
			//   1. Acquire procguard lock.
			//   2. Verify <skills>/<name> exists and contains SKILL.md.
			//   3. Clone source into cache, resolve ref → SHA, checkout.
			//   4. Recursive byte-equal diff between <skills>/<name> and cache target.
			//      - Equal → swap in junction (after rmdir).
			//      - Mismatch + !force → list divergent files, return error.
			//      - Mismatch + force → swap anyway, but require --yes confirmation.
			//   5. Upsert manifest + lockfile.
			return errNotImplemented
		},
	}
	cmd.Flags().StringVar(&source, "source", "", "git URL of the skill (required)")
	cmd.Flags().StringVar(&ref, "ref", "", "branch, tag, or commit to pin (default: main)")
	cmd.Flags().StringVar(&subdir, "subdir", "", "subdirectory inside the repo")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	cmd.Flags().BoolVar(&force, "force", false, "adopt even if local content diverges from the source")
	_ = cmd.MarkFlagRequired("source")
	return cmd
}
