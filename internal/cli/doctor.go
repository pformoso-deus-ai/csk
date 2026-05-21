package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/pformoso-deus-ai/csk/internal/cache"
	"github.com/pformoso-deus-ai/csk/internal/gitx"
	"github.com/pformoso-deus-ai/csk/internal/link"
	"github.com/pformoso-deus-ai/csk/internal/skill"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose cache/junction/lockfile drift (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveScope()
			if err != nil {
				return userErr(err)
			}

			mf, err := loadManifestOrEmpty(s.ManifestPath)
			if err != nil {
				return userErr(err)
			}
			lf, err := loadLockfileOrEmpty(s.LockfilePath)
			if err != nil {
				return userErr(err)
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			// Collect every name we know about, from either side.
			names := map[string]struct{}{}
			for k := range mf.Skills {
				names[k] = struct{}{}
			}
			for _, e := range lf.Skills {
				names[e.Name] = struct{}{}
			}
			sorted := make([]string, 0, len(names))
			for n := range names {
				sorted = append(sorted, n)
			}
			sort.Strings(sorted)

			out := cmd.OutOrStdout()
			problems := 0

			for _, name := range sorted {
				mfe, inMf := mf.Skills[name]
				lfe := lf.Find(name)

				switch {
				case !inMf:
					fmt.Fprintf(out, "  %s: in lockfile but not in manifest (run `csk add` to readopt or `csk remove`)\n", name)
					problems++
					continue
				case lfe == nil:
					fmt.Fprintf(out, "  %s: in manifest but not in lockfile (run `csk lock`)\n", name)
					problems++
					continue
				}

				cdir := cache.CacheDir(s, name)
				linkPath := cache.LinkPath(s, name)
				target := cdir
				if mfe.Subdir != "" {
					target = cache.LinkTarget(s, cache.Plan{Name: name, Subdir: mfe.Subdir})
				}

				// (a) cache exists
				if _, serr := os.Stat(cdir); errors.Is(serr, os.ErrNotExist) {
					fmt.Fprintf(out, "  %s: cache dir missing (run `csk install`)\n", name)
					problems++
					continue
				}

				// (b) cache HEAD matches lockfile commit
				head, herr := gitx.HeadCommit(ctx, cdir)
				switch {
				case herr != nil:
					fmt.Fprintf(out, "  %s: cache not a git repo (run `csk install --discard`)\n", name)
					problems++
					continue
				case head != lfe.Commit:
					fmt.Fprintf(out, "  %s: drifted — HEAD %s ≠ lockfile %s\n", name, shortSHA(head), shortSHA(lfe.Commit))
					problems++
				}

				// (c) subdir exists if specified
				if mfe.Subdir != "" {
					if _, serr := os.Stat(target); errors.Is(serr, os.ErrNotExist) {
						fmt.Fprintf(out, "  %s: subdir %q missing at commit %s\n", name, mfe.Subdir, shortSHA(lfe.Commit))
						problems++
						continue
					}
				}

				// (d) junction exists and points at our cache target
				if managed, _ := link.IsManagedLink(linkPath, target); !managed {
					if _, lerr := os.Lstat(linkPath); errors.Is(lerr, os.ErrNotExist) {
						fmt.Fprintf(out, "  %s: junction missing at %s (run `csk install`)\n", name, linkPath)
					} else {
						fmt.Fprintf(out, "  %s: junction %s does not point at cache target %s\n", name, linkPath, target)
					}
					problems++
					continue
				}

				// (e) SKILL.md present + name matches (warning only)
				if fm, ferr := skill.ReadFrontmatter(target); ferr != nil {
					fmt.Fprintf(out, "  %s: no readable SKILL.md at %s (%v)\n", name, target, ferr)
					problems++
				} else if fm.Name != "" && fm.Name != name {
					fmt.Fprintf(out, "  %s: SKILL.md frontmatter name is %q (mismatch is legal but confusing)\n", name, fm.Name)
				}
			}

			if problems == 0 {
				fmt.Fprintln(out, "csk: ok — all skills clean")
			} else {
				fmt.Fprintf(out, "csk: %d problem(s) found\n", problems)
			}
			return nil
		},
	}
}
