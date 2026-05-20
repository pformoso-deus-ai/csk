package cli

import "github.com/spf13/cobra"

func newAddCmd() *cobra.Command {
	var (
		name   string
		ref    string
		subdir string
		force  bool
	)
	cmd := &cobra.Command{
		Use:   "add <source>",
		Short: "Clone a git source and register it as a skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := args[0]
			s, err := resolveScope()
			if err != nil {
				return err
			}
			_ = s
			_ = source
			// TODO(v1):
			//   1. Acquire procguard lock.
			//   2. Load (or create) manifest.
			//   3. Infer name: --name > SKILL.md frontmatter > repo basename.
			//   4. Refuse if name exists with a different source unless --force.
			//   5. cache.Resolve(...) to clone + get the SHA.
			//   6. Validate SKILL.md exists at target path.
			//   7. Upsert manifest entry. Upsert lockfile entry.
			//   8. cache.Reconcile to create the junction.
			//   9. Persist manifest + lockfile.
			return errNotImplemented
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "override the install name (default: SKILL.md name, falling back to repo basename)")
	cmd.Flags().StringVar(&ref, "ref", "", "branch, tag, or commit to pin (default: main)")
	cmd.Flags().StringVar(&subdir, "subdir", "", "subdirectory inside the repo to expose")
	cmd.Flags().BoolVar(&force, "force", false, "allow replacing an existing entry with a different source")
	return cmd
}
