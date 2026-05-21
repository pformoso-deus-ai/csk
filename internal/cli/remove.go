package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pformoso/csk/internal/cache"
	"github.com/pformoso/csk/internal/link"
	"github.com/pformoso/csk/internal/manifest"
	"github.com/pformoso/csk/internal/procguard"
)

func newRemoveCmd() *cobra.Command {
	var pruneCache bool
	cmd := &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Remove a skill: drop the junction, optionally delete the cache",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if name == "" {
				return userErr(errors.New("name must not be empty"))
			}
			s, err := resolveScope()
			if err != nil {
				return userErr(err)
			}
			if _, err := os.Stat(s.ManifestPath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return userErr(fmt.Errorf("no manifest at %s — run `csk init` first", s.ManifestPath))
				}
				return envErr(err)
			}

			g, err := procguard.Acquire(s.ProcLockPath)
			if err != nil {
				return classifyProcguard(err)
			}
			defer g.Unlock()

			mf, err := manifest.Load(s.ManifestPath)
			if err != nil {
				return userErr(err)
			}
			lf, err := loadLockfileOrEmpty(s.LockfilePath)
			if err != nil {
				return userErr(err)
			}

			_, inMf := mf.Skills[name]
			inLf := lf.Find(name) != nil
			if !inMf && !inLf {
				return userErr(fmt.Errorf("skill %q is not in the manifest or lockfile", name))
			}

			// Remove junction if it's one we manage.
			linkPath := cache.LinkPath(s, name)
			cdir := cache.CacheDir(s, name)
			if managed, _ := link.IsManagedLink(linkPath, cdir); managed {
				if err := link.Remove(linkPath); err != nil {
					return envErr(err)
				}
			} else if _, lerr := os.Lstat(linkPath); lerr == nil {
				// Path exists but isn't a managed link pointing at our cache.
				// Don't touch it — that's user data.
				fmt.Fprintf(cmd.OutOrStdout(), "csk: warning: %s exists but is not a csk-managed link; leaving it alone\n", linkPath)
			}

			// Optional cache prune.
			if pruneCache {
				if err := os.RemoveAll(cdir); err != nil {
					return envErr(err)
				}
			}

			delete(mf.Skills, name)
			lf.Remove(name)
			if err := mf.Save(s.ManifestPath); err != nil {
				return envErr(err)
			}
			if err := lf.Save(s.LockfilePath); err != nil {
				return envErr(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "csk: removed %s\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&pruneCache, "prune", false, "also delete the cache directory for this skill")
	return cmd
}

