package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pformoso/csk/internal/cache"
	"github.com/pformoso/csk/internal/lockfile"
	"github.com/pformoso/csk/internal/manifest"
	"github.com/pformoso/csk/internal/procguard"
	"github.com/pformoso/csk/internal/skill"
)

func newAddCmd() *cobra.Command {
	var (
		nameFlag   string
		refFlag    string
		subdirFlag string
		force      bool
	)
	cmd := &cobra.Command{
		Use:   "add <source>",
		Short: "Clone a git source and register it as a skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := args[0]
			if source == "" {
				return userErr(errors.New("source must not be empty"))
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
			lf, err := lockfile.Load(s.LockfilePath)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return userErr(err)
				}
				lf = lockfile.New()
			}

			ref := refFlag
			if ref == "" {
				ref = manifest.DefaultRef
			}

			// If this source is already declared, lock onto that name.
			existingNameForSource := ""
			for k, v := range mf.Skills {
				if v.Source == source {
					existingNameForSource = k
					break
				}
			}

			// Tentative name: --name > existing-by-source > basename
			tentative := nameFlag
			if tentative == "" {
				if existingNameForSource != "" {
					tentative = existingNameForSource
				} else {
					tentative = skill.InferNameFromSource(source)
				}
			}
			if tentative == "" {
				return userErr(errors.New("could not infer skill name from source; pass --name"))
			}

			// Conflict at tentative name (different-source case).
			if existing, ok := mf.Skills[tentative]; ok {
				if existing.Source != source && !force {
					return userErr(fmt.Errorf("skill %q already declared with a different source (use --force to override)", tentative))
				}
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			// Clone or fetch, resolve ref → commit.
			plan := cache.Plan{Name: tentative, Source: source, Ref: ref, Subdir: subdirFlag}
			commit, err := cache.Resolve(ctx, s, plan)
			if err != nil {
				return envErr(err)
			}
			plan.Commit = commit

			// SKILL.md-based name upgrade, only on a fresh add (no --name, no
			// existing source match).
			finalName := tentative
			if nameFlag == "" && existingNameForSource == "" {
				if fm, ferr := skill.ReadFrontmatter(cache.LinkTarget(s, plan)); ferr == nil && fm.Name != "" && fm.Name != tentative {
					if _, ok := mf.Skills[fm.Name]; ok {
						return userErr(fmt.Errorf("SKILL.md declares name %q which is already in the manifest under a different source", fm.Name))
					}
					newDir := cache.CacheDir(s, fm.Name)
					if _, err := os.Stat(newDir); err == nil {
						return userErr(fmt.Errorf("SKILL.md declares name %q but cache dir %s already exists", fm.Name, newDir))
					}
					if err := os.Rename(cache.CacheDir(s, tentative), newDir); err != nil {
						return envErr(err)
					}
					finalName = fm.Name
					plan.Name = finalName
				}
			}

			// Require SKILL.md at the final target.
			if _, ferr := skill.ReadFrontmatter(cache.LinkTarget(s, plan)); ferr != nil {
				if errors.Is(ferr, os.ErrNotExist) {
					return userErr(fmt.Errorf("no SKILL.md at %s — is this a Claude Code skill?", cache.LinkTarget(s, plan)))
				}
				return userErr(ferr)
			}

			if err := cache.Reconcile(ctx, s, plan, false); err != nil {
				return envErr(err)
			}

			mf.Skills[finalName] = manifest.Entry{Source: source, Ref: refFlag, Subdir: subdirFlag}
			lf.Upsert(lockfile.Entry{Name: finalName, Source: source, Ref: ref, Commit: commit, Subdir: subdirFlag})
			if err := mf.Save(s.ManifestPath); err != nil {
				return envErr(err)
			}
			if err := lf.Save(s.LockfilePath); err != nil {
				return envErr(err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "csk: added %s @ %s (%s)\n", finalName, ref, shortSHA(commit))
			return nil
		},
	}
	cmd.Flags().StringVar(&nameFlag, "name", "", "override the install name")
	cmd.Flags().StringVar(&refFlag, "ref", "", "branch, tag, or commit to pin (default: main)")
	cmd.Flags().StringVar(&subdirFlag, "subdir", "", "subdirectory inside the repo to expose")
	cmd.Flags().BoolVar(&force, "force", false, "allow replacing an existing entry with a different source")
	return cmd
}

func shortSHA(sha string) string {
	if len(sha) <= 12 {
		return sha
	}
	return sha[:12]
}
